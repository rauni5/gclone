package docker

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/rauni5/gclone/internal/fsx"
	"github.com/rauni5/gclone/internal/installer"
	"github.com/rauni5/gclone/internal/proc"
	"github.com/rauni5/gclone/internal/project"
	"github.com/rauni5/gclone/internal/ui"
)

func dockerExeName() string {
	if runtime.GOOS == "windows" {
		return "docker.exe"
	}
	return "docker"
}

// findDockerBin adds well-known Docker locations to PATH for this process, so a
// Docker that was just installed (and is not on PATH yet) can be used.
func findDockerBin() {
	if proc.HasBin("docker") {
		return
	}
	var dirs []string
	switch runtime.GOOS {
	case "windows":
		dirs = []string{filepath.Join(os.Getenv("ProgramFiles"), "Docker", "Docker", "resources", "bin")}
	case "darwin":
		dirs = []string{"/Applications/Docker.app/Contents/Resources/bin", "/usr/local/bin", "/opt/homebrew/bin"}
	default:
		dirs = []string{"/usr/bin", "/usr/local/bin", "/snap/bin"}
	}
	for _, d := range dirs {
		if fsx.FileExists(filepath.Join(d, dockerExeName())) {
			_ = os.Setenv("PATH", d+string(os.PathListSeparator)+os.Getenv("PATH"))
			return
		}
	}
}

// dockerState reports "ready", "missing" (not installed), "stopped" (installed
// but the engine is not answering) or "denied" (running, but not for this user).
func State() string {
	findDockerBin()
	if !proc.HasBin("docker") {
		return "missing"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var out bytes.Buffer
	cmd := exec.CommandContext(ctx, "docker", "info", "--format", "{{.ServerVersion}}")
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err == nil {
		return "ready"
	}
	if strings.Contains(strings.ToLower(out.String()), "permission denied") {
		return "denied"
	}
	return "stopped"
}

func Note(state string) string {
	switch state {
	case "stopped":
		return "Docker is not running, you will be asked to start it"
	case "missing":
		return "Docker is not installed, you will be asked about installing it"
	case "denied":
		return "Docker is running but this user has no permission to use it"
	}
	return ""
}

func PlanTag(state string) string {
	if n := Note(state); n != "" {
		return " [" + n + "]"
	}
	return ""
}

const DockerDeniedMsg = "Docker is running, but your user has no permission to use it (permission denied). Add your user to the docker group and log in again, or run gclone with sudo."

func SudoPrefix() ([]string, bool) {
	if os.Geteuid() == 0 {
		return nil, true
	}
	if proc.HasBin("sudo") {
		return []string{"sudo"}, true
	}
	return nil, false
}

func startDocker() error {
	switch runtime.GOOS {
	case "windows":
		exe := filepath.Join(os.Getenv("ProgramFiles"), "Docker", "Docker", "Docker Desktop.exe")
		if !fsx.FileExists(exe) {
			return fmt.Errorf("docker desktop was not found at %s, start Docker manually", exe)
		}
		return exec.Command(exe).Start()
	case "darwin":
		return exec.Command("open", "-a", "Docker").Run()
	default:
		prefix, ok := SudoPrefix()
		if !ok {
			return fmt.Errorf("cannot start the Docker service without root or sudo, run `systemctl start docker` as an administrator")
		}
		args := append(append([]string{}, prefix...), "systemctl", "start", "docker")
		return proc.RunStream("", true, args[0], args[1:]...)
	}
}

func waitDocker(d time.Duration) bool {
	end := time.Now().Add(d)
	for {
		switch State() {
		case "ready":
			fmt.Println()
			return true
		case "denied":
			fmt.Println("\n" + DockerDeniedMsg)
			return false
		}
		if time.Now().After(end) {
			fmt.Println()
			return false
		}
		fmt.Print(".")
		time.Sleep(3 * time.Second)
	}
}

func StartAndWait() bool {
	findDockerBin()
	fmt.Println("[..] Starting Docker (this can take a minute)")
	if err := startDocker(); err != nil {
		fmt.Println("[!!]", err)
		return false
	}
	if waitDocker(150 * time.Second) {
		fmt.Println("[ok] Docker is running")
		return true
	}
	fmt.Println("[!!] Docker did not become ready. Open Docker Desktop once, accept its terms, wait until it says it is running, then run `gclone setup` again.")
	return false
}

func InstallOption() *installer.Option {
	switch runtime.GOOS {
	case "windows":
		if proc.HasBin("winget") {
			return &installer.Option{
				Manager: "winget",
				Cmds:    [][]string{{"winget", "install", "-e", "--id", "Docker.DockerDesktop", "--accept-package-agreements", "--accept-source-agreements"}},
				Note:    "Docker Desktop is a large download, needs virtualization (WSL 2), may need a restart, and has its own license terms.",
			}
		}
	case "darwin":
		if proc.HasBin("brew") {
			return &installer.Option{
				Manager: "Homebrew",
				Cmds:    [][]string{{"brew", "install", "--cask", "docker"}},
				Note:    "Docker Desktop is a large download and has its own license terms.",
			}
		}
	case "linux":
		prefix, ok := SudoPrefix()
		if ok && proc.HasBin("apt-get") {
			with := func(args ...string) []string { return append(append([]string{}, prefix...), args...) }
			return &installer.Option{
				Manager: "apt",
				Cmds: [][]string{
					with("apt-get", "update"),
					with("apt-get", "install", "-y", "docker.io"),
				},
				Note: "This needs administrator rights, and your user may need to join the docker group afterwards.",
			}
		}
	}
	return nil
}

// ensureDocker makes sure the Docker engine answers, asking before it installs
// or starts anything. It reports whether Docker is usable.
func Ensure(o project.Options) bool {
	switch State() {
	case "ready":
		return true
	case "denied":
		fmt.Println(DockerDeniedMsg)
		return false
	case "missing":
		inst := InstallOption()
		if inst == nil {
			fmt.Println("Docker is not installed and gclone cannot install it on this system. Get it from https://docs.docker.com/get-docker/ and run `gclone setup` again.")
			return false
		}
		if !ui.AskYesNo(fmt.Sprintf("Docker is not installed. Install it with %s? %s", inst.Manager, inst.Note), false) {
			return false
		}
		if err := inst.Run(); err != nil {
			fmt.Println("[!!] Docker install failed:", err)
			return false
		}
		return StartAndWait()
	default:
		if !o.InstallServices && !ui.AskYesNo("Docker is installed but not running. Start it?", false) {
			return false
		}
		return StartAndWait()
	}
}

// ensureImage downloads an image first so a failed download gets a clear message.
func EnsureImage(image string) error {
	if exec.Command("docker", "image", "inspect", image).Run() == nil {
		return nil
	}
	fmt.Printf("Downloading image %s (this can take a few minutes)...\n", image)
	if err := proc.RunStream("", false, "docker", "pull", image); err != nil {
		return fmt.Errorf("could not download image %s (no internet, a proxy, or a Docker Hub rate limit; try `docker login`): %w", image, err)
	}
	return nil
}

func ComposeBase() ([]string, error) {
	if exec.Command("docker", "compose", "version").Run() == nil {
		return []string{"docker", "compose"}, nil
	}
	if proc.HasBin("docker-compose") {
		return []string{"docker-compose"}, nil
	}
	return nil, fmt.Errorf("neither the `docker compose` plugin nor docker-compose was found")
}
