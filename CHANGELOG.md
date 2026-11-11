# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/). While the version is 0.x, minor versions may
include breaking changes.

## [Unreleased]

### Added

- Release tooling: `scripts/build-release.sh`, a release workflow that publishes archives
  and SHA-256 checksums, CI and lint workflows.
- Contributor documentation, issue and pull request templates, security and support policies.

## [0.1.0] - first release, not yet tagged

### Added

- `gclone <repo> [directory]` clones a repository and sets up Node.js projects (npm, pnpm,
  yarn, bun) found at the repository root or one folder down.
- `gclone setup [directory]` does the same for a repository that is already cloned.
- Node version check against `.nvmrc`, `.node-version`, `.tool-versions` and `engines`.
- `.env` creation from example files, variable triage (must set, change for real use,
  fine) and detection of variables the code reads but the example omits.
- Detection of MongoDB, PostgreSQL, MySQL and Redis; starting compose database services; an
  ask-first flow to run a missing database in Docker or install it natively.
- Handling of Docker not installed, not running, permission denied and image download
  failures.
- Detection of other repositories a project needs (README links, local `file:` dependencies,
  compose build contexts, API URLs, git submodules), with ask-first cloning and setup.
- `gclone verify`: environment, services, build, start, port and HTTP checks, with exit
  code 1 when not ready. Optional after setup, and recorded in the guide.
- `SETUP_GUIDE.md` generation, excluded from `git status`.
- `--dry-run`, `--yes`, `--ignore-scripts`, `--no-install`, `--no-services`,
  `--install-services`, `--clone-deps`, `--no-deps`, `--verify`, `--no-build`,
  `--timeout`.

### Security

- The guide and terminal output list variable names and notes, not values from env files
  (exceptions: `PORT` as the expected port, and the login of a database container gclone
  creates).
