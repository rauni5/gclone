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
