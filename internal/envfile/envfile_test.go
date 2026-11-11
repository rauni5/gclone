package envfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClassify(t *testing.T) {
	cases := []struct{ key, val, want string }{
		{"PORT", "3000", "ok"},
		{"LOG_LEVEL", "info", "ok"},
		{"STRIPE_API_KEY", "your_stripe_key_here", "blocking"},
		{"SOME_VAR", "", "blocking"},
		{"JWT_SECRET", "dev-secret", "review"},
		{"DATABASE_URL", "postgres://u:p@localhost:5432/app", "review"},
		{"API_URL", "https://api.example.com", "blocking"},
	}
	for _, c := range cases {
		if got, _ := classify(c.key, c.val); got != c.want {
			t.Errorf("classify(%s=%q) = %s, want %s", c.key, c.val, got, c.want)
		}
	}
}

func TestParseEnvFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env.example")
	content := "# optional: tracing\nTRACE=\nexport A=\"quoted value\"\nB=plain # trailing\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	vars := ParseEnvFile(path)
	if len(vars) != 3 {
		t.Fatalf("got %d vars, want 3", len(vars))
	}
	if vars[0].Kind != "ok" {
		t.Errorf("optional empty var should be ok, got %s", vars[0].Kind)
	}
	if vars[1].Value != "quoted value" || vars[2].Value != "plain" {
		t.Errorf("values parsed wrong: %+v", vars)
	}
}
