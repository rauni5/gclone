// Package golang is the Go adapter: it finds Go modules, reads go.mod, compares the Go
// version the module asks for with the one installed, and says how to build and test it.
package golang

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/rauni5/gclone/internal/fsx"
	"github.com/rauni5/gclone/internal/project"
)

var (
	moduleRe    = regexp.MustCompile(`(?m)^\s*module\s+(\S+)`)
	goRe        = regexp.MustCompile(`(?m)^\s*go\s+(\d+(?:\.\d+){0,2})\b`)
	toolchainRe = regexp.MustCompile(`(?m)^\s*toolchain\s+go(\d+(?:\.\d+){0,2})\b`)
	versionRe   = regexp.MustCompile(`(\d+)\.(\d+)(?:\.(\d+))?`)
)

// FindModules returns the directories that hold a go.mod: just the root when it has one,
// otherwise folders one level down.
func FindModules(root string) []string {
	if fsx.FileExists(filepath.Join(root, "go.mod")) {
		return []string{root}
	}
	var dirs []string
	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") || fsx.SkipDirs[e.Name()] {
			continue
		}
		d := filepath.Join(root, e.Name())
		if fsx.FileExists(filepath.Join(d, "go.mod")) {
			dirs = append(dirs, d)
		}
	}
	return dirs
}

// ParseGoMod reads the module path, the go directive and the toolchain directive.
func ParseGoMod(text string) (module, goVersion, toolchain string) {
	if m := moduleRe.FindStringSubmatch(text); m != nil {
		module = strings.Trim(m[1], `"`)
	}
	if m := goRe.FindStringSubmatch(text); m != nil {
		goVersion = m[1]
	}
	if m := toolchainRe.FindStringSubmatch(text); m != nil {
		toolchain = m[1]
	}
	return
}

// Installed returns the installed Go version such as "1.24.3", or "" when go is missing.
func Installed() string {
	out, err := exec.Command("go", "env", "GOVERSION").Output()
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(strings.TrimSpace(string(out)), "go")
}

func parse(v string) [3]int {
	var r [3]int
	if m := versionRe.FindStringSubmatch(v); m != nil {
		r[0], _ = strconv.Atoi(m[1])
		r[1], _ = strconv.Atoi(m[2])
		r[2], _ = strconv.Atoi(m[3])
	}
	return r
}

// Older reports whether version a is older than b ("1.21" is older than "1.24.1").
func Older(a, b string) bool {
	x, y := parse(a), parse(b)
	for i := range x {
		if x[i] != y[i] {
			return x[i] < y[i]
		}
	}
	return false
}

// Analyze reads one module. installed is the result of Installed, passed in so a repository
// with several modules runs `go env` once.
func Analyze(root, dir, installed string) (*project.GoModule, error) {
	b, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return nil, err
	}
	module, goVer, tool := ParseGoMod(string(b))
	if module == "" {
		return nil, fmt.Errorf("go.mod has no module line")
	}
	m := &project.GoModule{
		Dir: dir, Rel: fsx.Rel(root, dir), Module: module,
		GoWanted: goVer, Toolchain: tool, Installed: installed,
		HasMain: fsx.FileExists(filepath.Join(dir, "main.go")),
		BinName: filepath.Base(module),
	}
	if strings.HasSuffix(m.BinName, "/v2") || regexp.MustCompile(`^v\d+$`).MatchString(m.BinName) {
		m.BinName = filepath.Base(filepath.Dir(module))
	}
	want := goVer
	if tool != "" {
		want = tool
	}
	switch {
	case installed == "":
		m.Warnings = append(m.Warnings, "Go was not found in PATH. Install Go "+want+" or newer from https://go.dev/dl/")
	case want != "" && Older(installed, want):
		m.Warnings = append(m.Warnings, fmt.Sprintf(
			"go.mod asks for Go %s but Go %s is installed. Go 1.21 and newer can download the right toolchain by itself (GOTOOLCHAIN=auto, the default); otherwise install Go %s from https://go.dev/dl/", want, installed, want))
	}
	return m, nil
}

// BuildArgs is the build command for a module.
func BuildArgs(m *project.GoModule) []string {
	if m.HasMain {
		return []string{"go", "build", "-o", m.BinName, "."}
	}
	return []string{"go", "build", "./..."}
}

// Runnables describes how to build each module. Go modules are libraries or command-line
// tools, so nothing is started; verification builds them and says so.
func Runnables(p *project.Project, noBuild bool) []project.Runnable {
	var out []project.Runnable
	for _, m := range p.GoModules {
		r := project.Runnable{Label: m.Rel + " (Go)", Dir: m.Dir, NoStart: "Go module: built, not started (run its commands by hand, see the guide)"}
		if !noBuild {
			r.Build = BuildArgs(m)
		}
		out = append(out, r)
	}
	return out
}
