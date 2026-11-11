# Contributing to gclone

Thanks for wanting to help. Small contributions are welcome: a typo, a clearer error
message, a test for something that already works. You do not need to understand the whole
codebase to make a useful change.

## Quick start

You need Go 1.24 or newer and git. Node.js 20+ and npm are needed for the tests that start
a real server; those tests skip themselves when Node is missing.

```bash
git clone https://github.com/rauni5/gclone.git
cd gclone
go build -o gclone .
go test ./...
```

Once you have a built `gclone`, you can let it do the same for itself:
`./gclone rauni5/gclone my-copy` clones, downloads modules, and writes a `SETUP_GUIDE.md`;
`./gclone verify my-copy` builds it. This is how we check the Go support.

Try it on a throwaway repository:

```bash
./gclone --dry-run someone/app
```

## Project structure

```
main.go                  entry point: sets the version, calls internal/cli
internal/
  cli/                   arguments, and the flow for one repository (setupRepo)
  project/               shared data model: Project, Pkg, Need, Companion, Report, Options
  analyze/               read a repository and build a Project (no side effects)
  adapter/node/          Node.js: packages, package manager, Node version, runnables
  adapter/golang/        Go: go.mod, Go version check, build command
  compose/               docker compose file parsing
  envfile/               env example files: parse, classify, find variables used in code
  services/              which databases a project needs, start or install them, ask first
  docker/                Docker states (missing, stopped, denied), install/start, images
  installer/             one install/start option, run after the user agrees
  plan/                  build, show and execute the setup steps
  repos/                 other repositories a project needs (README, local deps, submodules)
  verify/                build, start, port check, HTTP probe, report
  guide/                 write SETUP_GUIDE.md
  proc/                  run programs, process groups (unix and windows files)
  fsx/, ui/, buildinfo/  small helpers: files, prompts, version
testdata/                small sample repositories used by tests
docs/                    architecture, good first issues, competitor analysis, growth plan
Makefile                 make build | test | check | release
```

[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) explains how the pieces fit together.

## Ground rules

These keep gclone safe to run on code you did not write.

1. **Ask before changing the machine.** Anything that installs software, starts a service,
   clones another repository or runs project code needs a prompt or an explicit flag. The
   default answer for risky actions is *no*.
2. **Never print or store secret values.** Variable names are fine; values from `.env` files
   are not. Check your log lines and the guide.
3. **Never overwrite or delete the user's files.** An existing `.env` stays as it is.
4. **No shell strings built from repository content.** Use `exec.Command(name, args...)`.
5. **Prefer the standard library.** gclone has no dependencies today. Open an issue before
   adding one.
6. **Say what you did not test.** If you cannot run a code path (Windows, a real Docker
   engine, winget), say so in the pull request.

## Coding conventions

- `gofmt` and `go vet` must be clean (CI checks both).
- Errors start with a lowercase letter and say what failed and what the user can do.
- User-facing text is plain and short. Terminal status markers are `[ok]`, `[..]`, `[!!]`,
  `[warn]`, `[FAIL]` and `[skip]`.
- Keep detection (reading files) separate from execution (running commands) where you can.
  Detection should be testable without touching the machine.
- Write a test with your change. Table tests are welcome.

## Running the tests

```bash
go test ./...            # everything
go test -run Verify ./...  # one group
make check            # gofmt, vet and tests, same as CI
```

Tests that need Node (`TestVerify*`) start real short-lived servers on free ports.

**Faking Docker.** There is no Docker engine in most test environments. To test a Docker
code path, put a small `docker` script first on `PATH` that records its arguments and
returns what you want (for example `exit 1` for `info` to simulate a stopped engine). Real
Docker behaviour should still be checked by hand before you claim it works.

## Pull request workflow

1. Open an issue first for anything bigger than a small fix, so we can agree on the approach.
2. Fork, create a branch, make the change with tests.
3. Run `gofmt -l .`, `go vet ./...` and `go test ./...`.
4. Update `CHANGELOG.md` under **Unreleased** and any docs the change affects.
5. Open a pull request and fill in the template. Keep it focused: one change per PR.
6. A maintainer reviews it. Please expect some back and forth; it is about the code, not you.

Commit messages: a short imperative first line (`Add RabbitMQ detection`), then details if
needed.

## Adding a feature

1. Find where it belongs using the structure above.
2. Add detection first and test it with plain files in a temporary directory.
3. Add the action second, behind a prompt or flag if it changes the machine.
4. Show it in the plan output and in `SETUP_GUIDE.md` so the user can see what happened.
5. Document the flag or behaviour in the README.

## Adding support for a new language or framework

Today the Node.js logic lives in `internal/adapter/node`.
A proper adapter interface is planned together with the Java adapter (see
[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)), so **please open an issue before writing a
new ecosystem**, and we will agree on the shape together. In the meantime, you can help
with:

- more frameworks in the `frameworks` list in `internal/adapter/node/node.go`
- another service in `svcDefs`, `depKind` and `imageKind` in `internal/services/services.go`
- another env example file name in `envFileNames` in `internal/envfile/envfile.go`

Each of these is a small, self-contained change. See
[docs/GOOD_FIRST_ISSUES.md](docs/GOOD_FIRST_ISSUES.md).

## Reporting bugs

Use the bug report form. The most useful reports include the command you ran, the full
output, your OS, `gclone --version`, and a link to the repository if it is public. **Remove
secrets** from anything you paste, including `SETUP_GUIDE.md`.

For security problems, do not open a public issue. See [SECURITY.md](SECURITY.md).

## Code of Conduct

By taking part you agree to follow the [Code of Conduct](CODE_OF_CONDUCT.md).
