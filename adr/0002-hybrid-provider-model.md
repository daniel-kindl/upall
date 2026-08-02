# 0002 — Hybrid provider model: manifests and native code

**Status:** Accepted
**Date:** 2026-08-02

## Context

A provider is one thing that can update software. upall will have a lot of them, and
the long-term health of the project depends on how cheap it is to add the next one.

Look at what they actually do and they fall into two unequal groups.

The large group is a shell wrapper. `winget upgrade` lists updates in a table;
`winget upgrade --all --silent` applies them. `apt list --upgradable` lists;
`apt upgrade -y` applies. Detection is running the tool with `--version` and seeing
whether it exists. There is no logic here — only commands, and a way to read their
output.

The small group cannot be expressed that way at all. Windows Update is a COM API.
Docker is a socket speaking JSON. Compose needs to resolve project files, compare
image digests, and decide which services to recreate. These need real code.

Serving only the small group's needs makes every trivial provider a Go file, a test,
and a release. Serving only the large group's needs makes the interesting providers
impossible.

## Decision

Both, behind one interface.

- **Declarative TOML manifests** describe detection, plan, and apply as commands plus
  a named output parser. This is the default and the common case.
- **Native Go providers** implement the same interface directly, for the exceptions.

The manifest loader produces an adapter satisfying the identical interface a native
provider implements. **Nothing downstream can tell them apart** — not the registry,
the pipeline, the config layer, or either frontend.

The rule for choosing: *write a manifest, unless the provider needs an API, a
non-textual protocol, or logic no parser can express.*

## Consequences

- Adding a typical provider is a TOML file and a parser test. No new Go code, no new
  control flow, nothing to review beyond the commands themselves.
- Manifests are data, so they can be validated as data: unknown fields, missing
  fields, and unknown parser names fail loudly at load with a file and field name.
- The interesting providers stay possible, and pay no tax for the manifests' existence.
- **Two code paths for one concept.** Every change to the provider contract must be
  made in the interface, the manifest schema, and the loader. This is the real cost
  and it is permanent.
- The parser catalogue becomes a shared dependency: parsers must handle every
  manifest's output, so a parser change can break unrelated providers. Fixture tests
  per provider are the mitigation, and they are required.
- Manifests constrain what a manifest provider can do, by design. There will be a
  provider that is *almost* expressible and tempts someone to add a field for it.
  Adding that field is how this schema turns into a programming language. Write it
  native instead.
- **A manifest is arbitrary command execution.** This is why built-in manifests are
  embedded in the binary and user overrides are off by default. See ARCHITECTURE.md's
  security model.

## Alternatives considered

### Native providers only

One interface, one code path, no schema, no parser catalogue, no loader. Genuinely
the simplest thing that works, and the honest argument for it is that the manifest
system is machinery that exists to avoid writing fifteen small, boring Go files.

Rejected because those fifteen files are not the cost — the ongoing one is. Every
provider becomes a code change, a review, and a release, which raises the price of
the project's most common contribution. It also invites drift: fifteen hand-written
providers will implement "is this tool installed" fifteen slightly different ways.

### Manifests only

Force everything into the declarative model and grow the schema until it fits. Fewer
moving parts than the hybrid, and one uniform way to add a provider.

Rejected because the schema would have to grow conditionals, loops, and structured
API calls to express Windows Update and Compose — at which point it is a programming
language with no debugger, no type checking, and no tests, reimplemented in TOML.
That failure mode is well documented across configuration formats and it does not end
well.

### External plugin binaries

Providers as separate executables speaking JSON over stdio. Maximum flexibility, any
implementation language, third parties ship providers without touching core.

Rejected because it breaks the no-prerequisites promise the moment anything is not
built in, and because the trust story is considerably worse: a plugin discovered on
`PATH` and executed — potentially elevated — is a far larger attack surface than an
embedded manifest. The flexibility buys nothing upall needs; nobody is asking to
write a provider in Python.
