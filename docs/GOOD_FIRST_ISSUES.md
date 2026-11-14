# Good first issues

Issue-ready descriptions for small, self-contained tasks. Each one says where to look and
what "done" means. To use one, copy it into a new issue (the maintainer can also do this) and
add the labels `good first issue` and `help wanted`.

Suggested labels for the repository: `bug`, `enhancement`, `documentation`,
`good first issue`, `help wanted`, `needs-triage`, `needs-repro`, `windows`, `macos`,
`docker`, `detection`, `verify`.

Before you start: read [CONTRIBUTING.md](../CONTRIBUTING.md) and comment on the issue so
nobody duplicates the work.

---

## 1. Detect more JavaScript frameworks

**Labels:** `good first issue`, `detection`

**Why.** gclone shows the framework and the expected port in the guide, and `verify` uses the
port as a fallback. Frameworks that are not in the list show neither.

**Where.** The `frameworks` list in `internal/adapter/node/node.go`.

**What to do.** Add a few frameworks (for example Gatsby, Docusaurus, VitePress, Eleventy,
Parcel). Check each tool's documentation for the correct dependency name and default
development port before adding it. Mind the order: more specific frameworks must come before
generic ones such as `vite` or `react`.

**Acceptance criteria**
- Each new entry has the dependency name, a display name and the default port.
- A table test builds a `package.json` for each and checks the detected framework and port.
- Existing tests still pass.

---

## 2. Add a service: RabbitMQ

**Labels:** `good first issue`, `detection`, `docker`

**Why.** gclone detects four services. Projects that use a message queue currently get no
help starting one.

**Where.** `internal/services/services.go`: `svcDefs`, `kindOrder`, `depKind`, `urlSchemeRe`, `schemeKind`,
`imageKindRe`/`imageKind`. Also the `dbImage` expression in `internal/compose/compose.go`.

**What to do.** Add RabbitMQ: dependency `amqplib`, URL scheme `amqp://` and `amqps://`, port
5672, image `rabbitmq:3`, data directory `/var/lib/rabbitmq`.

**Acceptance criteria**
- A project depending on `amqplib` or with `AMQP_URL=amqp://localhost:5672` shows RabbitMQ in
  "Services the project needs".
- A compose file with a `rabbitmq` image counts as providing it.
- `amqps://` and non-local hosts are treated as remote.
- Tests cover detection and `dockerRunArgs`.
- The README's service list is updated.

---

## 3. Explain why `npm ci` fell back to `npm install`

**Labels:** `good first issue`

**Why.** When a lockfile is out of sync, `npm ci` fails and gclone retries with
`npm install`. The current message only says that the lockfile may be out of sync.

**Where.** The fallback in `BuildPlan` and `ExecutePlan` in `internal/plan/plan.go`.

**What to do.** Make the message say what happened and what the user may want to do (commit
the updated lockfile, or report it upstream). Record it in the guide's "What gclone did" table
as it is now.

**Acceptance criteria**
- The message is one or two plain sentences with no stack trace.
- The guide row for the step mentions the fallback.
- A test covers the message construction (extract it into a small function).

---

## 4. A test helper for a fake `docker`

**Labels:** `good first issue`, `docker`, `testing`

**Why.** The Docker flows were checked by hand with a stand-in command. They should be
repeatable by anyone.

**Where.** A new test helper in a `_test.go` file.

**What to do.** Write a helper that creates a temporary directory containing an executable
`docker` script, prepends it to `PATH` for the test, and records the arguments it receives.
Support modes for: engine ready, engine stopped, permission denied, image download fails.
Use it to test `docker.State`, `docker.EnsureImage` and the `OfferInstall` decisions.

**Acceptance criteria**
- Tests for all four modes, on Linux and macOS (skip on Windows with a clear reason).
- No test needs a real Docker engine.
- The helper is documented in a comment and in `CONTRIBUTING.md`.

---

## 5. Make the tests pass on Windows

**Labels:** `windows`, `help wanted`

**Why.** CI runs the tests on Windows, but nobody has verified gclone there by hand.

**Where.** `internal/proc/proc_windows.go`, the `TestVerify*` tests in `internal/verify`, path
handling in `internal/adapter/node`.

**What to do.** Run `go test ./...` on a Windows machine. Fix failures. In particular check
that `taskkill` stops the whole process tree, that `npm` resolves to `npm.cmd`, and that
paths in the guide use forward slashes where they are shown as commands.

**Acceptance criteria**
- `go test ./...` passes on Windows in CI.
- A short note in the PR on what you ran by hand (`gclone --dry-run`, `gclone verify`).
- Anything that cannot work on Windows is skipped in tests with a reason, and listed in the
  README's *Experimental* section.

---

## 6. Offer `corepack enable` when pnpm or yarn is missing

**Labels:** `detection`

**Why.** A project with `"packageManager": "pnpm@9..."` fails to install when pnpm is not on
the machine. The error only says the program was not found.

**Where.** The install step in `BuildPlan` in `internal/plan/plan.go`.

**What to do.** When the package manager is missing and Node's `corepack` is available, ask
whether to run `corepack enable`, then retry. Respect the existing rule: ask first, default
to no.

**Acceptance criteria**
- With a missing manager and `corepack` present, the user is asked and the answer is honoured.
- With neither, the message says how to install the manager.
- The step is shown in the plan and recorded in the guide.

---

## 7. `gclone verify --json`

**Labels:** `verify`, `enhancement`

**Why.** CI jobs and editors want to read the result without parsing text.

**Where.** `Report`, `UnitReport` and `Check` in `internal/project/report.go`, flag parsing in
`internal/cli/cli.go`.

**What to do.** Add JSON tags and a `--json` flag that prints the report as one JSON document
to standard output (progress text goes to standard error). The exit code stays the same.

**Acceptance criteria**
- Stable, documented field names.
- Output is valid JSON for ready, warning and failing runs.
- A test parses the output.
- The flag is added to the README flag table.

---

## 8. A troubleshooting page

**Labels:** `documentation`

**Why.** People hit the same few problems: Docker permission denied, Node version mismatch,
port already in use, registry login.

**Where.** New file `docs/TROUBLESHOOTING.md`, linked from the README and `SUPPORT.md`.

**What to do.** For each message gclone can print, explain what it means and the fix. Use the
real wording from the code (search for the strings under `internal/`).

**Acceptance criteria**
- At least eight entries, each with the exact message, the cause and the fix.
- Every message quoted exists in the code (check with a search).
- Linked from the README.

---

## 9. Warn when two packages in one repository expect the same port

**Labels:** `detection`

**Why.** A repository with `client/` and `server/` that both default to port 3000 will fail
the second time something starts. gclone already warns about clashes with another repository,
but not inside one.

**Where.** `AnalyzeProject` in `internal/analyze` and `PortFor` in `internal/project/pkg.go`.

**What to do.** After analyzing all packages, add a warning when two of them expect the same
port.

**Acceptance criteria**
- The warning appears in the plan output and in the guide.
- Packages with unknown ports are ignored.
- A test covers a clashing and a non-clashing layout.

---

## 10. Add a failure hint

**Labels:** `good first issue`, `detection`

**Why.** When a setup step fails, gclone looks for a known pattern in the output and prints
what it probably means and what to try. Each real failure someone reports can become a new
rule, so the list grows with use.

**Where.** `internal/explain/explain.go` (the `rules` list) and `internal/explain/explain_test.go`.

**What to do.** Pick an error you have seen (for example a Python-style `ModuleNotFoundError`
is out of scope today, but Gradle, Maven, npm, Docker and git errors are all fine). Copy the
real output into a test case, add a rule with a short plain-language meaning and a concrete
next step. Keep the wording honest: "probably", not "this is".

**Acceptance criteria**
- A test with the real error text that expects the new rule id.
- The rule does not match unrelated output (add a negative case if the pattern is broad).
- The advice never tells people to disable security checks or use `sudo`.
