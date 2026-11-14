package analyze

import (
	"path/filepath"
	"testing"
	"os"
	"path/filepath"
	"strings"
)

func TestAnalyzeProjectNodeExpress(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("..", "..", "testdata", "node-express"))
	if err != nil {
		t.Fatal(err)
	}
	p := AnalyzeProject(dir)
	if len(p.Pkgs) != 1 {
		t.Fatalf("want 1 package, got %d", len(p.Pkgs))
	}
	pkg := p.Pkgs[0]
	if pkg.PM != "npm" || pkg.Framework != "Express" {
		t.Errorf("got PM=%q framework=%q", pkg.PM, pkg.Framework)
	}
	if len(p.Needs) != 1 || p.Needs[0].Kind != "mongodb" {
		t.Errorf("want one MongoDB need, got %+v", p.Needs)
	}
	if len(pkg.MissingVars) != 1 || pkg.MissingVars[0] != "API_KEY" {
		t.Errorf("want API_KEY missing from the example file, got %v", pkg.MissingVars)
	}
}

func TestAnalyzeProjectGoModule(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("..", "..", "testdata", "go-cli"))
	if err != nil {
		t.Fatal(err)
	}
	p := AnalyzeProject(dir)
	if len(p.GoModules) != 1 {
		t.Fatalf("want 1 Go module, got %d", len(p.GoModules))
	}
	m := p.GoModules[0]
	if m.Module != "example.com/go-cli" || m.GoWanted != "1.21" || !m.HasMain || m.BinName != "go-cli" {
		t.Errorf("unexpected module: %+v", m)
	}
	for _, s := range p.OtherStacks {
		t.Errorf("Go should not be listed as an unsupported stack: %s", s)
	}
}

func TestAnalyzeProjectSpringMaven(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("..", "..", "testdata", "spring-maven"))
	if err != nil {
		t.Fatal(err)
	}
	p := AnalyzeProject(dir)
	if len(p.JavaProjects) != 1 {
		t.Fatalf("want 1 Java project, got %d", len(p.JavaProjects))
	}
	j := p.JavaProjects[0]
	if j.Build != "maven" || !j.SpringBoot || j.JavaWanted != 17 || j.Port != "8081" || !j.PortFromConfig {
		t.Errorf("unexpected project: %+v", j)
	}
	if len(p.Needs) != 1 || p.Needs[0].Kind != "postgres" || p.Needs[0].User != "demo" || p.Needs[0].DB != "demo" {
		t.Errorf("want a PostgreSQL need with user demo, got %+v", p.Needs)
	}
	blocking := false
	for _, v := range j.Config {
		if v.Key == "spring.datasource.password" && v.Kind == "blocking" {
			blocking = true
		}
	}
	if !blocking {
		t.Error("spring.datasource.password with no default should be blocking")
	}
	for _, s := range p.OtherStacks {
		t.Errorf("Java should not be listed as unsupported: %s", s)
	}
}


func TestAnalyzeProjectPortConflicts(t *testing.T) {
    tests := []struct {
        name       string
        clientDep  string
        serverDep  string
        wantWarn   bool
    }{
        {
            name:      "clashing ports",
            clientDep: `"next": "*"`,
            serverDep: `"@nestjs/core": "*"`,
            wantWarn:  true,
        },
        {
            name:      "non-clashing ports",
            clientDep: `"next": "*"`,
            serverDep: `"vite": "*"`,
            wantWarn:  false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            root := t.TempDir()

            packages := map[string]string{
                "client": tt.clientDep,
                "server": tt.serverDep,
            }

            for dir, dep := range packages {
                pkgDir := filepath.Join(root, dir)

                if err := os.MkdirAll(pkgDir, 0755); err != nil {
                    t.Fatal(err)
                }

                content := `{
                    "name": "` + dir + `",
                    "dependencies": {
                        ` + dep + `
                    }
                }`

                if err := os.WriteFile(
                    filepath.Join(pkgDir, "package.json"),
                    []byte(content),
                    0644,
                ); err != nil {
                    t.Fatal(err)
                }
            }

            p := AnalyzeProject(root)

            found := false
            for _, warning := range p.Warnings {
                if strings.Contains(warning, "Multiple packages expect port") {
                    found = true
                    break
                }
            }

            if found != tt.wantWarn {
                t.Errorf("want warning %v, got %v; warnings: %v",
                    tt.wantWarn, found, p.Warnings)
            }
        })
    }
}
