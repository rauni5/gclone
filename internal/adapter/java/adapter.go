package java

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rauni5/gclone/internal/adapter"
	"github.com/rauni5/gclone/internal/fsx"
	"github.com/rauni5/gclone/internal/proc"
	"github.com/rauni5/gclone/internal/project"
)

// Adapter is the Java ecosystem (Maven, Gradle, Spring Boot).
type Adapter struct{}

var _ adapter.Adapter = Adapter{}

func (Adapter) Name() string                                                { return "Java" }
func (Adapter) Units(p *project.Project) int                                { return len(p.JavaProjects) }
func (Adapter) Markers() []string                                           { return buildFiles }
func (Adapter) Prepare(*project.Project, project.Options) []project.Planned { return nil }

func (Adapter) Analyze(p *project.Project, root string) {
	dirs := FindProjects(root)
	if len(dirs) == 0 {
		return
	}
	installed := InstalledJava()
	for _, d := range dirs {
		j, err := Analyze(root, d, installed)
		if err != nil {
			p.Warnings = append(p.Warnings, fmt.Sprintf("%s: %v", fsx.Rel(root, d), err))
			continue
		}
		p.JavaProjects = append(p.JavaProjects, j)
	}
}

func (Adapter) Warnings(p *project.Project) []string {
	var out []string
	for _, j := range p.JavaProjects {
		for _, w := range j.Warnings {
			out = append(out, j.Rel+": "+w)
		}
	}
	return out
}

func (Adapter) Detected(p *project.Project) []string {
	var out []string
	for _, j := range p.JavaProjects {
		line := fmt.Sprintf("%s: Java (%s)", j.Rel, j.Build)
		if j.SpringBoot {
			line += ", Spring Boot"
		}
		if j.JavaWanted > 0 {
			line += fmt.Sprintf(", Java %d", j.JavaWanted)
		}
		out = append(out, line)
	}
	return out
}

func (Adapter) Install(p *project.Project, o project.Options) []project.Planned {
	if o.NoInstall {
		return nil
	}
	var steps []project.Planned
	for _, j := range p.JavaProjects {
		j := j
		argv := Argv(j, DependencyGoals(j)...)
		steps = append(steps, project.Planned{
			Where: j.Rel,
			Name:  "Download Java dependencies (" + j.Build + ")",
			Cmd:   Display(j, DependencyGoals(j)...),
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
	return steps
}

func (Adapter) Runnables(p *project.Project, noBuild bool) []project.Runnable {
	return Runnables(p, noBuild)
}

func (Adapter) Next(p *project.Project) []string {
	var out []string
	for _, j := range p.JavaProjects {
		prefix := ""
		if j.Rel != "." {
			prefix = "cd " + j.Rel + " && "
		}
		if j.SpringBoot {
			out = append(out, prefix+Display(j, RunGoals(j)...))
		} else {
			out = append(out, prefix+Display(j, TestGoals()...))
		}
	}
	return out
}

func (Adapter) Guide(p *project.Project) string {
	var b strings.Builder
	w := func(format string, a ...interface{}) { fmt.Fprintf(&b, format+"\n", a...) }
	for _, j := range p.JavaProjects {
		title := j.Rel
		if j.Rel == "." {
			title = filepath.Base(p.Dir)
		}
		w("### %s (Java, %s)", title, j.Build)
		w("")
		stack := "Java"
		if j.JavaWanted > 0 {
			stack += fmt.Sprintf(" %d", j.JavaWanted)
		}
		if j.SpringBoot {
			stack = "Spring Boot, " + stack
		}
		w("Stack: %s", stack)
		if j.SpringBoot && j.Port != "" {
			w("")
			w("Expected port: %s", j.Port)
		}
		w("")
		w("```bash")
		if j.Rel != "." {
			w("cd %s", j.Rel)
		}
		if j.SpringBoot {
			w("%s", Display(j, RunGoals(j)...))
		}
		w("%s   # build", Display(j, BuildGoals(j)...))
		w("%s   # tests", Display(j, TestGoals()...))
		w("```")
		w("")
	}
	return b.String()
}

// GuideConfig lists the settings in application.properties or application.yml that need
// attention. Values are not printed, only names and reasons.
func (Adapter) GuideConfig(p *project.Project) []string {
	var out []string
	for _, j := range p.JavaProjects {
		var blocking, review []project.EnvVar
		for _, v := range j.Config {
			switch v.Kind {
			case "blocking":
				blocking = append(blocking, v)
			case "review":
				review = append(review, v)
			}
		}
		if len(blocking) == 0 && len(review) == 0 {
			continue
		}
		var b strings.Builder
		w := func(format string, a ...interface{}) { fmt.Fprintf(&b, format+"\n", a...) }
		w("### Configuration: %s (application config)", project.UnitLabel(j.Rel))
		w("")
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
		out = append(out, b.String())
	}
	return out
}
