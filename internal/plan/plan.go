package plan

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rauni5/gclone/internal/adapter/java"
	"github.com/rauni5/gclone/internal/adapter/node"
	"github.com/rauni5/gclone/internal/buildinfo"
	"github.com/rauni5/gclone/internal/docker"
	"github.com/rauni5/gclone/internal/explain"
	"github.com/rauni5/gclone/internal/fsx"
	"github.com/rauni5/gclone/internal/proc"
	"github.com/rauni5/gclone/internal/project"
)

type Planned struct {
	Where        string
	Name         string
	Cmd          string
	Run          func() error
	Fallback     func() error
	FallbackNote string
}

func BuildPlan(p *project.Project, o project.Options) []Planned {
	var plan []Planned

	for _, pkg := range p.Pkgs {
		pkg := pkg
		if pkg.EnvExample == "" || fsx.FileExists(filepath.Join(pkg.Dir, ".env")) {
			continue
		}
		src := filepath.Join(pkg.Dir, pkg.EnvExample)
		dst := filepath.Join(pkg.Dir, ".env")
		plan = append(plan, Planned{
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

	if !o.NoServices {
		for _, c := range p.Composes {
			c := c
			if len(c.DBServices) == 0 {
				continue
			}
			rest := append([]string{"-f", c.Path, "up", "-d"}, c.DBServices...)
			shown := append([]string{"docker", "compose", "-f", filepath.Base(c.Path), "up", "-d"}, c.DBServices...)
			plan = append(plan, Planned{
				Where: filepath.ToSlash(filepath.Dir(c.Rel)),
				Name:  "Start local services: " + strings.Join(c.DBServices, ", ") + docker.PlanTag(p.DockerState),
				Cmd:   strings.Join(shown, " "),
				Run: func() error {
					if !docker.Ensure(o) {
						return fmt.Errorf("docker is not available, so these services were not started (see the guide)")
					}
					p.DockerState = "ready"
					base, err := docker.ComposeBase()
					if err != nil {
						return err
					}
					args := append(append([]string{}, base[1:]...), rest...)
					if err := proc.RunStream(filepath.Dir(c.Path), false, base[0], args...); err != nil {
						return fmt.Errorf("%w (if an image could not be downloaded, check your internet connection or proxy, or run `docker login`)", err)
					}
					c.Started = true
					return nil
				},
			})
		}
	}

	if !o.NoInstall {
		for _, pkg := range p.Pkgs {
			pkg := pkg
			name, args := node.InstallCmd(pkg, o.IgnoreScripts)
			step := Planned{
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
				step.FallbackNote = npmCIFallbackNote()
			}
			plan = append(plan, step)
		}
	}
	if !o.NoInstall {
		for _, m := range p.GoModules {
			m := m
			plan = append(plan, Planned{
				Where: m.Rel,
				Name:  "Download Go modules",
				Cmd:   "go mod download",
				Run: func() error {
					if _, err := exec.LookPath("go"); err != nil {
						return fmt.Errorf("go was not found in PATH, install it from https://go.dev/dl/")
					}
					return proc.RunStream(m.Dir, false, "go", "mod", "download")
				},
			})
		}
	}
	if !o.NoInstall {
		for _, j := range p.JavaProjects {
			j := j
			argv := java.Argv(j, java.DependencyGoals(j)...)
			plan = append(plan, Planned{
				Where: j.Rel,
				Name:  "Download Java dependencies (" + j.Build + ")",
				Cmd:   java.Display(j, java.DependencyGoals(j)...),
				Run: func() error {
					if _, err := exec.LookPath("java"); err != nil {
						return fmt.Errorf("java was not found in PATH, install a JDK from https://adoptium.net/")
					}
					if _, err := exec.LookPath(argv[0]); err != nil {
						return fmt.Errorf("%s was not found in PATH", argv[0])
					}
					return proc.RunStream(j.Dir, false, argv[0], argv[1:]...)
				},
			})
		}
	}
	return plan
}

func PrintPlan(p *project.Project, plan []Planned, o project.Options) {
	fmt.Println()
	if !p.HasUnits() {
		fmt.Println("No Node.js, Go or Java project found (no package.json, go.mod, pom.xml or build.gradle at the root or one level down).")
		if len(p.OtherStacks) > 0 {
			fmt.Println("Detected other stacks, which gclone v" + buildinfo.Version + " does not set up yet:")
			for _, s := range p.OtherStacks {
				fmt.Println("  - " + s)
			}
		}
		fmt.Println("The repository was cloned, nothing else was done.")
		return
	}

	fmt.Println("Detected:")
	for _, j := range p.JavaProjects {
		line := fmt.Sprintf("  - %s: Java (%s)", j.Rel, j.Build)
		if j.SpringBoot {
			line += ", Spring Boot"
		}
		if j.JavaWanted > 0 {
			line += fmt.Sprintf(", Java %d", j.JavaWanted)
		}
		fmt.Println(line)
	}
	for _, m := range p.GoModules {
		line := fmt.Sprintf("  - %s: %s, Go", m.Rel, m.Module)
		if m.GoWanted != "" {
			line += " " + m.GoWanted
		}
		fmt.Println(line)
	}
	for _, pkg := range p.Pkgs {
		line := fmt.Sprintf("  - %s: %s, %s", pkg.Rel, pkg.Name, pkg.PM)
		if pkg.Framework != "" {
			line += ", " + pkg.Framework
		}
		if pkg.NodeWanted != "" {
			line += ", node " + pkg.NodeWanted
		}
		fmt.Println(line)
	}
	for _, s := range p.OtherStacks {
		fmt.Println("  - also found (not handled yet): " + s)
	}

	var warns []string
	warns = append(warns, p.Warnings...)
	for _, pkg := range p.Pkgs {
		for _, w := range pkg.Warnings {
			warns = append(warns, pkg.Rel+": "+w)
		}
	}
	if len(warns) > 0 {
		fmt.Println("\nWarnings:")
		for _, w := range warns {
			fmt.Println("  [!!] " + w)
		}
	}

	var scripts []string
	for _, pkg := range p.Pkgs {
		for _, s := range pkg.LifecycleScripts {
			scripts = append(scripts, pkg.Rel+" -> "+s)
		}
	}
	if len(scripts) > 0 && !o.NoInstall {
		fmt.Println("\nThis repo runs its own scripts during install (review them if you do not trust it):")
		for _, s := range scripts {
			fmt.Println("  - " + s)
		}
		fmt.Println("  Use --ignore-scripts to skip these and your dependencies' install scripts.")
	}

	if len(plan) == 0 {
		fmt.Println("\nNothing to run.")
		return
	}
	fmt.Println("\nPlan:")
	for i, s := range plan {
		fmt.Printf("  %d. %s   (in %s)\n       $ %s\n", i+1, s.Name, loc(s.Where), s.Cmd)
	}
}

func npmCIFallbackNote() string {
	return "npm ci failed because the lockfile may be out of sync, so gclone used npm install instead. Commit the updated lockfile, or report the issue upstream."
}

func loc(where string) string {
	if where == "" || where == "." {
		return "repo root"
	}
	return where
}

func ExecutePlan(p *project.Project, plan []Planned) {
	for _, s := range plan {
		fmt.Printf("\n[..] %s (in %s)\n", s.Name, loc(s.Where))
		err := s.Run()
		note := ""
		if err != nil && s.Fallback != nil {
			fmt.Printf("[!!] %v\n[..] %s\n", err, s.FallbackNote)
			err = s.Fallback()
			note = s.FallbackNote
		}
		st := project.Step{Where: loc(s.Where), Name: s.Name, Cmd: s.Cmd, Status: "ok", Note: note}
		if err != nil {
			st.Status, st.Note = "failed", err.Error()
			fmt.Printf("[!!] failed: %v\n", err)
			if h, ok := explain.FromError(err); ok {
				explain.Print(h)
				st.Hint = h.Line()
			}
		} else {
			fmt.Println("[ok] done")
		}
		p.Steps = append(p.Steps, st)
	}
}

func PrintNext(p *project.Project) {
	fmt.Println("\nNext:")
	for _, pkg := range p.Pkgs {
		if s := project.DevScript(pkg); s != "" {
			prefix := ""
			if pkg.Rel != "." {
				prefix = "cd " + pkg.Rel + " && "
			}
			fmt.Printf("  %s%s\n", prefix, pkg.PM+" run "+s)
		}
	}
	for _, m := range p.GoModules {
		prefix := ""
		if m.Rel != "." {
			prefix = "cd " + m.Rel + " && "
		}
		fmt.Printf("  %sgo test ./...\n", prefix)
	}
	for _, j := range p.JavaProjects {
		prefix := ""
		if j.Rel != "." {
			prefix = "cd " + j.Rel + " && "
		}
		if j.SpringBoot {
			fmt.Printf("  %s%s\n", prefix, java.Display(j, java.RunGoals(j)...))
		} else {
			fmt.Printf("  %s%s\n", prefix, java.Display(j, java.TestGoals()...))
		}
	}
	fmt.Println("  Open SETUP_GUIDE.md for the values you still need to set.")
}
