package fsx

import (
	"os"
	"path/filepath"
	"strings"
)

var SkipDirs = map[string]bool{
	"node_modules": true, ".git": true, "dist": true, "build": true, ".next": true, "vendor": true,
}

func IsInside(parent, child string) bool {
	r, err := filepath.Rel(parent, child)
	return err == nil && r != "." && !strings.HasPrefix(r, "..")
}

func FileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func Rel(root, p string) string {
	r, err := filepath.Rel(root, p)
	if err != nil {
		return p
	}
	return filepath.ToSlash(r)
}

func CopyFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o600)
}
