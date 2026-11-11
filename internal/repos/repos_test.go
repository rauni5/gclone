package repos

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rauni5/gclone/internal/adapter/node"
	"github.com/rauni5/gclone/internal/compose"
	"github.com/rauni5/gclone/internal/project"
)

func TestReadmeCompanions(t *testing.T) {
	dir := t.TempDir()
	readme := "Backend repo: https://github.com/acme/shop-api\n" +
		"Built with [Express](https://github.com/expressjs/express) for the backend.\n" +
		"CI uses https://github.com/actions/checkout\n"
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(readme), 0o644); err != nil {
		t.Fatal(err)
	}
	got := readmeCompanions(dir, "acme/shop-web")
	if len(got) != 1 || got[0].URL != "https://github.com/acme/shop-api.git" {
		t.Fatalf("unexpected companions: %+v", got)
	}
}

func TestAPIRefs(t *testing.T) {
	vars := []project.EnvVar{{Key: "VITE_API_URL", Value: "http://localhost:4000", Kind: "ok"}}
	p := &project.Project{Pkgs: []*project.Pkg{{Rel: ".", Framework: "Vite", EnvVars: vars}}}
	refs := apiRefs(p)
	if len(refs) != 1 || refs[0].Port != 4000 {
		t.Fatalf("expected one ref on port 4000, got %+v", refs)
	}
	p.Pkgs[0].Framework = "Next.js" // has its own server, so no missing backend
	if refs := apiRefs(p); len(refs) != 0 {
		t.Errorf("full-stack framework should not report a missing backend: %+v", refs)
	}
}

func TestComposeOutsideContext(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "docker-compose.yml")
	content := "services:\n  api:\n    build:\n      context: ../backend\n  db:\n    image: postgres:16\n"
	_ = os.WriteFile(path, []byte(content), 0o644)
	c := compose.ParseCompose(dir, path)
	if c == nil || len(c.Outside) != 1 || c.Outside[0] != "../backend" {
		t.Fatalf("outside context not detected: %+v", c)
	}
}

func TestLocalDeps(t *testing.T) {
	dir := t.TempDir()
	pj := `{"name":"web","dependencies":{"shared":"file:../shared","react":"^18.0.0"}}`
	_ = os.WriteFile(filepath.Join(dir, "package.json"), []byte(pj), 0o644)
	pkg, err := node.AnalyzePkg(dir, dir, node.NodeInfo{Major: 22})
	if err != nil {
		t.Fatal(err)
	}
	if pkg.LocalDeps["shared"] != "../shared" || len(pkg.LocalDeps) != 1 {
		t.Errorf("local deps wrong: %+v", pkg.LocalDeps)
	}
}

func TestExpandURLAndRepoName(t *testing.T) {
	if got := ExpandURL("someone/app"); got != "https://github.com/someone/app.git" {
		t.Errorf("shorthand expanded to %s", got)
	}
	if got := ExpandURL("https://example.com/x/y.git"); got != "https://example.com/x/y.git" {
		t.Errorf("url changed: %s", got)
	}
	for in, want := range map[string]string{
		"https://github.com/a/b.git": "b",
		"git@github.com:a/b.git":     "b",
		"https://github.com/a/b/":    "b",
	} {
		if got := RepoName(in); got != want {
			t.Errorf("repoName(%s) = %s, want %s", in, got, want)
		}
	}
}
