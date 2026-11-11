# Organic growth plan

The goal is users who succeed with gclone and contributors who come back. Not stars.
Everything here is something you can do honestly: no fake accounts, no bought or traded
stars, no coordinated upvotes, no misleading claims.

## Principles

1. **Be useful before being visible.** A tool that works on ten real repositories is worth
   more than a launch post.
2. **Say what it does not do.** The README's *Experimental* section builds trust.
3. **Make the first contribution easy.** Small, labelled, well-described issues.
4. **Answer people quickly and kindly.** Early users become the community.
5. **Measure things that matter** (see the end), not vanity counts.

## Before announcing anything (the launch checklist)

- [ ] `v0.1.0` is tagged and the release has archives and `checksums.txt`
- [ ] CI is green on Linux, macOS and Windows (or Windows is marked experimental and why)
- [ ] You ran gclone by hand on at least 10 real public repositories and wrote down the
      results, including failures, in `docs/COMPATIBILITY.md`
- [ ] A short terminal recording exists (for example with asciinema or VHS) and is in the README
- [ ] 6 to 8 labelled issues from [GOOD_FIRST_ISSUES.md](GOOD_FIRST_ISSUES.md) are open
- [ ] Issues, and optionally Discussions, are enabled and private vulnerability reporting is on
- [ ] You have time in the first week to reply to everything

## GitHub discoverability

**Repository description** (short, accurate):

> git clone plus everything a project needs to run: env files, databases, dependencies, a
> verify step and a setup guide. Written in Go.

**Topics** (pick the ones that are true): `golang`, `cli`, `developer-tools`, `git`,
`onboarding`, `environment-setup`, `devtools`, `docker`, `dotenv`.

Also: upload a social preview image (a clean screenshot of the demo works), pin a roadmap
issue, keep labels tidy, and make every release's notes human-written (what changed and why
a user should care), not just a commit list.

## 30 days

**Week 1: be ready.**
- Create the repository, push, tag `v0.1.0`, check the release workflow works.
- Run gclone on 10 or more real repositories across different styles (plain Express app,
  full-stack with a database, a frontend with a separate backend, a monorepo). Record what
  broke. Fix the top three problems.
- Write `docs/COMPATIBILITY.md` from those runs.

**Week 2: first users.**
- Record the demo. Update the README.
- Show it to people you know: classmates, teammates, your instructor. Watch someone use it
  without helping. Every place they hesitate is a bug or a docs problem.
- Open the first good-first issues.

**Week 3: one public post.**
- Write one honest technical post, for example *"What breaks when you clone an unfamiliar
  repo, and how I automated checking it"*. Show real failures from your compatibility log.
  Do not write an advertisement.
- Share it where it genuinely fits, after reading each community's self-promotion rules
  (for example a Go community for Go-specific content, a "show and tell" or project thread
  where that is allowed). One post per community. Do not repost.

**Week 4: listen.**
- Reply to every issue and comment within a day or two.
- Fix what people actually hit. Release `v0.1.1` quickly. Thank people by name in the notes.

## 90 days

**Month 2: depth, not breadth.**
- Ship the adapter interface and the Java adapter (your own stack, so you can test it on real
  projects). Write a short post about how adapters work, which also recruits contributors.
- Ship the state file and clearer error explanations.
- Review every pull request within a few days. First-time contributors get extra care.

**Month 3: credibility.**
- Publish the comparison with Rex, Bivvy and mise as a fair, factual page. Offer to correct
  anything the maintainers say is wrong. Consider opening a discussion with those projects
  about interoperating, such as reading `mise.toml`.
- Add Flutter. Show a "set up a Flutter and Spring Boot project in one command" walkthrough.
- Post a "Show HN" or similar only when the tool is solid on the common cases and you can
  respond all day. Lead with a link to the repository and a plain description, not hype.
- Add a "contributors" section and recognize people in release notes.

## Where to share (and how)

| Place | What fits | Rule of thumb |
|---|---|---|
| Go community forums and subreddit | Go implementation details, CLI design | Read the rules, share when there is something to learn, not only to promote |
| Developer subreddits and forums | The problem it solves, with the demo | One post, answer every comment |
| Hacker News (Show HN) | A working tool with a clear description | Be present for the first hours, be honest about limitations |
| dev.to, a personal blog | The technical write-ups | Real examples and real failures |
| Your college and local developer groups | Live demo, feedback | Great for first testers and contributors |

## Contributor acquisition

- Keep setup to three commands (`git clone`, `go build`, `go test`).
- Keep 5 to 10 good-first issues open at any time, each with acceptance criteria.
- Respond to a first pull request within two days, even if only to say "reviewing this week".
- Never require reading the whole codebase for a small change: the structure table in
  `CONTRIBUTING.md` is the entry point.
- Label things clearly, and close stale threads kindly.

## Release quality

Semantic versioning (0.x while the interface may change), a changelog entry for every
user-visible change, cross-platform archives, SHA-256 checksums, and an upgrade note
whenever a flag or behaviour changes.

## What to measure

- Repositories people report working (and not working) on
- Bug reports with a reproduction, and time to first reply
- Pull requests from people other than you
- Whether people use it a second time

Stars and downloads are nice, but they are an outcome, not a goal. If those other numbers
are healthy, the stars follow on their own.

## What not to do

- No buying, trading or asking for stars. Do not run giveaways tied to starring.
- No fake accounts or reviews, no coordinated upvoting.
- No claiming features that are not there, or benchmarks you did not run.
- No posting the same announcement in many places on the same day.
- No arguing with critics. Thank them, check whether they are right, fix it if they are.
