package python

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rauni5/gclone/internal/project"
)

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	p := filepath.Join(dir, name)
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestOlder(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{{"3.9", "3.10", true}, {"3.12", "3.10", false}, {"3.10", "3.10", false}, {"2.7", "3.8", true}}
	for _, c := range cases {
		if got := Older(c.a, c.b); got != c.want {
			t.Errorf("Older(%s,%s)=%v", c.a, c.b, got)
		}
	}
}

func TestFlask(t *testing.T) {
	d := t.TempDir()
	write(t, d, "requirements.txt", "flask>=3\n")
	write(t, d, "app.py", "from flask import Flask\napp = Flask(__name__)\n")
	y := Analyze(d, d, "3.12")
	if y.Framework != "Flask" || y.Manager != "pip" || y.Port != "5000" {
		t.Fatalf("got %+v", y)
	}
	if got := strings.Join(y.Start, " "); !strings.Contains(got, "flask") || !strings.Contains(got, "app:app") {
		t.Errorf("start = %q", got)
	}
}

func TestAppTargets(t *testing.T) {
	for _, framework := range []string{"Flask", "FastAPI"} {
		t.Run(framework, func(t *testing.T) {
			instance := "application = " + framework + "()\n"
			factory := "def create_app():\n    return " + framework + "()\n"
			cases := []struct {
				name, main, app, target string
				factory                 bool
			}{
				{"instance", instance, "", "main:application", false},
				{"annotated instance", "application: " + framework + " = " + framework + "()\n", "", "main:application", false},
				{"factory", factory, "", "main:create_app", true},
				{"annotated factory", "def create_app() -> " + framework + ":\n    return " + framework + "()\n", "", "main:create_app", true},
				{"whitespace", "def\tcreate_app ( \t) :\r\n    return " + framework + "()\r\n", "", "main:create_app", true},
				{"local instance", "def create_app():\n    " + instance + "    return application\n", "", "main:create_app", true},
				{"same file instance wins", factory + instance, "", "main:application", false},
				{"later instance wins", factory, instance, "app:application", false},
				{"first factory wins", factory, factory, "main:create_app", true},
				{"instance named create_app", "create_app = " + framework + "()\n", "", "main:create_app", false},
				{"comment", "# " + factory, "", "", false},
				{"nested function", "def outer():\n    def create_app():\n        return " + framework + "()\n", "", "", false},
				{"class method", "class App:\n    def create_app():\n        return " + framework + "()\n", "", "", false},
				{"async function", "async " + factory, "", "", false},
				{"required argument", "def create_app(config):\n    return " + framework + "()\n", "", "", false},
				{"different name", "def create_application():\n    return " + framework + "()\n", "", "", false},
				{"missing target", "", "", "", false},
			}
			for _, c := range cases {
				t.Run(c.name, func(t *testing.T) {
					d := t.TempDir()
					write(t, d, "requirements.txt", strings.ToLower(framework)+"\n")
					write(t, d, "main.py", c.main)
					write(t, d, "app.py", c.app)
					y := Analyze(d, d, "3.12")
					var want []string
					if c.target != "" {
						if framework == "Flask" {
							want = []string{"-m", "flask", "--app", c.target, "run", "--port", "5000"}
						} else {
							want = []string{"-m", "uvicorn", c.target, "--port", "8000"}
							if c.factory {
								want = append(want, "--factory")
							}
						}
					}
					if !slices.Equal(y.Start, want) {
						t.Errorf("start = %q, want %q", y.Start, want)
					}
					if c.target == "" && y.StartNote == "" {
						t.Error("missing target should explain why it cannot start")
					}
				})
			}
		})
	}
}

func TestAppFactoryFixtures(t *testing.T) {
	for _, c := range []struct {
		fixture, framework, start string
		port                      int
	}{
		{"python-flask-factory", "Flask", "-m flask --app app:create_app run --port 5000", 5000},
		{"python-fastapi-factory", "FastAPI", "-m uvicorn app.main:create_app --port 8000 --factory", 8000},
	} {
		t.Run(c.fixture, func(t *testing.T) {
			d, err := filepath.Abs(filepath.Join("..", "..", "..", "testdata", c.fixture))
			if err != nil {
				t.Fatal(err)
			}
			y := Analyze(d, d, "3.12")
			if y.Framework != c.framework || strings.Join(y.Start, " ") != c.start {
				t.Fatalf("got %+v", y)
			}
			p := &project.Project{Dir: d, PyProjects: []*project.PyProject{y}}
			a := Adapter{}
			rs := a.Runnables(p, false)
			want := append([]string{VenvPython(d)}, y.Start...)
			if len(rs) != 1 || !slices.Equal(rs[0].Start, want) || rs[0].Port != c.port || rs[0].Dir != d {
				t.Fatalf("runnables = %+v", rs)
			}
			next := a.Next(p)
			shown := shownVenvPython() + " " + c.start
			if len(next) != 1 || next[0] != shown || !strings.Contains(a.Guide(p), shown) {
				t.Errorf("factory command missing from next steps or guide: %v", next)
			}
		})
	}
}

func TestDjango(t *testing.T) {
	d := t.TempDir()
	write(t, d, "requirements.txt", "Django==5.0\n")
	write(t, d, "manage.py", "")
	y := Analyze(d, d, "3.12")
	if y.Framework != "Django" || y.Port != "8000" || strings.Join(y.Start, " ") != "manage.py runserver" {
		t.Fatalf("got %+v", y)
	}
}

func TestFastAPIAndStreamlit(t *testing.T) {
	d := t.TempDir()
	write(t, d, "requirements.txt", "fastapi\nuvicorn\n")
	write(t, d, "main.py", "from fastapi import FastAPI\napp = FastAPI()\n")
	if y := Analyze(d, d, "3.12"); y.Framework != "FastAPI" || y.Port != "8000" {
		t.Fatalf("fastapi: %+v", y)
	}
	s := t.TempDir()
	write(t, s, "requirements.txt", "streamlit\n")
	write(t, s, "app.py", "import streamlit as st\n")
	if y := Analyze(s, s, "3.12"); y.Framework != "Streamlit" || y.Port != "8501" {
		t.Fatalf("streamlit: %+v", y)
	}
}

func TestManagerChoice(t *testing.T) {
	for file, want := range map[string]string{"poetry.lock": "poetry", "uv.lock": "uv", "Pipfile": "pipenv"} {
		d := t.TempDir()
		write(t, d, file, "")
		write(t, d, "pyproject.toml", "[project]\nname=\"x\"\n")
		if got := Manager(d, "[project]\nname=\"x\"\n"); got != want {
			t.Errorf("%s: got %s want %s", file, got, want)
		}
	}
}

func TestVersionWarning(t *testing.T) {
	d := t.TempDir()
	write(t, d, "pyproject.toml", "[project]\nname=\"x\"\nrequires-python = \">=3.12\"\n")
	y := Analyze(d, d, "3.9")
	if len(y.Warnings) == 0 {
		t.Errorf("expected a warning for Python 3.9 against >=3.12")
	}
}

func TestFindProjectsSkipsVenv(t *testing.T) {
	d := t.TempDir()
	write(t, d, "requirements.txt", "flask\n")
	write(t, d, ".venv/lib/site-packages/x/requirements.txt", "junk\n")
	if got := FindProjects(d); len(got) != 1 {
		t.Errorf("got %v", got)
	}
}

// TestExtraRequirementsDetected verifies that requirements files other than the
// main one are found but not treated as the install target. Issue #32.
func TestExtraRequirementsDetected(t *testing.T) {
	d := t.TempDir()
	write(t, d, "requirements.txt", "flask\n")
	write(t, d, "requirements-dev.txt", "pytest\n")
	write(t, d, "requirements/test.txt", "coverage\n")

	y := Analyze(d, d, "3.11")
	if y.Requirements != "requirements.txt" {
		t.Fatalf("main requirements = %q", y.Requirements)
	}
	want := map[string]bool{
		"requirements-dev.txt":  true,
		"requirements/test.txt": true,
	}
	if len(y.ExtraRequirements) != len(want) {
		t.Fatalf("extra = %v, want %d entries", y.ExtraRequirements, len(want))
	}
	for _, r := range y.ExtraRequirements {
		if !want[r] {
			t.Fatalf("unexpected extra requirement %q", r)
		}
		delete(want, r)
	}
	if len(want) != 0 {
		t.Fatalf("missing extra requirements: %v", want)
	}
}
