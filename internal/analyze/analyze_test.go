package analyze

import (
	"path/filepath"
	"testing"
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
