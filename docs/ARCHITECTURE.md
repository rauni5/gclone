# Architecture

This page is for people changing gclone. It describes what exists today and, clearly
marked, what is planned.

## The flow

`setupRepo` in `internal/cli` runs one repository through these stages. Each stage reads the
`Project` struct built by the one before.

```
clone ──> analyze ──> plan ──> confirm ──> execute ──> services ──> other repos ──> verify ──> guide
```

| Stage | Code | Touches the machine? |
|---|---|---|
| Clone | `run` in `internal/cli` | Yes, `git clone` (plus submodules, after asking) |
| Analyze | `analyze.AnalyzeProject` | No. Reads files only |
| Plan | `plan.BuildPlan` | No. Builds a list of steps |
| Confirm | `ui.AskYesNo` | Prompt, default yes |
| Execute | `plan.ExecutePlan` | Yes. Copies `.env`, starts compose services, installs dependencies |
| Services | `services.ProvisionServices` | Only after a prompt (or `--install-services`) |
| Other repos | `repos.HandleCompanions` | Only after a prompt (or `--clone-deps`) |
| Verify | `verify.RunVerify` | Runs project code, only after a prompt (or `--verify`) |
| Guide | `guide.WriteGuide` | Writes `SETUP_GUIDE.md` |

The split between analyze (pure) and execute (side effects) is the most important rule in the
codebase. New detection should be testable with files in a temporary directory.

## Key types (`internal/project`)

- **`Project`**: everything known about a repository: packages, compose files, needed
  services, other repositories, warnings, the steps that ran and the verification report.
- **`Pkg`**: one Node.js package directory: package manager, Node version wanted, scripts,
  framework, env variables, database hints.
- **`Need`**: a database or service the project depends on, with the port,
  credentials taken from the example URL, and whether it is remote.
- **`Companion`**: another repository the project seems to need.
- **`plan.Planned`**: one step to run, with its command text for display.
- **`Runnable`**: how one part of a repository is built and started. This is
  the seam other ecosystems will use.
- **`Report` / `Check`**: the verification result.

## Invariants

1. Ask before installing, starting, cloning or running project code. Risky prompts default
   to no.
2. Never overwrite or delete user files. `.env` is created only when missing.
3. Never print or write secret values. Names and notes only.
4. Run external programs with `exec.Command(name, args...)`.
5. Output is plain text with the markers `[ok]`, `[..]`, `[!!]`, `[warn]`, `[FAIL]`, `[skip]`.
6. A check that cannot be confirmed is a warning, never a pass.

## How external tools are tested

There is no Docker engine, winget, brew or apt in most test environments. Tests either work
on pure functions (parsing, classification, port detection) or use real short-lived Node
processes (`internal/verify/verify_test.go`). Docker flows were exercised with a stand-in `docker` script on
`PATH`. That proves the control flow, not real Docker behaviour, which is why the README marks
those paths as experimental.

## Planned: an adapter interface

Node.js logic lives in `internal/adapter/node` and Go logic in `internal/adapter/golang`.
They are called directly, and Go modules are kept in `Project.GoModules` next to the Node
`Project.Pkgs`. The plan is to introduce a shared interface when the Java adapter is added, so
that the interface is shaped by three real cases instead of guessed. Both adapters already
produce `project.Runnable` values, which is the one seam `verify` uses today.

A sketch, **not implemented and open to change**:

```go
// Adapter knows one ecosystem (Node, Java, Flutter, ...).
type Adapter interface {
    Name() string
    // Detect finds the parts of the repository this adapter understands.
    Detect(root string) ([]Unit, error)
    // Plan returns the steps to set a unit up (install, env, migrations).
    Plan(u Unit, o Options) []Step
    // Runnable says how to build and start a unit for verification.
    Runnable(u Unit) (Runnable, bool)
}
```

If you want to add an ecosystem, please open an issue first so the interface can be agreed
with you rather than around you.

## Things deliberately not built

A plugin system, a recipe registry, Dockerfile generation, a task runner and an AI that runs
commands itself. See [COMPETITOR_ANALYSIS.md](COMPETITOR_ANALYSIS.md) for the reasoning.

## Package dependencies

Packages only depend downwards, so there are no cycles:

```
cli -> guide, plan, repos, verify, analyze, services, docker, ui, project
verify -> analyze, node, services, project
analyze -> node, compose, envfile, services
services -> docker, installer, proc, ui, project
docker -> installer, proc, ui, project
repos -> services, node (and a SetupFunc passed in by cli, so it never imports cli)
project, fsx, proc, ui, buildinfo -> standard library only
```
