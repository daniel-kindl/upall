# Architecture Decision Records

An ADR records a decision that was hard to make, together with what it cost and what
was given up. It exists so that whoever meets the decision next — months later, with
no memory of the discussion — can tell the difference between a considered choice and
an accident.

The important part of an ADR is not the decision. It is the **alternatives that were
rejected and why**. Anyone can read the code and see what was chosen. Nobody can read
the code and see what was tried, considered, and abandoned for good reason.

## When you need one

Write an ADR when a decision:

- **would be expensive to reverse** — data formats, public interfaces, the choice of a
  UI toolkit;
- **looks wrong from the outside** — where a reasonable person would ask "why on earth
  did they do it that way";
- **rules something out** — a non-goal, a constraint, a thing deliberately not built;
- **traded something real away** — every good decision here cost something, and the
  cost is the part that gets forgotten first.

You do not need one for reversible, local choices. Which sort algorithm, how a
function is named, whether to extract a helper — that is what code review is for.

If you are unsure, ask whether a future contributor might undo this without knowing
what it was protecting. If yes, write the ADR.

## How they work

- Numbered sequentially, four digits, never reused: `0001`, `0002`, …
- Filename is `NNNN-short-kebab-case-title.md`.
- Copy [`0000-template.md`](0000-template.md) to start.
- **ADRs are immutable once accepted.** You do not edit a decision to reflect a change
  of mind. You write a new ADR that supersedes it, and mark the old one superseded.
  The record of having believed something is itself worth keeping.
- Add a row to the index below in the same pull request.

### Status

| Status | Meaning |
|---|---|
| **Proposed** | Written, under discussion, not yet in effect |
| **Accepted** | In effect. The codebase is expected to comply. |
| **Superseded by NNNN** | Replaced. Kept for the history, not the guidance. |

## Index

| # | Title | Status | Date |
|---|---|---|---|
| [0001](0001-go-as-implementation-language.md) | Go as the implementation language | Accepted | 2026-08-02 |
| [0002](0002-hybrid-provider-model.md) | Hybrid provider model: manifests and native code | Accepted | 2026-08-02 |
| [0003](0003-per-provider-elevation.md) | Per-provider elevation | Accepted | 2026-08-02 |
| [0004](0004-internal-first-api-surface.md) | Internal-first API surface | Accepted | 2026-08-02 |
| [0005](0005-fyne-for-the-gui-client.md) | Fyne for the GUI client | Accepted | 2026-08-02 |
| [0006](0006-branching-model-and-release-channels.md) | Branching model and release channels | Accepted | 2026-08-02 |
| [0007](0007-godoc-as-reference-documentation.md) | Godoc as the reference documentation | Accepted | 2026-08-02 |

## A note on 0001–0007

These seven were all written before a line of code existed, which is unusual and
worth explaining. This project is worked on in bursts with long gaps, by contributors
starting cold each time. Decisions made and forgotten get silently re-litigated,
usually badly and usually by re-deriving only the arguments that are easy to
re-derive.

Front-loading them trades some ceremony now for not having that argument later.
Later ADRs will arrive one at a time, the normal way.
