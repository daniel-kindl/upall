# 0004. Internal-first API surface

**Status:** Accepted
**Date:** 2026-08-02

## Context

upall ships two binaries, `upall` and `upall-gui`, over one shared core. That
arrangement raises a question it does not answer: should the core be importable from
outside the module?

It is worth separating two things that look connected and are not.

Sharing code between the two binaries requires nothing. Both live in this module, and
Go's `internal/` rule is scoped to the module, so `cmd/upall` and `cmd/upall-gui` can
import `internal/core` freely. The two-frontend design forces no decision here at all.

Publishing the API is a separate choice. Packages outside `internal/` are importable
by anyone and are rendered on pkg.go.dev. Packages inside are neither.

[ADR-0007](0007-godoc-as-reference-documentation.md) makes godoc the reference
documentation for this project, which is the reason the question is live. It is
tempting to conclude that the packages must be public for their documentation to
exist. They need not. `go doc ./internal/pipeline` works, and a local `godoc -http`
server renders internal packages in full. The only thing `internal/` withholds is the
publicly hosted pkg.go.dev page.

The asymmetry that decides this: moving from `internal/` to public is a non-breaking
change available at any time, while moving from public to `internal/` is a major
version bump.

## Decision

Everything is `internal/`, including `core`, `provider`, `pipeline`, and `journal`.
Nothing in upall is importable from outside the module.

The Go API is consequently **not** part of the semver contract. The public interfaces
under semver are the CLI surface and exit codes, the config schema, the JSON output
schema, and the provider manifest schema, all enumerated in [AGENTS.md](../AGENTS.md).

## Consequences

- The domain types stay free to change shape. At M2 they have never been used by
  anything, and the first three providers will reveal that they are wrong in ways no
  amount of upfront design would have found. Renaming a field is a commit, not a
  release cycle.
- Refactoring across package boundaries needs no deprecation period, no compatibility
  shim, and no argument about whether it is breaking.
- Godoc discipline is unaffected and enforced everywhere regardless. Every package
  gets a `doc.go`, every exported identifier gets a comment, and `revive`'s `exported`
  rule fails the build otherwise. Contributors read it with `go doc`.
- **No pkg.go.dev page.** Anyone wanting to read the API in a browser must clone and
  run `godoc` locally. This is the whole cost of the decision.
- **Nobody can build on upall as a library.** If someone wants to embed the update
  pipeline in their own tool, they cannot, and will not ask twice.
- Revisit trigger: a concrete external consumer wants to import it. Not a hypothetical
  one, and not "it would be nice if", but someone with an actual use. Promotion is
  mechanical at that point. Move the package up a level, leaving the `internal/` path
  as a thin alias if anything internal still refers to it.

Making any package public requires an ADR superseding this one. The bar is not high,
but the decision should be deliberate rather than a side effect of someone moving a
file.

## Alternatives considered

### Public core from the start

`core/`, `provider/`, `pipeline/`, and `journal/` public at M2. pkg.go.dev renders the
documentation automatically, third parties can build on it, and the "godoc is the
documentation" policy gains a public home rather than a local one.

Rejected on timing rather than merit. Every exported identifier would join the semver
contract at exactly the moment the types are least settled, before a single provider
has been written against them. The first real providers will change these types, and
each change would be a breaking release owed to users who do not exist yet. The
benefit is available later at no cost. The obligation is not removable later at any.

### Public domain types only

Make `core/` public, exposing `Update`, `Plan`, and `Result`, and keep everything else
internal. The smallest, most stable, most useful surface for an outside reader, with a
fraction of the obligation.

A reasonable middle path, and the runner-up. Rejected because it takes on the permanent
part of the cost for the least valuable part of the API. Data types with no behavior
are the part an outsider could most easily define themselves, while the pipeline they
would actually want to reuse stays out of reach. It buys a pkg.go.dev page for a
package that mostly says `type Update struct`.
