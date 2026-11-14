//go:build !windows

package docker

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// FakeDockerMode configures the behavior of the fake docker test helper.
//
// Modes:
//   - "ready": `docker info` succeeds; `image inspect` and `pull` succeed.
//   - "stopped": `docker info` fails without a permission-denied message.
//   - "permission denied": `docker info` fails with "permission denied".
//   - "image download fails": `docker info` succeeds but `image inspect` and
//     `pull` fail, simulating an unavailable image download.
type FakeDockerMode string

const (
	FakeDockerReady             FakeDockerMode = "ready"
	FakeDockerStopped           FakeDockerMode = "stopped"
	FakeDockerPermissionDenied  FakeDockerMode = "permission denied"
	FakeDockerImageDownloadFail FakeDockerMode = "image download fails"
)

type FakeDocker struct {
	Dir     string
	LogPath string
}

// NewFakeDocker creates a temporary directory containing an executable `docker`
// script, prepends it to PATH for the test, and records the arguments it
// receives in a log file. It supports modes for engine ready, engine stopped,
// permission denied, and image download fails. Tests using it should skip on
// Windows because the helper writes a POSIX shell script.
func NewFakeDocker(t *testing.T, mode FakeDockerMode) *FakeDocker {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake docker helper uses a POSIX shell script; skipped on Windows")
	}

	dir := t.TempDir()
	logPath := filepath.Join(dir, "docker.log")
	scriptPath := filepath.Join(dir, "docker")
	script := fakeDockerScript(mode, logPath)
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake docker: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return &FakeDocker{Dir: dir, LogPath: logPath}
}

func (f *FakeDocker) Calls(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile(f.LogPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("read docker log: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	return lines
}

func fakeDockerScript(mode FakeDockerMode, logPath string) string {
	// $@ is recorded one call per line as space-joined args.
	base := "#!/bin/sh\n" +
		"echo \"$@\" >> " + shellQuote(logPath) + "\n" +
		"cmd=\"$1\"\n" +
		"sub=\"$2\"\n"
	switch mode {
	case FakeDockerReady:
		return base +
			"if [ \"$cmd\" = \"info\" ]; then exit 0; fi\n" +
			"if [ \"$cmd\" = \"image\" ] && [ \"$sub\" = \"inspect\" ]; then exit 0; fi\n" +
			"if [ \"$cmd\" = \"pull\" ]; then exit 0; fi\n" +
			"exit 0\n"
	case FakeDockerStopped:
		return base +
			"if [ \"$cmd\" = \"info\" ]; then echo 'Cannot connect to the Docker daemon' >&2; exit 1; fi\n" +
			"exit 0\n"
	case FakeDockerPermissionDenied:
		return base +
			"if [ \"$cmd\" = \"info\" ]; then echo 'permission denied while trying to connect to the Docker daemon socket' >&2; exit 1; fi\n" +
			"exit 0\n"
	case FakeDockerImageDownloadFail:
		return base +
			"if [ \"$cmd\" = \"info\" ]; then exit 0; fi\n" +
			"if [ \"$cmd\" = \"image\" ] && [ \"$sub\" = \"inspect\" ]; then exit 1; fi\n" +
			"if [ \"$cmd\" = \"pull\" ]; then echo 'pull failed' >&2; exit 1; fi\n" +
			"exit 0\n"
	default:
		return base + "exit 0\n"
	}
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
