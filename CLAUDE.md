# upall

Updates everything on a machine: package managers, OS updates, containers.
Go. CLI (`upall`) plus a Fyne GUI (`upall-gui`) over one shared core.
Self-contained binaries, no prerequisites. Windows and Linux now, macOS post-1.0.

@docs/ARCHITECTURE.md
@docs/ROADMAP.md
@AGENTS.md

## Start here

Open `docs/ROADMAP.md` and find the first milestone with unchecked boxes. That is the
current work. Do not start the next milestone until every box above it is ticked.

## Invariants

- Build argv arrays, never shell strings. Reviews reject violations.
- Platform code goes behind build tags, not `runtime.GOOS` branches.
- Unprivileged by default. Providers declare elevation.
- "Not installed" is not an error.
- Nothing below `internal/cli` may assume a terminal: no printing, no prompting, no
  TTY checks. The GUI shares that core.
- Every exported identifier has a godoc comment. Godoc IS the reference docs.
- How one package works goes in its `doc.go`, never in `docs/`. Never write it twice.
- Everything is `internal/`. Making a package public needs an ADR superseding 0004.
- Every subprocess goes through `internal/exec`. No test invokes a real package
  manager.
- CI green on windows-latest AND ubuntu-latest, or it does not merge.

## Git

Work on `feature/*`. PR into `dev` only, never into `main`. Conventional commits.
Full rules in `AGENTS.md`.

## Commands

```
go build -o bin/ ./cmd/...   # -o is required: two main packages
go test ./...
go vet ./...
golangci-lint run
go doc ./internal/<pkg>
```

Decisions already settled, and what they cost, are recorded in `adr/`. Read before
reversing one.
