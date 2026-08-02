# 0006 — Branching model and release channels

**Status:** Accepted
**Date:** 2026-08-02

## Context

upall is a tool people install and then trust to run privileged operations on their
machine, unattended in some cases. A bad release is not a rolled-back deploy; it is
somebody's laptop.

That argues for a branch whose contents are only ever released, tested code. It
argues equally for somewhere that integration happens continuously, because a
long-lived unreleasable branch is how projects accumulate a six-month merge.

Two audiences exist and want different things. Most users want the version that
works. A few — including the maintainer, on their own machine — want what is being
built now and are willing to hit bugs to get it.

The project is worked on in bursts. A model requiring ceremony at the wrong moment
will be skipped, so it has to be cheap to follow on a Tuesday evening after two months
away.

## Decision

Three kinds of branch:

```
feature/* ──PR──▶ dev ──release PR──▶ main
                   │                    │
                   │                    └─ tag vX.Y.Z      → stable channel
                   └─ tag vX.Y.Z-dev.N → dev channel
```

- **`main`** holds stable, tested, released code and nothing else. Tags on `main` are
  releases: `v1.2.0`.
- **`dev`** is the integration branch and is itself a released channel. Merges tag a
  prerelease: `v1.3.0-dev.4`.
- **`feature/*`** is where work happens, and may open a pull request **only against
  `dev`**.

Both `main` and `dev` are protected: no direct pushes, pull request required, CI
green on both operating systems.

**`main` receives merges only from `dev`**, except for `hotfix/*`, which may PR
directly to `main` and **must** then be back-merged to `dev`. This exception exists
because urgent fixes are real, and a model with no path for them gets bypassed
entirely the first time one is needed.

A pull request from `feature/*` targeting `main` is closed rather than retargeted.
Retargeting quietly produces a `main` containing untested work, which is the one
thing this model exists to prevent.

Feature branches **squash-merge** into `dev`, so one pull request becomes one
conventional commit and the history stays readable as a changelog.

**Every merge to `dev` must leave `dev` usable.** It is a channel people install, not
a staging area.

### Versioning

Both channels are semver, and prereleases use semver's own prerelease syntax so
ordering is defined by the spec rather than by convention:

```
1.3.0-dev.1  <  1.3.0-dev.2  <  1.3.0-dev.3  <  1.3.0
```

Version bumps derive from conventional commit types; the mapping is in
[AGENTS.md](../AGENTS.md).

## Consequences

- `main` is always releasable and always released. "Is this version safe" has an
  unambiguous answer.
- Dev users get continuous builds without a separate versioning scheme, and any
  version string can be compared against any other by a standard semver parser — which
  is what a future self-updater will need to offer a channel switch.
- Squash merges give a commit history that generates release notes directly.
- **Overhead on every change.** A one-line typo fix in a doc still needs a branch and
  a pull request, and for a solo maintainer that is friction with no reviewer at the
  end of it. Accepted deliberately: the discipline is worth more than the minutes, and
  it means the project is already shaped correctly if a second contributor arrives.
- **Two release pipelines** to build and maintain, one per channel.
- **`dev` must stay usable**, which is a real constraint on how work is split. A
  change that cannot land in a working state has to be sequenced behind a flag or
  broken up, rather than merged half-finished.
- The hotfix path is a documented hole in the model. It is narrow, but it exists, and
  a forgotten back-merge silently reverts the fix on the next release. Back-merging is
  part of the hotfix, not a follow-up.

## Alternatives considered

### Trunk-based: everything to `main`

One branch, short-lived features, releases cut by tagging whatever is on `main`.
Substantially less ceremony, no back-merge hazard, no second pipeline, and it is what
most small projects do successfully.

Rejected because `main` then contains untested work between releases, and the
question "is the current `main` safe to install" has no answer. For a tool that runs
elevated on other people's machines, having a branch that is *by definition* only
released code is worth the overhead. It also means anyone who installs from source
gets a tested version by default rather than by luck.

### Full git-flow, with release and support branches

`develop`, `release/*`, `hotfix/*`, `support/*` and the full ceremony. Handles
parallel maintenance of multiple released versions, which this model does not.

Rejected as far too much process for a project with one maintainer and no
supported-version obligations. The model chosen here is git-flow with the parts
removed that only pay off with a release manager and simultaneously supported
versions. If upall ever needs to patch a 1.x while 2.x is current, that is a new ADR.

### Date-stamped nightlies on `dev`

`main` tags semver; `dev` publishes `nightly-2026-08-02` builds outside the version
scheme. A clean separation, and it avoids implying that a dev build is "on the way to"
any particular version.

Rejected because the two streams cannot then be ordered by a version comparison. A
self-updater, or a user asking "is my nightly newer than the stable release", needs
channel-specific logic to answer something semver answers for free. The implied
next-version number is a minor cost; being outside the version system is not.
