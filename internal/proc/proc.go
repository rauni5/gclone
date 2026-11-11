package proc

import (
	"os"
	"os/exec"
	"strings"
)

func RunStream(dir string, stdin bool, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if stdin {
		cmd.Stdin = os.Stdin
	}
	return cmd.Run()
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
