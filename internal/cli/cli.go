package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rauni5/gclone/internal/analyze"
	"github.com/rauni5/gclone/internal/buildinfo"
	"github.com/rauni5/gclone/internal/docker"
	"github.com/rauni5/gclone/internal/explain"
	"github.com/rauni5/gclone/internal/fsx"
	"github.com/rauni5/gclone/internal/guide"
	"github.com/rauni5/gclone/internal/plan"
	"github.com/rauni5/gclone/internal/proc"
	"github.com/rauni5/gclone/internal/project"
	"github.com/rauni5/gclone/internal/repos"
	"github.com/rauni5/gclone/internal/services"
	"github.com/rauni5/gclone/internal/ui"
	"github.com/rauni5/gclone/internal/verify"
)

const usage = `gclone - git clone, plus environment setup and a run guide

Usage:
  gclone <repo-url | user/repo | local-path> [directory] [options]
  gclone setup [directory] [options]     set up a repo that is already cloned
  gclone verify [directory] [options]    build it, start it, check it answers; exit 1 if not ready

Options:
  -n, --dry-run         Clone and analyze, show the plan, run nothing else
  -y, --yes             Do not ask for confirmation
      --ignore-scripts  Pass --ignore-scripts to the package manager
      --no-install      Skip installing dependencies
      --no-services     Do not start docker compose database services
      --install-services  Approve the first available way to start a missing
                        database without asking (default is to ask, skip on Enter)
      --clone-deps      Clone and set up other repositories this project needs
                        (for example a backend) without asking
      --no-deps         Do not look for other repositories this project needs
      --verify          Run the verification after setup without asking
      --no-build        Skip the build step when verifying
      --timeout=SECONDS How long verify waits for the app to listen (default 60)
  -v, --version         Print the version
  -h, --help            Print this help

Examples:
  gclone https://github.com/someone/app.git
  gclone someone/app my-folder --yes
  gclone someone/app --dry-run
  gclone setup ./app
`

func parseArgs(args []string) (project.Options, error) {
	var o project.Options
	var pos []string
	for _, a := range args {
		switch a {
		case "-n", "--dry-run":
			o.DryRun = true
		case "-y", "--yes":
			o.Yes = true
		case "--ignore-scripts":
			o.IgnoreScripts = true
		case "--no-install":
			o.NoInstall = true
		case "--no-services":
			o.NoServices = true
		case "--install-services":
			o.InstallServices = true
		case "--clone-deps":
			o.CloneCompanions = true
		case "--no-deps":
			o.NoCompanions = true
		case "--verify":
			o.Verify = true
		case "--no-build":
			o.NoBuild = true
		case "-v", "--version":
			fmt.Println("gclone", buildinfo.Version)
			os.Exit(0)
		case "-h", "--help":
			fmt.Print(usage)
			os.Exit(0)
		default:
			if strings.HasPrefix(a, "--timeout=") {
				n, err := strconv.Atoi(strings.TrimPrefix(a, "--timeout="))
				if err != nil || n <= 0 {
					return o, fmt.Errorf("--timeout needs a number of seconds, for example --timeout=90")
				}
				o.Timeout = n
				continue
			}
			if strings.HasPrefix(a, "-") && len(a) > 1 {
				return o, fmt.Errorf("unknown option %q", a)
			}
			pos = append(pos, a)
		}
	}
	if len(pos) > 0 && (pos[0] == "setup" || pos[0] == "verify") {
		o.Setup = pos[0] == "setup"
		o.VerifyCmd = pos[0] == "verify"
		o.Dir = "."
		if len(pos) == 2 {
			o.Dir = pos[1]
		} else if len(pos) > 2 {
			return o, fmt.Errorf("%s takes at most one directory", pos[0])
		}
		return o, nil
	}
	if len(pos) == 0 || len(pos) > 2 {
		return o, fmt.Errorf("expected a repository and an optional directory")
	}
	o.Url = pos[0]
	if len(pos) == 2 {
		o.Dir = pos[1]
	}
	return o, nil
}

func Main() {
	o, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "gclone:", err)
		fmt.Fprint(os.Stderr, "\n"+usage)
		os.Exit(2)
	}
	if err := run(o); err != nil {
		fmt.Fprintln(os.Stderr, "gclone:", err)
		os.Exit(1)
	}
}

func run(o project.Options) error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git was not found in PATH")
	}

	var abs string
	var err error
	if o.Setup || o.VerifyCmd {
		abs, err = filepath.Abs(o.Dir)
		if err != nil {
			return err
		}
		if info, e := os.Stat(abs); e != nil || !info.IsDir() {
			return fmt.Errorf("%s is not a directory", o.Dir)
		}
		if o.VerifyCmd {
			return verify.VerifyCommand(abs, o)
		}
	} else {
		url := repos.ExpandURL(o.Url)
		dir := o.Dir
		if dir == "" {
			dir = repos.RepoName(url)
		}
		if abs, err = filepath.Abs(dir); err != nil {
			return err
		}
		fmt.Printf("Cloning %s\n", url)
		if err := proc.RunStream("", true, "git", "clone", url, dir); err != nil {
			if h, ok := explain.FromError(err); ok {
				explain.Print(h)
			}
			return fmt.Errorf("git clone failed: %w", err)
		}
	}

	_, err = setupRepo(abs, o, 0)
	return err
}

// setupRepo analyzes, sets up and documents one repository. depth is 0 for the
// repository the user asked for and 1 for a companion repository; companions
// of companions are never followed.
func setupRepo(abs string, o project.Options, depth int) (*project.Project, error) {
	var subWarn []string
	if fsx.FileExists(filepath.Join(abs, ".gitmodules")) {
		subWarn = repos.InitSubmodules(abs, o)
	}

	p := analyze.AnalyzeProject(abs)
	p.Warnings = append(p.Warnings, subWarn...)
	p.URL = proc.GitOutput(abs, "remote", "get-url", "origin")
	if p.URL == "" {
		p.URL = "local repository"
	}
	p.Commit = proc.GitOutput(abs, "rev-parse", "--short", "HEAD")
	if p.Commit == "" {
		p.Commit = "unknown"
	}
	if depth == 0 && !o.NoCompanions {
		p.Companions = repos.DetectCompanions(p)
	}

	needsDocker := len(p.Needs) > 0
	for _, c := range p.Composes {
		if len(c.DBServices) > 0 {
			needsDocker = true
		}
	}
	if needsDocker {
		p.DockerState = docker.State()
	}

	steps := plan.BuildPlan(p, o)
	plan.PrintPlan(p, steps, o)
	services.PrintNeeds(p, o)
	repos.PrintCompanions(p)
	if !p.HasUnits() {
		return p, nil
	}
	if o.DryRun {
		fmt.Println("\nDry run: nothing was installed or changed.")
		return p, nil
	}
	if len(steps) > 0 && !o.Yes && !ui.AskYesNo("\nRun this plan?", true) {
		fmt.Println("Skipped. Run `gclone setup " + filepath.Base(abs) + "` later to apply it.")
		return p, nil
	}

	plan.ExecutePlan(p, steps)
	p.Executed = true
	services.ProvisionServices(p, o)
	if depth == 0 {
		repos.HandleCompanions(p, o, setupRepo)
		if o.Verify || ui.AskYesNo("\nVerify now? This builds the project, starts it for a few seconds and checks that it answers.", false) {
			p.Verify = verify.RunVerify(p, o)
		}
	}
	path, err := guide.WriteGuide(p)
	if err != nil {
		return p, fmt.Errorf("could not write the guide: %w", err)
	}
	fmt.Printf("\nGuide written to %s\n", path)
	plan.PrintNext(p)
	return p, nil
}
