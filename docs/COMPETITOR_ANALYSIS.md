# Competitor analysis and product direction

Maintainer notes, written 2026-11-10. Facts about other tools come from their own
pages as read on that date (links at the bottom). Where a page did not say something,
this document says "not described", which is not the same as "not supported". Re-check
before quoting any of this in the README.

## 1. What each tool is

| Tool | One line | Needs config in the repo? | Written in / license |
|---|---|---|---|
| **Rex** | Zero-config universal project runner: detects the stack, runs the standard command (`rex test`, `rex run`, `rex deps`). Has `rex clone <url>` (clone, detect, install dependencies). | No. Optional `rex.toml`. | Go, Apache-2.0 (about 7 stars when checked) |
| **Bivvy** | Declarative setup runner. Steps, `depends_on`, state tracking, re-runs only stale steps, retry/fix/skip on failure. | Yes, `.bivvy/config.yml`. | Appears to be Rust (installable with cargo); license not shown on the page read |
| **mise** | Tool versions, env vars and tasks in one CLI. `mise bootstrap` can set up a machine, including cloning repos. | Yes, `mise.toml` (also reads `.tool-versions`, `.nvmrc`). | MIT; language not stated on the page |
| **Dev Containers** | Specification plus reference CLI (`devcontainer up`) that builds a containerised environment from `devcontainer.json`. | Yes, `devcontainer.json`. Docker is implied by the examples. | Spec, not one tool |
| **Nix** | Reproducible, declarative environments. | Yes (flake or shell file). | Not re-checked for this document; general knowledge: powerful, steep learning curve |
| **Ona (formerly Gitpod)** | Hosted development environments built from a repo, supports `devcontainer.json` and automations. | Yes, for good results. | Hosted product. Rename to Ona seen in search results, details not verified |

## 2. Feature matrix

`yes` = described on the page read. `not described` = the page did not say. `gclone` column is what is implemented and tested today (0.1.1).

| Capability | gclone | Rex | Bivvy | mise | Dev Containers | Ona |
|---|---|---|---|---|---|---|
| Clone and set up in one command | yes | yes (`rex clone`) | not described | partly (`bootstrap.repos`) | no (clone first) | yes (hosted) |
| Works on a repo with no setup config | yes | yes | no (needs `.bivvy`) | no (needs `mise.toml`) | no | partly |
| Stack and package manager detection | Node only | many stacks | not described | tool registry (1,000+) | n/a | n/a |
| Runtime version check | Node only, warns | not described | not described | installs versions | image defines it | image defines it |
| Install dependencies | yes (Node) | yes | via steps | no (tools only) | via lifecycle scripts | via automations |
| Create `.env` from example | yes | auto-loads `.env` | not described | loads env | `remoteEnv`/`containerEnv` | yes |
| Triage env vars (must set / change / fine) | yes | not described | not described | not described | no | no |
| Env vars read by code but not documented | yes (Node) | not described | not described | not described | no | no |
| Detect needed databases | yes (4 kinds) | not described | not described | not described | no | no |
| Start databases | yes, asks first (Docker or native) | not described | via a docker-compose step you write | not described | via compose | yes |
| Handle Docker missing, stopped, denied, image failing | yes | not described | not described | n/a | not described | n/a |
| Companion repos (frontend needs backend) | yes (detect, ask, clone, set up) | not described | not described | not described | no | no |
| Generated run guide | yes (`SETUP_GUIDE.md`) | `rex` overview | `bivvy status` | no | no | no |
| State tracking, re-run only what changed | no | not described | yes | `mise lock` for versions | no | prebuilds |
| Verify the app actually starts | no | not described | not described | no | no | not described |
| Retry and recovery on failure | no | not described | yes | no | no | no |
| Dry run | yes | yes (`--dry-run`) | not described | n/a | no | no |
| Doctor / diagnostics | no | yes (`rex doctor`) | `lint`, `status` | yes (`mise doctor`) | no | no |
| CI-friendly output | no | exit codes passed through | yes | n/a | n/a | n/a |
| Dockerfile / compose generation | no | yes | no | no | n/a | n/a |
| Optional AI | no | not described | not described | no | no | AI features exist |

The matrix is only as good as the pages behind it. Rex and Bivvy are small projects
and their pages may lag their code.

## 3. Features that already exist elsewhere (do not rebuild)

- Ecosystem detection and running standard commands: Rex.
- Runtime version installation and project env/tasks: mise (and nvm, fnm, asdf).
- Declarative step graphs with state and stale detection: Bivvy.
- Containerised, shareable environments: Dev Containers, Ona.
- Reproducible-to-the-byte environments: Nix.
- Dockerfile and compose generation, watch mode, badges: Rex.

## 4. Where gclone is genuinely different

These are the gaps the pages read did not show anyone filling. Each is also the thing
a user hits when cloning an unfamiliar repo.

1. **Works on repos nobody prepared.** No config file, no recipe. It reads what is
   there: lockfiles, `.nvmrc`, `.env.example`, compose files, README links, source code.
2. **Env var triage.** Not "copy the file and hope": it says which variables block the
   app, which have working defaults but should change, and which the code reads but the
   example forgot.
3. **Services, asked first.** Detects MongoDB, PostgreSQL, MySQL and Redis from
   dependencies, Prisma and URLs, then offers Docker or a native install, with handling
   for Docker being missing, stopped, denied or unable to pull.
4. **Companion repositories.** A frontend whose backend lives in another repo is the
   common case a single-repo tool cannot handle.
5. **A human-readable guide as the output**, not only side effects.

Honest overlap: **Rex's `rex clone` does the basic version of this tool.** The pitch
cannot be "clone and install". It has to be "get a repo to actually run".

## 5. Highest-value additions, in order

| # | Item | Why | Effort |
|---|---|---|---|
| 1 | **Verify step.** After setup: run the build, start the dev script, wait for the port, check the HTTP response, report pass or fail with the log tail. `gclone verify`. | No tool read here describes it. It is the difference between "installed" and "ready". | Medium |
| 2 | **Adapter interface, then Java (Maven/Gradle, Spring Boot) and Flutter.** | Today only Node works. Two adapters make the interface real. | Medium |
| 3 | **State file** (`.gclone/state.json`): step results, lockfile hashes. Skip unchanged steps, `gclone status`. | Bivvy proves the demand. Makes re-runs fast and safe. | Small |
| 4 | **Rule-based error classification.** Match known failures (Node version mismatch, native module build failure, port in use, auth to registry) to a plain explanation and a suggested fix. | Real value with no AI dependency. | Small to medium |
| 5 | **Interop with existing config.** If the repo has `mise.toml`, `.tool-versions`, `devcontainer.json`, `Justfile` or a Makefile `setup` target, offer to use it instead of guessing. | Complements the other tools instead of competing. | Small each |
| 6 | **Port conflict detection and a `gclone doctor`.** | Common first-run failure. | Small |
| 7 | **CI-friendly output** (`--ci`, no prompts, stable exit codes). | Needed for adoption in scripts. | Small |
| 8 | **Optional AI planner** behind an adapter. Reads the README, proposes steps, every command validated and approved by the user. Never required. | A real differentiator only after 1 to 4 exist. | Medium |

## 6. What should not be built

- **A task runner or version manager.** mise and Rex exist. Call them.
- **Dockerfile or compose generation.** Rex does it; it is not the problem this tool solves.
- **A plugin system before two adapters exist.** An interface, yes. Dynamic plugins, no.
- **A recipe registry or website.** Needs users first. A static docs page is enough.
- **An AI that runs commands on its own.** Suggestions only, validated and approved.
- **Watch mode, badges, stats.** Not onboarding.
- **Rollback.** Hard to do honestly for installs. State plus a clear "what changed" list is enough.
- **MCP and agent integrations** until the CLI is solid. A JSON output mode (`--json`) is the cheap first step.

## 7. Roadmap

**Short term (next releases):** adapter interface; verify step; state file and
`gclone status`; error classification; `--ci` and `--json`; real integration tests
(fake repos, recorded Docker behaviour); CI, releases, checksums.

**Medium term:** Java (Maven/Gradle, Spring Boot) and Flutter adapters; Python adapter;
interop with `mise.toml`, devcontainer and Makefile targets; `gclone doctor`; port
conflict detection; `gclone export devcontainer`; Windows testing in CI.

**Long term:** optional LLM planner; remote analysis before cloning (read the repo
through the API and show the plan first); multi-repo workspaces; shared recipes if users
ask for them.

## 8. Sources

- Rex: https://github.com/rexrun-dev/rex
- Bivvy: https://bivvy.dev/docs/ and https://github.com/bivvy-dev/bivvy
- mise: https://github.com/jdx/mise and https://mise.jdx.dev/
- Dev Containers: https://containers.dev/implementors/reference/
- Ona (Gitpod): https://ona.com/docs/ona/editors/overview
- Nix: https://nixos.org/ (not fetched for this document)
