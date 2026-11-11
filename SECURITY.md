# Security policy

gclone runs commands on your computer on behalf of repositories you point it at, so
security reports are taken seriously.

## What is in scope

- gclone running commands, installing software or starting services **without** the prompt
  or flag it is supposed to require.
- Secret values (from `.env` files or elsewhere) appearing in terminal output, in
  `SETUP_GUIDE.md`, or in files with permissions that are too open.
- A crafted repository making gclone read or write outside the project directory, or
  overwrite or delete files it should not touch.
- Command injection through repository content (file names, package scripts' names, env
  values, README links).

## What is not in scope

- A repository's own install, build or start scripts doing something harmful. gclone
  warns about install scripts, but it does not sandbox project code. Running `gclone` on a
  repository you do not trust is running that repository's code. Use `--dry-run` and
  `--ignore-scripts` first.
- Problems in a database image, Docker, npm or other tools gclone calls.

## How to report

Please **do not open a public issue** for a vulnerability.

Use GitHub's private reporting: go to the
[Security tab](https://github.com/rauni5/gclone/security/advisories/new) of the repository
and choose *Report a vulnerability*. If that is unavailable, contact the maintainer through
the address on their GitHub profile (<https://github.com/rauni5>).

Include the gclone version (`gclone --version`), your operating system, what you expected,
what happened, and steps to reproduce. A small example repository helps a lot.

## What to expect

This is a small project, so these are best-effort goals rather than guarantees: an
acknowledgement within about a week, a fix or a clear plan for confirmed issues, and credit
in the changelog if you want it. Please give us a reasonable time to fix a problem before
discussing it publicly.

## Supported versions

Only the latest release receives security fixes while the project is on 0.x.
