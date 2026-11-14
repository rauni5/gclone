package plan

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/rauni5/gclone/internal/adapters"
	"github.com/rauni5/gclone/internal/buildinfo"
	"github.com/rauni5/gclone/internal/docker"
	"github.com/rauni5/gclone/internal/explain"
	"github.com/rauni5/gclone/internal/proc"
	"github.com/rauni5/gclone/internal/project"
)

func BuildPlan(p *project.Project, o project.Options) []project.Planned {
	var plan []project.Planned

	for _, a := range adapters.All() {
		plan = append(plan, a.Prepare(p, o)...)
	}

	if !o.NoServices {
		for _, c := range p.Composes {
			c := c
			if len(c.DBServices) == 0 {
				continue
			}
			rest := append([]string{"-f", c.Path, "up", "-d"}, c.DBServices...)
			shown := append([]string{"docker", "compose", "-f", filepath.Base(c.Path), "up", "-d"}, c.DBServices...)
			plan = append(plan, project.Planned{
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

	for _, a := range adapters.All() {
		plan = append(plan, a.Install(p, o)...)
	}
	return plan
}

func PrintPlan(p *project.Project, plan []project.Planned, o project.Options) {
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
	for _, a := range adapters.All() {
		for _, line := range a.Detected(p) {
			fmt.Println("  - " + line)
		}
	}
	for _, s := range p.OtherStacks {
		fmt.Println("  - also found (not handled yet): " + s)
	}

	var warns []string
	warns = append(warns, p.Warnings...)
	for _, a := range adapters.All() {
		warns = append(warns, a.Warnings(p)...)
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

func loc(where string) string {
	if where == "" || where == "." {
		return "repo root"
	}
	return where
}

func ExecutePlan(p *project.Project, plan []project.Planned) {
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
	for _, a := range adapters.All() {
		for _, cmd := range a.Next(p) {
			fmt.Println("  " + cmd)
		}
	}
	fmt.Println("  Open SETUP_GUIDE.md for the values you still need to set.")
}
