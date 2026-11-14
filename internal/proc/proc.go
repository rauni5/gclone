package proc

import (
	"io"
	"os"
	"os/exec"
	"strings"
)

// RunError is returned by RunStream when the command fails. It prints like the underlying
// error, and also carries the last part of the command's output so callers can explain it.
type RunError struct {
	Err    error
	Output string
}

func (e *RunError) Error() string { return e.Err.Error() }
func (e *RunError) Unwrap() error { return e.Err }

const tailLimit = 16 << 10

type tail struct{ buf []byte }

func (t *tail) Write(p []byte) (int, error) {
	t.buf = append(t.buf, p...)
	if len(t.buf) > tailLimit {
		t.buf = t.buf[len(t.buf)-tailLimit:]
	}
	return len(p), nil
}

// RunStream runs a command, showing its output as it happens.
func RunStream(dir string, stdin bool, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	t := &tail{}
	// The same writer for both streams keeps their order and avoids concurrent writes.
	w := io.MultiWriter(os.Stdout, t)
	cmd.Stdout = w
	cmd.Stderr = w
	if stdin {
		cmd.Stdin = os.Stdin
	}
	if err := cmd.Run(); err != nil {
		return &RunError{Err: err, Output: string(t.buf)}
	}
	return nil
}

func GitOutput(dir string, args ...string) string {
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func HasBin(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func AnyBin(names []string) bool {
	for _, n := range names {
		if HasBin(n) {
			return true
		}
	}
	return false
}
