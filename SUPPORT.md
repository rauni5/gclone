# Getting help

## Before you ask

- Read the [README](README.md), especially *Supported today* and *Safety and security*.
- Open `SETUP_GUIDE.md` in the cloned project. It lists what gclone did and what is left
  for you to set.
- Run `gclone verify` for a quick view of what is failing and why.
- Search the [existing issues](https://github.com/rauni5/gclone/issues).

## Where to ask

| You want to | Go to |
|---|---|
| Report something that does not work | [Bug report](https://github.com/rauni5/gclone/issues/new?template=bug_report.yml) |
| Suggest a feature | [Feature request](https://github.com/rauni5/gclone/issues/new?template=feature_request.yml) |
| Fix or improve the docs | [Documentation issue](https://github.com/rauni5/gclone/issues/new?template=documentation.yml) |
| Report a vulnerability | [SECURITY.md](SECURITY.md) (not a public issue) |
| Contribute code | [CONTRIBUTING.md](CONTRIBUTING.md) |

## What to include

- The command you ran and the full output
- `gclone --version` and your operating system
- A link to the repository you were setting up, if it is public
- Whether Docker and Node are installed (`docker info`, `node --version`)

**Remove secrets** (API keys, passwords, connection strings) from anything you paste,
including `SETUP_GUIDE.md`.

## Response times

gclone is maintained by volunteers. Expect replies in days, not hours. A reproducible
report with a public example repository gets fixed fastest.
