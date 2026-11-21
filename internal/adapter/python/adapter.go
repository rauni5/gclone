package python

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rauni5/gclone/internal/adapter"
	"github.com/rauni5/gclone/internal/envfile"
	"github.com/rauni5/gclone/internal/fsx"
	"github.com/rauni5/gclone/internal/proc"
	"github.com/rauni5/gclone/internal/project"
)

// Adapter is the Python ecosystem.
type Adapter struct{}

var _ adapter.Adapter = Adapter{}

func (Adapter) Name() string                 { return "Python" }
func (Adapter) Units(p *project.Project) int { return len(p.PyProjects) }
func (Adapter) Markers() []string            { return markers }

func (Adapter) Analyze(p *project.Project, root string) {
	dirs := FindProjects(root)
	if len(dirs) == 0 {
		return
	}
	_, version := Interpreter()
	for _, d := range dirs {
		p.PyProjects = append(p.PyProjects, Analyze(root, d, version))
	}
	for _, y := range p.PyProjects {
		var exclude []string
		for _, other := range p.PyProjects {
			if other != y && fsx.IsInside(y.Dir, other.Dir) {
				exclude = append(exclude, other.Dir)
			}
		}
		y.MissingVars = envfile.MissingVars(ScanEnv(y.Dir, exclude), y.EnvVars)
	}
}

func (Adapter) Warnings(p *project.Project) []string {
	var out []string
	for _, y := range p.PyProjects {
		for _, w := range y.Warnings {
			out = append(out, y.Rel+": "+w)
		}
	}
	return out
}

func (Adapter) Detected(p *project.Project) []string {
	var out []string
	for _, y := range p.PyProjects {
		line := fmt.Sprintf("%s: %s, Python", y.Rel, y.Name)
		if y.Framework != "" {
			line += " (" + y.Framework + ")"
		}
		line += ", " + y.Manager
		if y.PyWanted != "" {
			line += ", Python " + y.PyWanted
		}
		out = append(out, line)
	}
	return out
}

func (Adapter) Prepare(p *project.Project, o project.Options) []project.Planned {
	var steps []project.Planned
	for _, y := range p.PyProjects {
		y := y
		if y.EnvExample == "" || fsx.FileExists(filepath.Join(y.Dir, ".env")) {
			continue
		}
		src, dst := filepath.Join(y.Dir, y.EnvExample), filepath.Join(y.Dir, ".env")
		steps = append(steps, project.Planned{
			Where: y.Rel,
			Name:  "Create .env from " + y.EnvExample,
			Cmd:   "copy " + y.EnvExample + " .env",
			Run: func() error {
				if err := fsx.CopyFile(src, dst); err != nil {
					return err
				}
				y.EnvCreated = true
				return nil
			},
		})
	}
	return steps
}

var installHelp = map[string]string{
	"poetry": "install it from https://python-poetry.org/docs/#installation",
	"uv":     "install it from https://docs.astral.sh/uv/getting-started/installation/",
	"pipenv": "install it with `pip install --user pipenv`",
}

func (Adapter) Install(p *project.Project, o project.Options) []project.Planned {
	if o.NoInstall {
		return nil
	}
	var steps []project.Planned
	for _, y := range p.PyProjects {
		y := y
		if y.Manager == "pip" && !y.HasVenv {
			shown := "python3 -m venv .venv"
			if cmd, _ := Interpreter(); len(cmd) > 0 {
				shown = strings.Join(cmd, " ") + " -m venv .venv"
			}
			steps = append(steps, project.Planned{
				Where:     y.Rel,
				Name:      "Create virtual environment (.venv)",
				Cmd:       shown,
				Resumable: true,
				Run: func() error {
					cmd, _ := Interpreter()
					if cmd == nil {
						return fmt.Errorf("Python 3 was not found in PATH, install it from https://www.python.org/downloads/")
					}
					if err := proc.RunStream(y.Dir, false, cmd[0], append(append([]string{}, cmd[1:]...), "-m", "venv", ".venv")...); err != nil {
						return err
					}
					y.HasVenv = true
					fsx.ExcludeFromGit(p.Dir, filepath.ToSlash(filepath.Join(y.Rel, ".venv"))+"/")
					return nil
				},
			})
		}
		argv, shown := InstallArgs(y)
		steps = append(steps, project.Planned{
			Where:     y.Rel,
			Name:      "Install dependencies (" + y.Manager + ")",
			Cmd:       shown,
			Resumable: true,
			Run: func() error {
				if y.Manager == "pip" {
					if !fsx.FileExists(VenvPython(y.Dir)) {
						return fmt.Errorf("the virtual environment .venv was not created, so nothing was installed")
					}
				} else if _, err := exec.LookPath(argv[0]); err != nil {
					return fmt.Errorf("%s was not found in PATH, %s", argv[0], installHelp[y.Manager])
				}
				return proc.RunStream(y.Dir, false, argv[0], argv[1:]...)
			},
		})
	}
	return steps
}

func (Adapter) Runnables(p *project.Project, noBuild bool) []project.Runnable {
	var out []project.Runnable
	for _, y := range p.PyProjects {
		r := project.Runnable{Label: y.Rel + " (Python)", Dir: y.Dir, ExampleName: y.EnvExample, Missing: y.MissingVars}
		if y.Start != nil {
			runner, _ := Runner(y)
			r.Start = append(append([]string{}, runner...), y.Start...)
			if n, err := strconv.Atoi(y.Port); err == nil && n > 0 {
				r.Port, r.PortFrom = n, y.Framework+" default"
			}
		} else {
			r.NoStart = y.StartNote
		}
		envPath := filepath.Join(y.Dir, ".env")
		if fsx.FileExists(envPath) {
			r.HasEnv = true
			for _, v := range envfile.ParseEnvFile(envPath) {
				if v.Kind == "blocking" {
					r.Blocking = append(r.Blocking, v.Key)
				}
			}
		}
		out = append(out, r)
	}
	return out
}

func prefix(rel string) string {
	if rel == "." {
		return ""
	}
	return "cd " + rel + " && "
}

func (Adapter) Next(p *project.Project) []string {
	var out []string
	for _, y := range p.PyProjects {
		if y.Start == nil {
			continue
		}
		_, shown := Runner(y)
		out = append(out, prefix(y.Rel)+shown+" "+strings.Join(y.Start, " "))
	}
	return out
}

func (Adapter) Guide(p *project.Project) string {
	var b strings.Builder
	w := func(format string, a ...interface{}) { fmt.Fprintf(&b, format+"\n", a...) }
	for _, y := range p.PyProjects {
		title := y.Name
		if y.Rel != "." {
			title += " (`" + y.Rel + "`)"
		}
		w("### %s", title)
		w("")
		stack := "Python, " + y.Manager
		if y.Framework != "" {
			stack = y.Framework + ", " + stack
		}
		if y.PyWanted != "" {
			stack += fmt.Sprintf(" (Python %s or newer)", y.PyWanted)
		}
		w("Stack: %s", stack)
		if y.Port != "" && y.Start != nil {
			w("")
			w("Expected port: %s", y.Port)
		}
		w("")
		w("```bash")
		if y.Rel != "." {
			w("cd %s", y.Rel)
		}
		if y.Manager == "pip" && !y.HasVenv {
			w("python3 -m venv .venv          # once, a private environment for this project")
		}
		_, install := InstallArgs(y)
		w("%s   # dependencies", install)
		_, runner := Runner(y)
		if y.Framework == "Django" && fsx.FileExists(filepath.Join(y.Dir, "manage.py")) {
			w("%s manage.py migrate   # create the database tables", runner)
		}
		if y.Start != nil {
			w("%s %s", runner, strings.Join(y.Start, " "))
		} else {
			w("# %s", y.StartNote)
		}
		switch {
		case y.Pytest:
			w("%s -m pytest   # tests", runner)
		case y.Framework == "Django" && y.Start != nil:
			w("%s manage.py test   # tests", runner)
		}
		w("```")
		if len(y.ExtraRequirements) > 0 {
			w("")
			for _, r := range y.ExtraRequirements {
				w("- Also found `%s` (not installed). Run `pip install -r %s` if you need those tools.", r, r)
			}
		}
		w("")
	}
	return b.String()
}

func (Adapter) GuideConfig(p *project.Project) []string {
	var out []string
	for _, y := range p.PyProjects {
		var blocking, review []project.EnvVar
		for _, v := range y.EnvVars {
			switch v.Kind {
			case "blocking":
				blocking = append(blocking, v)
			case "review":
				review = append(review, v)
			}
		}
		if len(blocking) == 0 && len(review) == 0 && len(y.MissingVars) == 0 {
			continue
		}
		var b strings.Builder
		w := func(format string, a ...interface{}) { fmt.Fprintf(&b, format+"\n", a...) }
		w("### Environment: %s", project.UnitLabel(y.Rel))
		w("")
		if y.EnvCreated {
			w("`.env` was created from `%s`.", y.EnvExample)
			w("")
		}
		if len(blocking) > 0 {
			w("**Set these first:**")
			w("")
			for _, v := range blocking {
				w("- `%s`: %s", v.Key, v.Reason)
			}
			w("")
		}
		if len(review) > 0 {
			w("**Works on defaults, change for real use:**")
			w("")
			for _, v := range review {
				w("- `%s`: %s", v.Key, v.Reason)
			}
			w("")
		}
		if len(y.MissingVars) > 0 {
			w("**Read by the code but not in the example file:**")
			w("")
			for _, k := range y.MissingVars {
				w("- `%s`", k)
			}
			w("")
		}
		out = append(out, b.String())
	}
	return out
}
