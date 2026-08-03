# Contributing to upall

Thanks for looking. This file covers getting set up and getting a change merged.

> **The project is at milestone M1**, which builds the skeleton: both binaries, the
> version command, and the automated checks. Nothing updates anything yet. See
> [docs/ROADMAP.md](docs/ROADMAP.md) for the first unchecked box, which is always the
> current work.

## Before you start

Three files, in this order:

1. [docs/ROADMAP.md](docs/ROADMAP.md). The first milestone with unchecked boxes is the
   current work. Milestones are sequential.
2. [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md). How it fits together, and the rules
   that hold everywhere.
3. [AGENTS.md](AGENTS.md). Commit format, branching, PR scope, and all the mechanics.

Read [adr/](adr/README.md) too, before proposing anything that reverses a settled
decision. Every one of those records what it cost and what was rejected. Disagreeing is
fine, and that is what a superseding ADR is for, but do it knowing what the argument
already was.

## Setup

You need the current stable [Go](https://go.dev/dl/) and
[golangci-lint](https://golangci-lint.run/welcome/install/). Building the GUI, or
running the tests with `-race`, also needs a C toolchain; see
[Building the GUI](#building-the-gui) below.

```console
git clone https://github.com/daniel-kindl/upall.git
cd upall
go build -o bin/ ./cmd/...
go test ./...
golangci-lint run
```

The `-o bin/` is not decoration. There are two main packages, and given more than one,
`go build` compiles each and writes none of them: it checks that they build rather than
producing them. Without `-o` you get a successful command and no binaries.

A plain `go build` is enough for development, including for `upall version`. The
toolchain stamps the commit, its timestamp, and whether your tree was dirty into every
binary built inside a repository, so the only thing missing is a version number, and a
working tree does not have one:

```console
$ ./bin/upall version
upall dev (2324cc3-dirty, 2026-08-03T14:32:29Z)
go1.26.5 linux/amd64
```

Release builds supply the number with a linker flag. Nothing needs to type this by
hand; the release pipeline does it.

```console
go build -ldflags "-X github.com/daniel-kindl/upall/internal/buildinfo.version=1.2.3" -o bin/ ./cmd/...
```

### Building the GUI

Fyne renders through OpenGL, which means cgo, so `upall-gui` needs a C toolchain and
some development headers to *build*. Nothing extra is needed to *run* it, which is the
whole point of [ADR-0005](adr/0005-fyne-for-the-gui-client.md).

```console
# Debian/Ubuntu
sudo apt install libgl1-mesa-dev xorg-dev libx11-dev libwayland-dev libxkbcommon-dev

# Fedora
sudo dnf install libX11-devel libXcursor-devel libXrandr-devel \
                 libXinerama-devel mesa-libGL-devel libXi-devel libXxf86vm-devel \
                 wayland-devel libxkbcommon-devel
```

The Wayland packages are needed even on an X11-only desktop. glfw compiles its Wayland
backend regardless of which display server you will actually run under, so omitting them
fails the build on a missing `wayland-client-core.h`.

On Windows you need gcc, which the OS does not ship. mingw-w64 works:

```console
winget install BrechtSanders.WinLibs.POSIX.UCRT
```

The same toolchain is what `go test -race` needs on Windows, so this is not optional for
running the test suite either, GUI or no GUI.

The CLI itself has no such requirement on any platform.

## Finding the documentation

There is no documentation site. Reference documentation lives in the code:

```console
go doc ./internal/provider          # package overview
go doc ./internal/provider.Registry # one identifier
godoc -http=:6060                   # all of it, in a browser
```

This is deliberate. See [ADR-0007](adr/0007-godoc-as-reference-documentation.md).
Markdown covers only what spans packages: architecture, roadmap, decisions.

## Making a change

```console
git switch dev
git pull
git switch -c feature/short-description
```

**Pull requests go to `dev`. Never to `main`.** A PR targeting `main` gets closed
rather than retargeted, because `main` only ever contains released code. The full model
is in [AGENTS.md](AGENTS.md#branching).

Commits follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat(provider): add flatpak manifest
fix(exec): kill the process group on context cancellation
```

Keep a PR to one thing. If describing it needs the word "and", it is likely two PRs.
Refactors ship separately from behavior changes, because a diff that both moves code
and changes what it does cannot be reviewed for either.

Tick the ROADMAP box your change completes, in the same PR.

## Adding a provider

Usually a TOML manifest and a test, with no Go code at all. Write a native provider
only when it needs an API, a non-textual protocol, or logic no parser can express, and
say why in the package's `doc.go`.

Every provider needs fixture tests built from **real captured output**, including the
nothing-to-update case and at least one error case. Run the tool by hand, paste what it
printed into a fixture file, and test the parser against that. Invented output tests the
parser against your imagination.

Two rules that reviews enforce without exception:

- **Build argv arrays, never shell strings.** No quoting rule is correct on both
  `cmd.exe` and `sh`, and string interpolation into a shell is an injection surface in
  a tool that runs elevated.
- **A missing tool is not an error.** `Detect` returns false. Most machines will not
  have most providers, and that is normal.

## Testing

```console
go test ./...
```

**No test may invoke a real package manager.** Fake `internal/exec` instead, because a
test suite that mutates the machine running it is not a test suite. There is a fake
runner for this; use it.

CI runs everything on Windows and Linux for every PR, and both must pass. If you can
only test on one, say so in the PR and let CI cover the other.

## Reporting things

- Bugs and feature requests: the [issue
  templates](https://github.com/daniel-kindl/upall/issues/new/choose).
- A provider you want supported: there is a template for that specifically.
- Security vulnerabilities: not a public issue. See [SECURITY.md](SECURITY.md).

## A note on pace

This project is maintained by one person in bursts, with long gaps. Reviews may take a
while. That is not disinterest, and a ping on a stale PR is welcome rather than rude.

Everyone participating is expected to follow the
[Code of Conduct](CODE_OF_CONDUCT.md).
