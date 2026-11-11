package compose

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseCompose(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "docker-compose.yml")
	content := "services:\n  db:\n    image: postgres:16\n  admin:\n    image: mongo-express\n  cache:\n    image: \"redis:7\"\n  web:\n    build: .\n"
	_ = os.WriteFile(path, []byte(content), 0o644)
	c := ParseCompose(dir, path)
	if c == nil || len(c.DBServices) != 2 || c.DBServices[0] != "db" || c.DBServices[1] != "cache" {
		t.Fatalf("unexpected db services: %+v", c)
	}
}
