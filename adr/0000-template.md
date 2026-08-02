# NNNN — Short title in the imperative or as a noun phrase

**Status:** Proposed | Accepted | Superseded by [NNNN](NNNN-title.md)
**Date:** YYYY-MM-DD

## Context

What is true that makes this a decision rather than an obvious call? The forces in
play: constraints, requirements, things already committed to elsewhere. Someone with
no memory of the discussion should be able to read this section and feel the tension
before seeing how it was resolved.

Write it in the present tense, as a description of the situation — not a narrative of
who said what.

## Decision

What was chosen, stated plainly and in one or two sentences. Then whatever detail is
needed to act on it.

Use active voice: "Providers declare their own elevation requirement", not "it was
decided that elevation requirements would be declared".

## Consequences

What is now true as a result — the good and the bad in the same list, without
flattering the decision. Every real decision costs something; if this section contains
only benefits, the decision was not hard and probably did not need an ADR.

Include here:

- What becomes easy.
- What becomes hard or impossible.
- What ongoing obligation this creates.
- **The revisit trigger, if the decision is deliberately provisional** — the specific
  condition that should cause someone to reopen this. Write a condition you would
  recognize on sight, not "when it becomes a problem".

## Alternatives considered

The most valuable section. For each option seriously weighed:

### Option name

What it was, and **why it was rejected**. Be concrete and fair — state the real
argument for it, not a weak version that makes the chosen option look better. If an
alternative was rejected for a reason that might not hold forever, say so here; that
is the sentence a future reader needs most.

---

<!--
Worked example, to fix the tone and level of detail. This is a hypothetical decision,
not a real one — delete this whole comment block when you copy the template.

## Context

Every run appends a record to the journal, and `upall history` reads it back. Runs
are append-only and never modified after the fact. The journal must survive being
written to while a previous run is being read, and must not grow without bound. upall
ships as a single binary with no prerequisites, so anything requiring a server is out.

## Decision

The journal is a JSON Lines file: one run per line, appended, never rewritten in
place. Rotation happens by size, oldest file discarded.

## Consequences

- Appending is a single write with no read-modify-write, so a crash mid-run costs at
  most the current line and never corrupts earlier ones.
- The format is greppable and readable without upall, which matters when someone is
  debugging a bad run at 2am.
- Querying means scanning. `upall history` stays fast because it reads from the end
  and stops, but anything analytical over the full history would be slow.
- Rotation loses old runs outright. There is no archive.
- Revisit if history grows a query surface beyond "show me recent runs" — at that
  point a real embedded database earns its weight.

## Alternatives considered

### SQLite

Proper queries, indexes, transactional integrity, and rotation becomes a DELETE. The
real argument for it is that "show me every time provider X failed" is a natural
question a JSON Lines file answers badly. Rejected because it pulls in cgo, which
complicates the cross-compilation story that ADR-0001 is built on, and because the
journal has exactly one reader asking exactly one question today.

### A single JSON array file

Simpler to parse in one shot. Rejected because appending requires rewriting the whole
file, which makes a crash mid-write lose the entire history rather than one line.
-->
