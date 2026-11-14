package analyze

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rauni5/gclone/internal/adapter/golang"
	"github.com/rauni5/gclone/internal/adapter/java"
	"github.com/rauni5/gclone/internal/adapter/node"
	"github.com/rauni5/gclone/internal/compose"
	"github.com/rauni5/gclone/internal/envfile"
	"github.com/rauni5/gclone/internal/fsx"
	"github.com/rauni5/gclone/internal/project"
	"github.com/rauni5/gclone/internal/services"
)

func AnalyzeProject(root string) *project.Project {
	p := &project.Project{Dir: root}
	inst := node.InstalledNode()
	for _, d := range node.FindPackageDirs(root) {
		pkg, err := node.AnalyzePkg(root, d, inst)
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
	if dirs := golang.FindModules(root); len(dirs) > 0 {
		installed := golang.Installed()
		for _, d := range dirs {
			m, err := golang.Analyze(root, d, installed)
			if err != nil {
				p.Warnings = append(p.Warnings, fmt.Sprintf("%s: %v", fsx.Rel(root, d), err))
				continue
			}
			p.GoModules = append(p.GoModules, m)
			for _, w := range m.Warnings {
				p.Warnings = append(p.Warnings, m.Rel+": "+w)
			}
		}
	}
	if dirs := java.FindProjects(root); len(dirs) > 0 {
		installed := java.InstalledJava()
		for _, d := range dirs {
			j, err := java.Analyze(root, d, installed)
			if err != nil {
				p.Warnings = append(p.Warnings, fmt.Sprintf("%s: %v", fsx.Rel(root, d), err))
				continue
			}
			p.JavaProjects = append(p.JavaProjects, j)
			for _, w := range j.Warnings {
				p.Warnings = append(p.Warnings, j.Rel+": "+w)
			}
		}
	}
	p.Composes = compose.FindComposes(root, p.Pkgs)
	p.Needs = services.DetectNeeds(p)
	p.OtherStacks = findOtherStacks(root, handledMarkers(p))
	return p
}

var otherMarkers = map[string]string{
	"pom.xml":          "Java (Maven)",
	"build.gradle":     "Java/Kotlin (Gradle)",
	"build.gradle.kts": "Java/Kotlin (Gradle)",
	"pubspec.yaml":     "Flutter/Dart",
	"requirements.txt": "Python",
	"pyproject.toml":   "Python",
	"go.mod":           "Go",
	"Cargo.toml":       "Rust",
	"composer.json":    "PHP",
	"Gemfile":          "Ruby",
}

// handledMarkers lists build files that an adapter already covers, so they are not
// reported as unsupported stacks.
func handledMarkers(p *project.Project) map[string]bool {
	h := map[string]bool{}
	if len(p.GoModules) > 0 {
		h["go.mod"] = true
	}
	if len(p.JavaProjects) > 0 {
		for _, f := range []string{"pom.xml", "build.gradle", "build.gradle.kts"} {
			h[f] = true
		}
	}
	return h
}

func findOtherStacks(root string, handled map[string]bool) []string {
	markers := make([]string, 0, len(otherMarkers))
	for m := range otherMarkers {
		markers = append(markers, m)
	}
	sort.Strings(markers)

	dirs := []string{root}
	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") && !fsx.SkipDirs[e.Name()] {
			dirs = append(dirs, filepath.Join(root, e.Name()))
		}
	}
	seen := map[string]bool{}
	var out []string
	for _, d := range dirs {
		for _, m := range markers {
			if handled[m] {
				continue
			}
			if fsx.FileExists(filepath.Join(d, m)) {
				s := fmt.Sprintf("%s (%s in %s)", otherMarkers[m], m, fsx.Rel(root, d))
				if !seen[s] {
					seen[s] = true
					out = append(out, s)
				}
			}
		}
	}
	return out
}
