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

func TestDetectFrameworks(t *testing.T) {
	tests := []struct {
		dep       string
		framework string
		port      string
	}{
		{dep: "gatsby", framework: "Gatsby", port: "8000"},
		{dep: "@docusaurus/core", framework: "Docusaurus", port: "3000"},
		{dep: "vitepress", framework: "VitePress", port: "5173"},
		{dep: "@11ty/eleventy", framework: "Eleventy", port: "8080"},
		{dep: "parcel", framework: "Parcel", port: "1234"},
	}
	for _, tt := range tests {
		t.Run(tt.dep, func(t *testing.T) {
			dir := t.TempDir()
			pkg := `{"name":"app","dependencies":{"` + tt.dep + `":"1.0.0"}}`
			if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0o644); err != nil {
				t.Fatal(err)
			}
			got, err := AnalyzePkg(dir, dir, NodeInfo{})
			if err != nil {
				t.Fatalf("AnalyzePkg: %v", err)
			}
			if got.Framework != tt.framework || got.Port != tt.port {
				t.Fatalf("framework=%q port=%q, want %q %q", got.Framework, got.Port, tt.framework, tt.port)
			}
		})
	}
}
