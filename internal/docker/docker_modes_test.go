//go:build !windows

package docker

import (
	"runtime"
	"strings"
	"testing"
)

func skipOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake docker helper uses a POSIX shell script; skipped on Windows")
	}
}

func TestState(t *testing.T) {
	skipOnWindows(t)
	tests := []struct {
		name string
		mode FakeDockerMode
		want string
	}{
		{name: "engine ready", mode: FakeDockerReady, want: "ready"},
		{name: "engine stopped", mode: FakeDockerStopped, want: "stopped"},
		{name: "permission denied", mode: FakeDockerPermissionDenied, want: "denied"},
		{name: "image download fails", mode: FakeDockerImageDownloadFail, want: "ready"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fd := NewFakeDocker(t, tt.mode)
			if got := State(); got != tt.want {
				t.Fatalf("State() = %q, want %q", got, tt.want)
			}
			calls := strings.Join(fd.Calls(t), "\n")
			if !strings.Contains(calls, "info --format") {
				t.Fatalf("expected docker info call to be recorded, got %q", calls)
			}
		})
	}
}

func TestEnsureImage(t *testing.T) {
	skipOnWindows(t)

	t.Run("engine ready image available", func(t *testing.T) {
		fd := NewFakeDocker(t, FakeDockerReady)
		if err := EnsureImage("postgres:16"); err != nil {
			t.Fatalf("EnsureImage: %v", err)
		}
		calls := strings.Join(fd.Calls(t), "\n")
		if !strings.Contains(calls, "image inspect postgres:16") {
			t.Fatalf("expected image inspect call, got %q", calls)
		}
	})

	t.Run("image download fails", func(t *testing.T) {
		fd := NewFakeDocker(t, FakeDockerImageDownloadFail)
		err := EnsureImage("postgres:16")
		if err == nil {
			t.Fatal("expected EnsureImage to fail when pull fails")
		}
		calls := strings.Join(fd.Calls(t), "\n")
		if !strings.Contains(calls, "image inspect postgres:16") || !strings.Contains(calls, "pull postgres:16") {
			t.Fatalf("expected inspect and pull calls, got %q (err=%v)", calls, err)
		}
		if !strings.Contains(err.Error(), "could not download image postgres:16") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
