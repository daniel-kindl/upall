# 0001. Go as the implementation language

**Status:** Accepted
**Date:** 2026-08-02

## Context

upall must ship as a self-contained binary for Windows and Linux, with macOS to
follow. "No prerequisites" is a headline promise: a downloaded file runs, with no
runtime, interpreter, or shared library to install first. That alone rules out
anything with a managed runtime.

Most of what upall does is unglamorous. Spawn a subprocess, capture its output, parse
a table, collect results, print them. There is very little algorithmic work and almost
no performance-sensitive code, because the wall clock is dominated by `apt` talking to
a mirror rather than by anything upall computes.

The project is worked on in bursts with long gaps between sessions, by contributors
who arrive without context. Time to productive after six months away is a real
constraint here, not a soft preference.

## Decision

Go.

The standard library covers subprocess execution, context cancellation, JSON, HTTP,
and filesystem paths without third-party dependencies. `GOOS`/`GOARCH`
cross-compilation produces static binaries for every target from any host, in one
command and without a toolchain per target.

## Consequences

- Cross-compiling to all four v1.0 targets is a build matrix, not a project.
- Subprocess handling, context cancellation, and structured concurrency are the three
  things this codebase does most, and all three are standard library, well documented,
  and boring.
- Coming back cold is cheap. Go's small surface and explicitness mean six-month-old
  code reads the way it was written.
- Binaries are large: 10 to 20 MB for the CLI, 25 to 40 MB for the GUI. For a tool
  installed once and run occasionally this does not matter, but it will look startling
  next to a shell script.
- Error handling is verbose, and there is no type-system help for making illegal
  states unrepresentable. The domain, where a plan is either empty, partial, or
  complete, would model more cleanly in a language with sum types.
- **cgo is a liability to watch.** Pure-Go dependencies keep cross-compilation
  trivial; anything requiring cgo makes it painful. This constrains later choices, and
  it already has: see [ADR-0005](0005-fyne-for-the-gui-client.md).

## Alternatives considered

### Rust

The strongest alternative, and genuinely better on several axes. `Result` and
exhaustive matching would model the provider outcome taxonomy more precisely than
Go's error interface, and the domain types in `internal/core` would benefit from real
sum types. Binaries are smaller and there is no garbage collector.

Rejected on cross-compilation friction and iteration speed. Producing Windows binaries
from Linux, or vice versa, needs a linker and toolchain setup that Go gets right by
default, and this is a project where release engineering has to stay maintainable by
one person returning to it after months away. Compile times also tax the edit-run loop
that dominates provider parser work.

This rejection is about project circumstances, not language quality. It would deserve
revisiting if the project ever grew a team and a stable release pipeline.

### C# and .NET

Native AOT can produce self-contained binaries, and the Windows integration story is
better than anything else here, particularly the Windows Update COM API, which is a
native provider either way. Rejected because the Linux experience remains second-class
in practice, and the AOT toolchain adds a step that has to keep working across a
project with long dormant periods.

### Zig

Excellent cross-compilation, genuinely small binaries. Rejected as too young for a
project meant to be picked up cold: a smaller ecosystem, fewer contributors who know
it, and a language still changing under its own code.
