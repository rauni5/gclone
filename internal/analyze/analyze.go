package analyze

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rauni5/gclone/internal/adapters"
	"github.com/rauni5/gclone/internal/compose"
	"github.com/rauni5/gclone/internal/fsx"
	"github.com/rauni5/gclone/internal/project"
	"github.com/rauni5/gclone/internal/services"
)

func AnalyzeProject(root string) *project.Project {
	p := &project.Project{Dir: root}
	for _, a := range adapters.All() {
		a.Analyze(p, root)
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
	for _, a := range adapters.All() {
		if a.Units(p) > 0 {
			for _, m := range a.Markers() {
				h[m] = true
			}
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
