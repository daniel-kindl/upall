# 0008. Cobra for the command-line interface

**Status:** Accepted
**Date:** 2026-08-03

## Context

The CLI is upall's primary interface. It is the one that works on headless servers and
in CI, and the one the GUI is measured against. By v1.0 it carries a tree of
subcommands, `plan`, `apply`, `version`, `providers`, and `history`, with flags that
cross them: `--only`, `--except`, `--yes`, `--json`, `--config`, `--exit-code`.

That surface is public. [AGENTS.md](../AGENTS.md#semver) puts command names, flag names,
and flag semantics under semver, so how arguments are parsed is a compatibility promise
rather than an implementation detail.

Two M13 criteria set the bar higher than "parse some flags". Shell completions ship for
bash, zsh, fish, and PowerShell. And `--help` is a real deliverable, where every
description reads as a sentence, the root shows a worked example, and each subcommand
states the exit codes it can return.

Pulling the other way, [ADR-0001](0001-go-as-implementation-language.md) chose Go partly
for a standard library that covers this project's work without third-party code, and
the [security model](../docs/ARCHITECTURE.md#security-model) treats every dependency as
surface area in a program that runs elevated.

## Decision

[spf13/cobra](https://github.com/spf13/cobra) builds the command tree, in
`internal/cli`. `cmd/upall` calls into it and does nothing else.

Cobra brings `spf13/pflag` with it, so flags are GNU-style: `--yes`, `-y`, and
`--only=apt,snap`. That is now the parsing contract.

## Consequences

- **The four completion scripts are generated, not written.** Cobra emits bash, zsh,
  fish, and PowerShell completions from the command tree itself, which turns an M13
  criterion into roughly ten lines and, more importantly, means completions cannot drift
  away from the commands they describe.
- Subcommand dispatch, flag inheritance, `--help` on every command, and "did you mean"
  suggestions are all given rather than built.
- Three dependencies arrive: `cobra`, `pflag`, and `mousetrap` on Windows. All are pure
  Go with no cgo, so ADR-0001's cross-compilation story is untouched, and Dependabot
  already watches `gomod`.
- **Cobra's default help layout is Cobra's, not ours.** M13's help criteria mean
  overriding usage templates. Cobra makes that possible; it does not make it free, and
  the work is real.
- Cobra carries machinery upall will never use, including its own docs generation and
  the `__complete` protocol internals. That is dead weight in a binary ADR-0001 already
  concedes is large.
- GNU-style flags are what users of `gh`, `kubectl`, and `docker` expect, but choosing
  them is permanent. Moving to single-dash stdlib spelling later would break every
  script anyone has written.
- An ongoing obligation to track a third-party parser's releases, including the day one
  of them changes a default that quietly alters how an argument is read.
- Revisit if upall's surface ever collapses to a single command with no subcommands and
  no completions, which the roadmap gives no reason to expect.

## Alternatives considered

### Standard library `flag`, with a hand-rolled subcommand dispatcher

Zero dependencies, and the option most in keeping with ADR-0001. The real argument for
it is not just dependency count: M13 wants help output written deliberately rather than
accepted from a framework, and hand-rolling gives that for free instead of fighting a
template system for it.

Rejected on completions. Four shells means four scripts in four dialects, hand-written
against a command tree that grows at M5, M6, M8, M9, and M10. Every flag added in Go
would have to be added again, correctly, in bash and zsh and fish and PowerShell. That
is the same fact written in five places, which is precisely the failure
[ADR-0007](0007-godoc-as-reference-documentation.md) exists to prevent, only in shell
and with no linter watching.

### urfave/cli v3

Lighter than Cobra, a simpler API, and genuinely capable completion support. On the
merits it is close, and it would carry less unused machinery.

Rejected on familiarity rather than capability. ADR-0001 chose Go substantially because
this project is picked up cold after months away, by contributors who arrive without
context. Cobra is what `kubectl`, `gh`, `hugo`, and `docker` are built on, so its command
tree is a shape most Go contributors can already read. That advantage is worth more here
than a smaller dependency, and it is the kind of advantage that does not show up in a
feature comparison.

### Kong, or another struct-tag parser

The CLI declared as annotated structs, which is concise and keeps flags next to the
values they fill.

Rejected because the command tree stops being readable as a tree. Answering "what
commands exist and what does each take" means reading struct tags across several files
rather than one construction site, and that is a question a cold contributor asks first.
Completion support is also thinner than Cobra's.
