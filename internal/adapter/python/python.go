// Package python is the Python adapter. It understands pip (requirements.txt, pyproject.toml,
// setup.py), Poetry, uv and Pipenv, and knows the common web frameworks: Django, FastAPI,
// Flask and Streamlit. gclone never installs into the system Python: pip projects get a
// .venv folder, and the other managers use their own environments.
package python

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/rauni5/gclone/internal/envfile"
	"github.com/rauni5/gclone/internal/fsx"
	"github.com/rauni5/gclone/internal/project"
)

var markers = []string{"requirements.txt", "pyproject.toml", "Pipfile", "setup.py", "manage.py"}

func hasMarker(dir string) bool {
	for _, m := range markers {
		if fsx.FileExists(filepath.Join(dir, m)) {
			return true
		}
	}
	return false
}

// pySkip lists folders that hold installed packages or caches, never project sources.
var pySkip = map[string]bool{"venv": true, "env": true, "__pycache__": true, "site-packages": true, "docs": true}

// FindProjects returns the directories that look like Python projects: just the root when it
// has a marker file, otherwise folders one level down.
func FindProjects(root string) []string {
	if hasMarker(root) {
		return []string{root}
	}
	var dirs []string
	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") || fsx.SkipDirs[e.Name()] || pySkip[e.Name()] {
			continue
		}
		if d := filepath.Join(root, e.Name()); hasMarker(d) {
			dirs = append(dirs, d)
		}
	}
	return dirs
}

// ---- Python versions

var pyVersionRe = regexp.MustCompile(`Python (\d+)\.(\d+)(?:\.(\d+))?`)

// Interpreter finds a usable Python 3 and returns the command to start it and its version.
func Interpreter() (cmd []string, version string) {
	cands := [][]string{{"python3"}, {"python"}}
	if runtime.GOOS == "windows" {
		cands = [][]string{{"python"}, {"py", "-3"}}
	}
	for _, c := range cands {
		if _, err := exec.LookPath(c[0]); err != nil {
			continue
		}
		out, err := exec.Command(c[0], append(c[1:], "--version")...).CombinedOutput()
		if err != nil {
			continue
		}
		if m := pyVersionRe.FindStringSubmatch(string(out)); m != nil && m[1] == "3" {
			return c, m[1] + "." + m[2]
		}
	}
	return nil, ""
}

func majorMinor(v string) (int, int) {
	m := regexp.MustCompile(`(\d+)\.(\d+)`).FindStringSubmatch(v)
	if m == nil {
		return 0, 0
	}
	a, _ := strconv.Atoi(m[1])
	b, _ := strconv.Atoi(m[2])
	return a, b
}

// Older reports whether version a is older than b, comparing major and minor only.
func Older(a, b string) bool {
	a1, a2 := majorMinor(a)
	b1, b2 := majorMinor(b)
	if a1 != b1 {
		return a1 < b1
	}
	return a2 < b2
}

// VersionHint suggests how to get the Python version the project asks for, based on the
// tools already on this machine. It never installs or switches Python itself: it only
// checks PATH with exec.LookPath and returns a sentence to show the user.
func VersionHint(version string) string {
	v := version
	if v == "" {
		v = "3"
	}
	if _, err := exec.LookPath("uv"); err == nil {
		return "Install it with: uv python install " + v
	}
	if _, err := exec.LookPath("pyenv"); err == nil {
		return "Install it with: pyenv install " + v
	}
	return "Install it from https://www.python.org/downloads/"
}

var (
	lowerRe   = regexp.MustCompile(`(?:>=|\^|~=|~|==|>)\s*(\d+\.\d+)`)
	upperRe   = regexp.MustCompile(`<\s*=?\s*(\d+\.\d+)`)
	tomlReq   = regexp.MustCompile(`(?m)^\s*requires-python\s*=\s*["']([^"']+)["']`)
	poetryPy  = regexp.MustCompile(`(?m)^\s*python\s*=\s*["']([^"']+)["']`)
	pipfilePy = regexp.MustCompile(`(?m)^\s*python_version\s*=\s*["']([^"']+)["']`)
	runtimeRe = regexp.MustCompile(`python-(\d+\.\d+)`)
)

// ParseConstraint reads a version constraint such as ">=3.10,<3.13" or "^3.11" and returns
// the lowest and the first unsupported version, either of which may be empty.
func ParseConstraint(c string) (lower, upper string) {
	if m := lowerRe.FindStringSubmatch(c); m != nil {
		lower = m[1]
	} else if m := regexp.MustCompile(`^\s*(\d+\.\d+)`).FindStringSubmatch(c); m != nil {
		lower = m[1]
	}
	if m := upperRe.FindStringSubmatch(c); m != nil {
		upper = m[1]
	}
	return
}

// WantedVersion looks for the Python version a project asks for, and says where it read it.
func WantedVersion(dir string, pyproject, pipfile string) (lower, upper, source string) {
	if b, err := os.ReadFile(filepath.Join(dir, ".python-version")); err == nil {
		if m := regexp.MustCompile(`(\d+\.\d+)`).FindStringSubmatch(string(b)); m != nil {
			return m[1], "", ".python-version"
		}
	}
	if m := tomlReq.FindStringSubmatch(pyproject); m != nil {
		l, u := ParseConstraint(m[1])
		if l != "" || u != "" {
			return l, u, "requires-python"
		}
	}
	if m := poetryPy.FindStringSubmatch(pyproject); m != nil {
		l, u := ParseConstraint(m[1])
		if l != "" || u != "" {
			return l, u, "pyproject.toml"
		}
	}
	if m := pipfilePy.FindStringSubmatch(pipfile); m != nil {
		if l, _ := ParseConstraint(m[1]); l != "" {
			return l, "", "Pipfile"
		}
	}
	if b, err := os.ReadFile(filepath.Join(dir, "runtime.txt")); err == nil {
		if m := runtimeRe.FindStringSubmatch(string(b)); m != nil {
			return m[1], "", "runtime.txt"
		}
	}
	return "", "", ""
}

// ---- dependencies

func depRe(names string) *regexp.Regexp {
	return regexp.MustCompile(`(?im)(?:^|[^\w.\-])(` + names + `)(?:$|[^\w.\-])`)
}

var frameworkRes = []struct {
	name string
	port string
	re   *regexp.Regexp
}{
	{"Django", "8000", depRe(`django`)},
	{"FastAPI", "8000", depRe(`fastapi`)},
	{"Flask", "5000", depRe(`flask`)},
	{"Streamlit", "8501", depRe(`streamlit`)},
}

var kindRes = []struct {
	kind string
	re   *regexp.Regexp
}{
	{"postgres", depRe(`psycopg2-binary|psycopg2|psycopg|asyncpg|pg8000`)},
	{"mysql", depRe(`mysqlclient|pymysql|mysql-connector-python|aiomysql`)},
	{"mongodb", depRe(`pymongo|motor|mongoengine|beanie`)},
	{"redis", depRe(`redis|aioredis|django-redis`)},
}

// readDeps returns the text of the files that list dependencies, the main pip
// requirements file, and any other requirements files found on disk. The extra
// files (e.g. requirements-dev.txt or requirements/test.txt) are reported in the
// guide but never installed, so a user who needs those tools can install them
// deliberately.
func readDeps(dir string) (all, pyproject, pipfile, reqName string, extra []string) {
	var b strings.Builder
	read := func(name string) string {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return ""
		}
		b.WriteString(string(data))
		b.WriteString("\n")
		return string(data)
	}
	reqName = ""
	seen := map[string]bool{}
	for _, n := range []string{"requirements.txt", "requirements/base.txt", "requirements/prod.txt"} {
		seen[n] = true
		if t := read(n); t != "" && reqName == "" {
			reqName = n
		}
	}
	pyproject = read("pyproject.toml")
	pipfile = read("Pipfile")
	read("setup.py")
	read("setup.cfg")

	// Extra requirements files: common names at the root, then any other .txt
	// files under requirements/. They are reported in the guide, never installed.
	for _, n := range []string{"requirements-dev.txt", "requirements/test.txt", "requirements/dev.txt"} {
		if fsx.FileExists(filepath.Join(dir, n)) {
			seen[n] = true
			extra = append(extra, n)
		}
	}
	if entries, err := os.ReadDir(filepath.Join(dir, "requirements")); err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".txt") {
				continue
			}
			rel := "requirements/" + e.Name()
			if !seen[rel] {
				extra = append(extra, rel)
			}
		}
	}
	return b.String(), pyproject, pipfile, reqName, extra
}

// Manager decides which tool installs the dependencies.
func Manager(dir, pyproject string) string {
	switch {
	case fsx.FileExists(filepath.Join(dir, "uv.lock")) || strings.Contains(pyproject, "[tool.uv"):
		return "uv"
	case fsx.FileExists(filepath.Join(dir, "poetry.lock")) || strings.Contains(pyproject, "[tool.poetry"):
		return "poetry"
	case fsx.FileExists(filepath.Join(dir, "Pipfile")):
		return "pipenv"
	}
	return "pip"
}

var (
	fastapiRe = regexp.MustCompile(`(?m)^(\w+)\s*(?::[^=]+)?=\s*FastAPI\(`)
	flaskRe   = regexp.MustCompile(`(?m)^(\w+)\s*(?::[^=]+)?=\s*Flask\(`)
	factoryRe = regexp.MustCompile(`(?m)^def[ \t]+create_app[ \t]*\([ \t]*\)[ \t]*(?:->[ \t]*[^:\r\n]+)?[ \t]*:`)
	envUse    = []*regexp.Regexp{
		regexp.MustCompile(`os\.environ\[\s*['"]([A-Za-z_][A-Za-z0-9_]*)['"]\s*\]`),
		regexp.MustCompile(`os\.environ\.get\(\s*['"]([A-Za-z_][A-Za-z0-9_]*)['"]\s*\)`),
		regexp.MustCompile(`os\.getenv\(\s*['"]([A-Za-z_][A-Za-z0-9_]*)['"]\s*\)`),
	}
	scanner = envfile.Scanner{
		Exts:     map[string]bool{".py": true},
		Patterns: envUse,
		SkipDirs: pySkip,
		Ignored: map[string]bool{"HOME": true, "PATH": true, "PWD": true, "USER": true, "TZ": true, "PYTHONPATH": true,
			"VIRTUAL_ENV": true, "DJANGO_SETTINGS_MODULE": true, "CI": true, "DEBUG": true},
	}
)

// appTarget prefers app objects in the usual entry files, preserving their search order.
// If none is found, it falls back to the first top-level, no-argument create_app factory.
// This is a static heuristic: it never imports or runs project code.
func appTarget(dir string, re *regexp.Regexp) (module, variable, file string, factory bool) {
	for _, f := range []string{"main.py", "app.py", "server.py", "api.py", "wsgi.py", "app/main.py", "app/app.py", "src/main.py", "src/app.py", "src/app/main.py"} {
		b, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			continue
		}
		if m := re.FindSubmatch(b); m != nil {
			mod := strings.ReplaceAll(strings.TrimSuffix(f, ".py"), "/", ".")
			return mod, string(m[1]), f, false
		}
		if !factory && factoryRe.Match(b) {
			module = strings.ReplaceAll(strings.TrimSuffix(f, ".py"), "/", ".")
			variable, file, factory = "create_app", f, true
		}
	}
	return
}

// Analyze reads one Python project. interpreter is the installed Python's version, or "".
func Analyze(root, dir, interpreter string) *project.PyProject {
	deps, pyproject, pipfile, reqName, extra := readDeps(dir)
	y := &project.PyProject{
		Dir: dir, Rel: fsx.Rel(root, dir), Name: filepath.Base(dir),
		Manager: Manager(dir, pyproject), Requirements: reqName,
		ExtraRequirements: extra,
		PyInstalled:       interpreter, DepKinds: map[string]string{},
	}
	if y.Rel == "." {
		y.Name = filepath.Base(root)
	}
	if m := regexp.MustCompile(`(?m)^\s*name\s*=\s*["']([^"']+)["']`).FindStringSubmatch(pyproject); m != nil {
		y.Name = m[1]
	}
	y.HasVenv = fsx.FileExists(filepath.Join(dir, ".venv"))
	y.HasTests = fsx.FileExists(filepath.Join(dir, "tests")) || fsx.FileExists(filepath.Join(dir, "test"))
	y.Pytest = depRe(`pytest`).MatchString(deps)

	for _, f := range frameworkRes {
		if f.re.MatchString(deps) {
			y.Framework, y.Port = f.name, f.port
			break
		}
	}
	if y.Framework == "" && fsx.FileExists(filepath.Join(dir, "manage.py")) {
		y.Framework, y.Port = "Django", "8000"
	}
	for _, k := range kindRes {
		if m := k.re.FindStringSubmatch(deps); m != nil {
			y.DepKinds[k.kind] = strings.ToLower(m[1])
		}
	}

	switch y.Framework {
	case "Django":
		if fsx.FileExists(filepath.Join(dir, "manage.py")) {
			y.Start = []string{"manage.py", "runserver"}
		} else {
			y.StartNote = "Django project without manage.py, start it as the README says"
		}
	case "FastAPI":
		if mod, v, _, factory := appTarget(dir, fastapiRe); mod != "" {
			y.Start = []string{"-m", "uvicorn", mod + ":" + v, "--port", "8000"}
			if factory {
				y.Start = append(y.Start, "--factory")
			}
		} else {
			y.StartNote = "could not find the FastAPI app object or create_app() factory (looked in main.py, app.py and app/main.py)"
		}
	case "Flask":
		if mod, v, _, _ := appTarget(dir, flaskRe); mod != "" {
			y.Start = []string{"-m", "flask", "--app", mod + ":" + v, "run", "--port", "5000"}
		} else {
			y.StartNote = "could not find the Flask app object or create_app() factory (looked in app.py, main.py and app/)"
		}
	case "Streamlit":
		for _, f := range []string{"streamlit_app.py", "app.py", "main.py"} {
			if fsx.FileExists(filepath.Join(dir, f)) {
				y.Start = []string{"-m", "streamlit", "run", f, "--server.headless", "true", "--server.port", "8501"}
				break
			}
		}
		if y.Start == nil {
			y.StartNote = "could not find the Streamlit entry file"
		}
	default:
		y.StartNote = "no web framework detected, run it as the README says"
	}

	for _, n := range envfile.EnvFileNames {
		if fsx.FileExists(filepath.Join(dir, n)) {
			y.EnvExample = n
			y.EnvVars = envfile.ParseEnvFile(filepath.Join(dir, n))
			break
		}
	}
	if y.EnvExample == "" && fsx.FileExists(filepath.Join(dir, ".env")) {
		y.EnvVars = envfile.ParseEnvFile(filepath.Join(dir, ".env"))
	}

	lower, upper, src := WantedVersion(dir, pyproject, pipfile)
	y.PyWanted, y.PyUpper, y.PySource = lower, upper, src
	switch {
	case interpreter == "":
		want := "Python 3"
		if lower != "" {
			want = "Python " + lower + " or newer"
		}
		y.Warnings = append(y.Warnings, "Python was not found in PATH. Install "+want+" from https://www.python.org/downloads/")
	case lower != "" && Older(interpreter, lower):
		y.Warnings = append(y.Warnings, "the project asks for Python "+lower+" or newer ("+src+") but Python "+interpreter+" is installed. "+VersionHint(lower))
	case upper != "" && !Older(interpreter, upper):
		y.Warnings = append(y.Warnings, "the project supports Python below "+upper+" ("+src+") but Python "+interpreter+" is installed. Some packages may fail to install; consider an older Python. "+VersionHint(upper))
	}
	return y
}

// ScanEnv finds environment variables the code reads, minus those a nested project owns.
func ScanEnv(dir string, exclude []string) []string { return scanner.Scan(dir, exclude) }

// VenvPython is the interpreter inside the project's .venv.
func VenvPython(dir string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(dir, ".venv", "Scripts", "python.exe")
	}
	return filepath.Join(dir, ".venv", "bin", "python")
}

// shownVenvPython is the same path as a person would type it from the project folder.
func shownVenvPython() string {
	if runtime.GOOS == "windows" {
		return `.venv\Scripts\python.exe`
	}
	return ".venv/bin/python"
}

// Runner returns the command that runs Python code for this project: the venv's python for
// pip, or the manager's own "run" for Poetry, uv and Pipenv. shown is the form for people.
func Runner(y *project.PyProject) (argv []string, shown string) {
	switch y.Manager {
	case "poetry":
		return []string{"poetry", "run", "python"}, "poetry run python"
	case "uv":
		return []string{"uv", "run", "python"}, "uv run python"
	case "pipenv":
		return []string{"pipenv", "run", "python"}, "pipenv run python"
	}
	return []string{VenvPython(y.Dir)}, shownVenvPython()
}

// InstallArgs returns the command (after the manager or python) that installs dependencies.
func InstallArgs(y *project.PyProject) (argv []string, shown string) {
	switch y.Manager {
	case "poetry":
		return []string{"poetry", "install"}, "poetry install"
	case "uv":
		return []string{"uv", "sync"}, "uv sync"
	case "pipenv":
		return []string{"pipenv", "install", "--dev"}, "pipenv install --dev"
	}
	if y.Requirements != "" {
		return []string{VenvPython(y.Dir), "-m", "pip", "install", "-r", y.Requirements}, shownVenvPython() + " -m pip install -r " + y.Requirements
	}
	return []string{VenvPython(y.Dir), "-m", "pip", "install", "-e", "."}, shownVenvPython() + " -m pip install -e ."
}
