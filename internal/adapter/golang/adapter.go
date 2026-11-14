package golang

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/rauni5/gclone/internal/adapter"
	"github.com/rauni5/gclone/internal/fsx"
	"github.com/rauni5/gclone/internal/proc"
	"github.com/rauni5/gclone/internal/project"
)

// Adapter is the Go ecosystem.
type Adapter struct{}

var _ adapter.Adapter = Adapter{}

func (Adapter) Name() string                                                { return "Go" }
func (Adapter) Units(p *project.Project) int                                { return len(p.GoModules) }
func (Adapter) Markers() []string                                           { return []string{"go.mod"} }
func (Adapter) Prepare(*project.Project, project.Options) []project.Planned { return nil }
func (Adapter) GuideConfig(*project.Project) []string                       { return nil }

func (Adapter) Analyze(p *project.Project, root string) {
	dirs := FindModules(root)
	if len(dirs) == 0 {
		return
	}
	installed := Installed()
	for _, d := range dirs {
		m, err := Analyze(root, d, installed)
		if err != nil {
			p.Warnings = append(p.Warnings, fmt.Sprintf("%s: %v", fsx.Rel(root, d), err))
			continue
		}
		p.GoModules = append(p.GoModules, m)
	}
}

func (Adapter) Warnings(p *project.Project) []string {
	var out []string
	for _, m := range p.GoModules {
		for _, w := range m.Warnings {
			out = append(out, m.Rel+": "+w)
		}
	}
	return out
}

func (Adapter) Detected(p *project.Project) []string {
	var out []string
	for _, m := range p.GoModules {
		line := fmt.Sprintf("%s: %s, Go", m.Rel, m.Module)
		if m.GoWanted != "" {
			line += " " + m.GoWanted
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
	for _, m := range p.GoModules {
		m := m
		steps = append(steps, project.Planned{
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
	return steps
}

func (Adapter) Runnables(p *project.Project, noBuild bool) []project.Runnable {
	return Runnables(p, noBuild)
}

func (Adapter) Next(p *project.Project) []string {
	var out []string
	for _, m := range p.GoModules {
		prefix := ""
		if m.Rel != "." {
			prefix = "cd " + m.Rel + " && "
		}
		out = append(out, prefix+"go test ./...")
	}
	return out
}

func (Adapter) Guide(p *project.Project) string {
	var b strings.Builder
	w := func(format string, a ...interface{}) { fmt.Fprintf(&b, format+"\n", a...) }
	for _, m := range p.GoModules {
		title := m.Module
		if m.Rel != "." {
			title += " (`" + m.Rel + "`)"
		}
		w("### %s", title)
		w("")
		stack := "Go"
		if m.GoWanted != "" {
			stack += " " + m.GoWanted
		}
		w("Stack: %s", stack)
		w("")
		w("```bash")
		if m.Rel != "." {
			w("cd %s", m.Rel)
		}
		w("go mod download                 # dependencies")
		w("%s", strings.Join(BuildArgs(m), " "))
		w("go test ./...                   # tests")
		w("```")
		w("")
	}
	return b.String()
}
