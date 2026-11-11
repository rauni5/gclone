package node

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectPM(t *testing.T) {
	dir := t.TempDir()
	if pm, lock := detectPM(dir, ""); pm != "npm" || lock {
		t.Errorf("empty dir: %s %v", pm, lock)
	}
	_ = os.WriteFile(filepath.Join(dir, "pnpm-lock.yaml"), nil, 0o644)
	if pm, lock := detectPM(dir, ""); pm != "pnpm" || !lock {
		t.Errorf("pnpm lock: %s %v", pm, lock)
	}
	if pm, _ := detectPM(dir, "yarn@4.0.0"); pm != "yarn" {
		t.Errorf("packageManager field should win, got %s", pm)
	}
}
