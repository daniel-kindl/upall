# 0007. Godoc as the reference documentation

**Status:** Accepted
**Date:** 2026-08-02

## Context

Documentation that lives beside the code it describes goes stale more slowly than
documentation that lives elsewhere, for a mechanical reason: the person changing the
code is looking at it. A markdown file two directories away is not in their diff, not
in their editor, and not in their head.

The failure mode is specific and worth naming. When the same fact is written in two
places, they eventually disagree. Nothing breaks when they do, since no test fails and
no build goes red, so nobody notices, and the next reader has two answers and no way to
tell which is current. Stale documentation is worse than none. It is confidently wrong,
and it was trusted.

This project is worked on in bursts with long gaps. Whatever requires remembering to
update will not be updated.

There is a second, quieter problem. A markdown file is a place where prose can be
written about code without anyone checking it against the code. Comments have the same
hazard, but at least they are in the diff.

## Decision

Godoc is the reference documentation for this project.

Every package has a `doc.go` with a package comment explaining what the package does
and how it works. Every exported identifier has a doc comment beginning with its own
name. Reference documentation is never hand-written into `docs/`.

Markdown is reserved for what godoc has no place to put, meaning the cross-cutting
design that belongs to no single package:

| Lives in markdown | Lives in godoc |
|---|---|
| `docs/ARCHITECTURE.md`, what spans packages | How any one package works |
| `docs/ROADMAP.md`, what to build next | What the code currently does |
| `adr/`, why decisions were made | How to use a type or function |
| `README.md` and `CONTRIBUTING.md`, the project | Anything with a symbol name in it |

The operative rule, stated so it can be applied without judgment:

> If you are about to describe how one package works in a markdown file, write it in
> that package's `doc.go` instead.

`docs/ARCHITECTURE.md` states this at the top, and its layout table indexes the
`doc.go` files rather than summarizing them.

Enforcement is `revive`'s `exported` rule in `.golangci.yml`, wired into CI from M1. A
missing doc comment on an exported identifier fails the build.

## Consequences

- The documentation is in the diff. A reviewer looking at a changed function is looking
  at its comment, and a stale one is visible rather than remote.
- `go doc ./internal/pipeline` answers questions without leaving the terminal, and a
  local `godoc -http` renders the whole thing.
- No fact is written twice, so no two facts can disagree.
- The enforcement is mechanical. It does not depend on a reviewer caring on the day.
- **`revive`'s `exported` rule is easy to satisfy badly.** It checks that a comment
  exists, not that it says anything, so `// Apply applies.` passes. The rule catches
  omissions. Only review catches uselessness, and review will be one person much of the
  time.
- **Package-level prose is harder to write in `doc.go`** than in markdown. There are no
  headings, no tables, no links to other files, and no diagrams. Some explanations are
  genuinely worse for it, and gofmt's comment formatting is opinionated about the rest.
- The documentation is not browsable on the web, because everything is `internal/`. See
  [ADR-0004](0004-internal-first-api-surface.md), which owns that trade and its revisit
  trigger.
- Contributors arriving from projects with a documentation site will look for one and
  not find it. `CONTRIBUTING.md` points at `go doc` explicitly for this reason.

## Alternatives considered

### A documentation site

A generated site, on GitHub Pages or eventually `docs.upall.com`, with guides,
reference, and examples in one browsable place. Better for discovery, better for
newcomers, and the thing an established project is expected to have.

Rejected for now, and the reasoning is the same one that drives this ADR. A docs site
is structurally a second home for facts, and the one that rots fastest, because nothing
anywhere breaks when it goes stale. For a tool whose entire surface is three commands,
`README.md` plus genuinely well-written `--help` output covers what users need, and
`--help` is where CLI users look first anyway. It is an M13 deliverable with its own
acceptance criteria for that reason.

Revisit trigger: the manifest schema is stable and someone outside the project has
authored a provider. A manifest format with a schema, a parser catalogue, and worked
examples is a real authoring surface, and it is the only part of upall that plausibly
outgrows a README. Until then a site would be effort spent on documentation nobody has
asked for, describing software that keeps changing. GitHub Pages from `docs/` is the
cheap first step when it comes, and a custom domain can follow, costing nothing to
defer.

### Markdown reference docs alongside godoc

Keep `docs/` as the primary reference, with godoc comments as a secondary convenience.
Richer formatting, easier to read end to end, and better for onboarding, since a new
contributor can read one document rather than assembling a picture from twelve `doc.go`
files.

Rejected because it is exactly the two-sources-of-truth arrangement described in the
Context. The onboarding argument is real, and this decision does cost something there.
`docs/ARCHITECTURE.md` is meant to carry that weight by explaining how the pieces fit
together, and deliberately stopping short of explaining each piece.
