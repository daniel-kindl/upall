# Contributing rules

Mechanics of changing this repository. Applies to humans and to agents equally.

Architecture rules are in [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md). What to build
next is in [docs/ROADMAP.md](docs/ROADMAP.md). Getting set up is in
[CONTRIBUTING.md](CONTRIBUTING.md).

---

## Conventional commits

Every commit message follows [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>

[body]

[footer]
```

### Types

| Type | Meaning | Version bump |
|---|---|---|
| `feat` | New capability | minor |
| `fix` | Bug fix | patch |
| `perf` | Performance improvement | patch |
| `refactor` | Restructuring, no behavior change | none |
| `docs` | Documentation only | none |
| `test` | Tests only | none |
| `build` | Build system, dependencies | none |
| `ci` | CI configuration | none |
| `chore` | Everything else | none |

A `!` before the colon, or a `BREAKING CHANGE:` footer, means a **major** bump — and
below 1.0, a minor bump instead. See [Semver](#semver) for what counts as breaking.

### Scope

The scope is the package the change lives in, without the `internal/` prefix:
`provider`, `pipeline`, `exec`, `cli`, `gui`, `journal`, `elevate`, `paths`, `core`.
Use `release` for release engineering and omit the scope for repo-wide changes.

For a specific provider, use the provider ID: `feat(winget):`, `fix(apt):`.

Two scopes are reserved for automated dependency pull requests, and match what
`.github/dependabot.yml` is configured to emit: `build(deps)` for Go modules and
`ci(actions)` for GitHub Actions.

### Description

Imperative mood, lowercase, no trailing period. Describe what the change does, not
what you did.

```
feat(provider): add flatpak manifest
fix(exec): kill the process group on context cancellation
docs(pipeline): explain why apply is never concurrent within a provider
refactor(core): extract exit-code derivation from Result
feat(cli)!: rename --skip to --except

BREAKING CHANGE: --skip is removed. Use --except, which takes the same values.
```

Not this:

```
updated stuff
fix bug
WIP
feat: added a new provider for flatpak and also fixed the apt parser
```

The last one is two commits.

---

## Semver

Versions are [semver 2.0.0](https://semver.org/). Release channels and how tags map
to branches are in [ADR-0006](adr/0006-branching-model-and-release-channels.md).

### What is public

These are the interfaces under semver. Changing one incompatibly is a breaking change:

- **The CLI surface** — command names, flag names, and flag semantics.
- **Exit codes** — the contract in ARCHITECTURE.md.
- **The config schema** — TOML keys and their meanings.
- **The JSON output schema** — `--json` on `plan` and `apply`.
- **The provider manifest schema** — fields and parser names.

### What is not

**The Go API is not public.** Everything is `internal/`, so nothing outside this
module can import it, and no rename inside it is a breaking change. This is
deliberate — see [ADR-0004](adr/0004-internal-first-api-surface.md).

Do not add the Go API to the list above without an ADR superseding 0004.

### Before 1.0

Minor versions may break the interfaces listed above. The changelog must say so
explicitly, and the commit still needs its `!` or `BREAKING CHANGE:` footer — the
marker is how release notes are generated, and it is not optional just because the
bump is smaller.

---

## Branching

```
feature/* ──PR──▶ dev ──release PR──▶ main
                   │                    │
                   │                    └─ tag vX.Y.Z      → stable channel
                   └─ tag vX.Y.Z-dev.N → dev channel
```

- **`main`** — stable, tested, released code. Nothing else, ever.
- **`dev`** — integration branch, and itself a released channel.
- **`feature/*`** — where work happens.

### Rules

1. **No direct pushes to `main` or `dev`.** Both are protected; pull requests only.
2. **Feature branches PR into `dev`. Never into `main`.** A pull request from
   `feature/*` targeting `main` is closed, not retargeted — retargeting is how
   untested work reaches `main`.
3. **`main` receives merges only from `dev`**, via a release PR.
4. **Squash-merge into `dev`.** One pull request becomes one conventional commit, so
   the history reads as a changelog.
5. **Every merge to `dev` must leave `dev` usable.** People install it. If a change
   cannot land in a working state, sequence it behind a flag or split it up.
6. **CI green on `windows-latest` and `ubuntu-latest`** before any merge. A change
   that compiles on only one platform is not mergeable.

### Hotfixes

`hotfix/*` may PR directly to `main`. **The fix must then be back-merged to `dev` as
part of the same piece of work**, not as a follow-up — a forgotten back-merge silently
reverts the fix at the next release.

---

## The godoc rule

Reference documentation is godoc. Not markdown. See
[ADR-0007](adr/0007-godoc-as-reference-documentation.md).

- Every package has a `doc.go` with a package comment explaining what it does and how
  it works.
- Every exported identifier has a doc comment starting with its own name.
- **If you are about to describe how one package works in a markdown file, write it in
  that package's `doc.go` instead.**

`docs/` holds only what belongs to no single package: architecture, roadmap, ADRs.

`revive`'s `exported` rule fails the build on a missing comment. It cannot tell
whether the comment says anything useful — `// Apply applies.` passes and is worthless.
Write the comment for someone who has never seen the package.

---

## Pull requests

**One milestone criterion or less per PR.** If the PR description needs the word
"and", it is probably two PRs.

- **Name the criterion.** Every PR states which ROADMAP acceptance criterion it
  advances, or why it advances none.
- **Refactors ship separately from behavior changes.** A diff that moves code and
  changes what it does cannot be reviewed for either.
- **A new provider is its own PR** — manifest or native, plus its fixture tests.
- **Tests live with the change.** Not a follow-up PR.
- **No test invokes a real package manager.** Fake `internal/exec`. A test suite that
  mutates the machine running it is not a test suite.
- **Leave CI green.** Both platforms.

### Tick the boxes

When a PR completes a ROADMAP acceptance criterion, tick the box in the same PR. The
roadmap is only useful if it is accurate, and a cold session trusts it completely —
an unticked box for finished work sends the next session to redo it.
