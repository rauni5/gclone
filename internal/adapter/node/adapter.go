package node

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rauni5/gclone/internal/adapter"
	"github.com/rauni5/gclone/internal/envfile"
	"github.com/rauni5/gclone/internal/fsx"
	"github.com/rauni5/gclone/internal/proc"
	"github.com/rauni5/gclone/internal/project"
)

// Adapter is the Node.js ecosystem (npm, pnpm, yarn, bun).
type Adapter struct{}

var _ adapter.Adapter = Adapter{}

func (Adapter) Name() string                 { return "Node.js" }
func (Adapter) Units(p *project.Project) int { return len(p.Pkgs) }
func (Adapter) Markers() []string            { return nil }

func (Adapter) Analyze(p *project.Project, root string) {
	inst := InstalledNode()
	for _, d := range FindPackageDirs(root) {
		pkg, err := AnalyzePkg(root, d, inst)
		if err != nil {
			p.Warnings = append(p.Warnings, fmt.Sprintf("%s: %v", fsx.Rel(root, d), err))
			continue
		}
		p.Pkgs = append(p.Pkgs, pkg)
	}
	for _, pkg := range p.Pkgs {
		var exclude []string
		for _, other := range p.Pkgs {
			if other != pkg && fsx.IsInside(pkg.Dir, other.Dir) {
				exclude = append(exclude, other.Dir)
			}
		}
		pkg.MissingVars = envfile.MissingVars(envfile.ScanEnvUsage(pkg.Dir, exclude), pkg.EnvVars)
	}
}

func (Adapter) Warnings(p *project.Project) []string {
	var out []string
	for _, pkg := range p.Pkgs {
		for _, w := range pkg.Warnings {
			out = append(out, pkg.Rel+": "+w)
		}
	}
	return out
}

func (Adapter) Detected(p *project.Project) []string {
	var out []string
	for _, pkg := range p.Pkgs {
		line := fmt.Sprintf("%s: %s, %s", pkg.Rel, pkg.Name, pkg.PM)
		if pkg.Framework != "" {
			line += ", " + pkg.Framework
		}
		if pkg.NodeWanted != "" {
			line += ", node " + pkg.NodeWanted
		}
		out = append(out, line)
	}
	return out
}

func (Adapter) Prepare(p *project.Project, o project.Options) []project.Planned {
	var steps []project.Planned
	for _, pkg := range p.Pkgs {
		pkg := pkg
		if pkg.EnvExample == "" || fsx.FileExists(filepath.Join(pkg.Dir, ".env")) {
			continue
		}
		src := filepath.Join(pkg.Dir, pkg.EnvExample)
		dst := filepath.Join(pkg.Dir, ".env")
		steps = append(steps, project.Planned{
			Where: pkg.Rel,
			Name:  "Create .env from " + pkg.EnvExample,
			Cmd:   "copy " + pkg.EnvExample + " .env",
			Run: func() error {
				if err := fsx.CopyFile(src, dst); err != nil {
					return err
				}
				pkg.EnvCreated = true
				return nil
			},
		})
	}
	return steps
}

func (Adapter) Install(p *project.Project, o project.Options) []project.Planned {
	if o.NoInstall {
		return nil
	}
	var steps []project.Planned
	for _, pkg := range p.Pkgs {
		pkg := pkg
		name, args := InstallCmd(pkg, o.IgnoreScripts)
		step := project.Planned{
			Where: pkg.Rel,
			Name:  "Install dependencies (" + pkg.PM + ")",
			Cmd:   name + " " + strings.Join(args, " "),
			Run: func() error {
				if _, err := exec.LookPath(name); err != nil {
					return fmt.Errorf("%s was not found in PATH (try: corepack enable)", name)
				}
				return proc.RunStream(pkg.Dir, false, name, args...)
			},
		}
		if pkg.PM == "npm" && args[0] == "ci" {
			fb := []string{"install"}
			if o.IgnoreScripts {
				fb = append(fb, "--ignore-scripts")
			}
			step.Fallback = func() error { return proc.RunStream(pkg.Dir, false, "npm", fb...) }
			step.FallbackNote = "npm ci failed (lockfile out of sync?), so npm install was used instead"
		}
		steps = append(steps, step)
	}
	return steps
}

func (Adapter) Runnables(p *project.Project, noBuild bool) []project.Runnable {
	return Runnables(p, noBuild)
}

func (Adapter) Next(p *project.Project) []string {
	var out []string
	for _, pkg := range p.Pkgs {
		if s := project.DevScript(pkg); s != "" {
			prefix := ""
			if pkg.Rel != "." {
				prefix = "cd " + pkg.Rel + " && "
			}
			out = append(out, prefix+pkg.PM+" run "+s)
		}
	}
	return out
}

func (Adapter) Guide(p *project.Project) string {
	var b strings.Builder
	w := func(format string, a ...interface{}) { fmt.Fprintf(&b, format+"\n", a...) }
	for _, pkg := range p.Pkgs {
		title := pkg.Name
		if pkg.Rel != "." {
			title += " (`" + pkg.Rel + "`)"
		}
		w("### %s", title)
		w("")
		stack := pkg.PM
		if pkg.Framework != "" {
			stack = pkg.Framework + ", " + pkg.PM
		}
		w("Stack: %s", stack)
		if port := project.PortFor(pkg); port != "" {
			w("")
			w("Expected port: %s", port)
		}
		w("")
		w("```bash")
		if pkg.Rel != "." {
			w("cd %s", pkg.Rel)
		}
		if s := project.DevScript(pkg); s != "" {
			w("%s run %s", pkg.PM, s)
		} else {
			w("# no dev/start script found in package.json, check the README")
		}
		w("```")
		w("")
		var others []string
		for _, s := range []string{"build", "test", "lint", "migrate", "seed"} {
			if c, ok := pkg.Scripts[s]; ok {
				others = append(others, fmt.Sprintf("- `%s run %s`: `%s`", pkg.PM, s, c))
			}
		}
		if len(others) > 0 {
			w("Other scripts:")
			w("")
			for _, o := range others {
				w("%s", o)
			}
			w("")
		}
	}
	return b.String()
}

func (Adapter) GuideConfig(p *project.Project) []string {
	var out []string
	for _, pkg := range p.Pkgs {
		var b strings.Builder
		if guideEnv(&b, pkg) {
			out = append(out, b.String())
		}
	}
	return out
}

// guideEnv writes the env section for one package and reports whether it wrote anything.
func guideEnv(b *strings.Builder, pkg *project.Pkg) bool {
	if len(pkg.EnvVars) == 0 && len(pkg.MissingVars) == 0 {
		return false
	}
	w := func(format string, a ...interface{}) { fmt.Fprintf(b, format+"\n", a...) }

	w("### Environment: %s", project.UnitLabel(pkg.Rel))
	w("")
	switch {
	case pkg.EnvCreated:
		w("`.env` was created from `%s` with its default values.", pkg.EnvExample)
	case pkg.EnvExample != "":
		w("`.env` already existed, so it was left untouched. Compare it with `%s`.", pkg.EnvExample)
	default:
		w("No example env file was found. These variables come from the source code.")
	}
	w("")

	var blocking, review, ok []project.EnvVar
	for _, v := range pkg.EnvVars {
		switch v.Kind {
		case "blocking":
			blocking = append(blocking, v)
		case "review":
			review = append(review, v)
		default:
			ok = append(ok, v)
		}
	}
	if len(blocking) > 0 {
		w("**Set these first** (empty or placeholder, the app will likely fail without them):")
		w("")
		for _, v := range blocking {
			w("- `%s`: %s", v.Key, describe(v))
		}
		w("")
	}
	if len(review) > 0 {
		w("**Works on defaults, change for real use:**")
		w("")
		for _, v := range review {
			// Values are deliberately not printed: example files sometimes hold real secrets.
			w("- `%s`: %s", v.Key, describe(v))
		}
		w("")
	}
	if len(ok) > 0 {
		keys := make([]string, 0, len(ok))
		for _, v := range ok {
			keys = append(keys, "`"+v.Key+"`")
		}
		sort.Strings(keys)
		w("**Defaults that should be fine:** %s", strings.Join(keys, ", "))
		w("")
	}
	if len(pkg.MissingVars) > 0 {
		keys := make([]string, 0, len(pkg.MissingVars))
		for _, k := range pkg.MissingVars {
			keys = append(keys, "`"+k+"`")
		}
		w("**Read by the code but not in the example file** (check the README for what they should be): %s", strings.Join(keys, ", "))
		w("")
	}
	return true
}

func describe(v project.EnvVar) string {
	if v.Comment != "" {
		return v.Reason + ". Note from the example file: " + v.Comment
	}
	return v.Reason
}
