# Architecture

How upall is put together, and the rules that hold across the whole codebase.

## Where documentation lives

**This file describes only what spans packages.** How any single package works
internally belongs in that package's `doc.go`, and this file links to it rather than
restating it.

The rule exists because every fact written in two places eventually disagrees with
itself, and nothing breaks when it does, so nobody notices. Godoc is the reference
documentation for this project. Markdown holds only the cross-cutting design that
godoc has no place to put: this file, [ROADMAP.md](ROADMAP.md), and
[the ADRs](../adr/README.md).

If you are about to describe how one package works in a markdown file, stop and write
it in that package's `doc.go` instead. See [ADR-0007](../adr/0007-godoc-as-reference-documentation.md).

The [Repo layout](#repo-layout) table at the bottom is the index into those `doc.go`
files. Packages listed there do not exist yet. They arrive across M1 to M12, and the
table is the contract for where things go when they do.

---

## What upall is

A tool that updates everything on a machine, meaning OS package managers, OS updates,
and containers, behind one command, on Windows and Linux, as self-contained binaries
with no prerequisites.

It ships as two binaries over one shared core. `upall` is the CLI, the primary
interface, and the one that works on headless servers and in CI. `upall-gui` is a
desktop client built with Fyne, using the same core with the same behavior.

### Goals

- **One command updates the machine.** The point is to stop remembering six commands.
- **Nothing is installed to use it.** A downloaded binary runs. No runtime, no
  interpreter, no shared libraries. This constraint has already decided things: see
  [ADR-0005](../adr/0005-fyne-for-the-gui-client.md).
- **Nothing surprising happens.** You see the plan before anything changes.
- **A missing tool is not a failure.** Machines have different software on them.
- **Honest reporting.** Partial success is reported as partial success.

### Non-goals

These are settled. Re-opening one requires an ADR, not a pull request.

**Not a package manager.** upall never resolves dependencies or installs software that
isn't already there. It drives tools that do.

**Not configuration management.** No desired-state model, no convergence, no
inventory. That is Ansible's job and it is good at it.

**No rollback.** apt, winget, scoop, and docker have wildly different or absent
downgrade stories, so a rollback built on them would be unreliable in exactly the
situation you would reach for it. The run journal records what happened so you can undo
it deliberately with the underlying tool.

**No daemon, no scheduling.** systemd timers and Task Scheduler already exist.

**No remote or multi-host operation.** upall updates the machine it runs on.

---

## Two frontends, one core

`upall` and `upall-gui` are thin. Everything that decides anything lives in the core,
and both binaries drive the same code.

This forces one rule, and it is the most load-bearing rule in this document:

> **Nothing below `internal/cli` may assume a terminal.**
>
> No printing. No prompting. No color codes. No TTY checks. No `os.Stdout`. No
> `fmt.Println` reaching for the user.

The core communicates by returning values and emitting typed progress events on a
channel. Each frontend consumes those events and renders them its own way: the CLI as
lines of text, the GUI as a progress list. A core that writes to stdout is a core the
GUI cannot use, and the GUI arrives at M11, long after this code is written.

Anything the core needs from a human is an injected interface. Confirmation is the main
one. The pipeline takes a confirmer, the CLI satisfies it with a terminal prompt, the
GUI satisfies it with a dialog, and tests satisfy it with a canned answer. The core
never knows which it got.

This is why the GUI is a frontend and not a rewrite.

---

## The provider model

A **provider** is one thing that can update software: winget, apt, docker, Windows
Update. Providers are the unit of everything, including detection, planning, elevation,
failure, and reporting.

Every provider answers the same questions:

| | |
|---|---|
| `ID` | Stable short name, used in config and on the command line (`winget`, `apt`) |
| `Platforms` | Which OSes it can run on at all |
| `NeedsElevation` | Whether applying requires admin or root |
| `Detect` | Is this tool actually present and usable on this machine? |
| `Plan` | What would you update? Read-only. Never changes anything. |
| `Apply` | Do it. |

`Detect` returning false is **not an error**. Most machines will not have most
providers, and that is the normal case rather than a degraded one. A provider that
isn't there is omitted from the run and mentioned only if you ask.

### Two kinds of provider, one contract

Most providers are a thin wrapper around a command-line tool: run something, parse a
table, run something else. A few need real logic, such as the Windows Update COM API
and the Docker socket, and cannot be expressed as commands and parsers.

So providers come in two forms. Declarative TOML manifests describe the commands to run
and which parser reads their output, which is the common case. Native providers written
in Go cover the exceptions.

The manifest loader turns a manifest into an adapter satisfying the same interface a
native provider implements. **The rest of the system cannot tell them apart.** The
registry, the pipeline, the config layer, and both frontends see one kind of thing.

The decision rule, for when you're adding one:

> Write a manifest. Reach for a native provider only when the work needs an API, a
> non-textual protocol, or logic no parser can express.

Manifest schema, the parser catalogue, and how the registry resolves and orders
providers are documented in `internal/provider/doc.go`.

---

## The plan/apply pipeline

One linear pipeline. Every phase is separately testable, and every phase takes a
`context.Context` that propagates cancellation all the way down into subprocesses.

```
discover → detect → plan → aggregate → render → confirm → apply → report → journal
             ∥       ∥                                       ∥
```

| Phase | What happens |
|---|---|
| **discover** | Load built-in providers, apply config and `--only`/`--except` filters |
| **detect** | Ask each surviving provider whether it's present. Concurrent. |
| **plan** | Ask each present provider what it would update. Concurrent. |
| **aggregate** | Collect into one `Plan`; note which entries need elevation |
| **render** | Frontend presents the plan, as text or in the GUI |
| **confirm** | Injected confirmer says yes or no. `--yes` skips. |
| **apply** | Execute. Bounded concurrency. |
| **report** | Per-provider outcome, aggregate exit code |
| **journal** | Append the run to disk |

**Detect and plan run concurrently** across providers because they are read-only.
Nothing they do can conflict.

**Apply is bounded, and never concurrent within a single provider.** Most package
managers hold a global lock, so two `apt` invocations at once fail, loudly and
confusingly. Ordering and the concurrency bound are documented in
`internal/pipeline/doc.go`.

**A failing provider does not stop the others.** Each provider's outcome is
independent, so one failure means a failed run rather than an aborted one.

### The three commands

| Command | Behavior |
|---|---|
| `upall plan` | Stops after render. Never changes anything. |
| `upall apply` | Runs the whole pipeline. |
| `upall` | The same as `apply`, and the everyday path. |

Bare `upall` is the default because updating the machine is the point of the tool. It
is safe to type because `confirm` sits between plan and apply: you see the plan and
answer a prompt before anything happens.

`--yes` skips the prompt for CI and scheduled runs. **If stdin is not a terminal and
`--yes` was not passed, upall refuses and exits 2.** It never silently applies because
nobody was there to answer.

---

## Elevation

upall runs unprivileged by default and elevates as narrowly as it can.

Providers declare whether applying needs admin or root. Planning never does. The
rendered plan marks which entries will require elevation, so you know before you answer
the prompt rather than after.

At apply time, only the providers that declared the need are elevated, using `sudo` on
Linux and a UAC re-exec on Windows. Everything else runs as you. The alternative of
re-executing the whole run as root was rejected in
[ADR-0003](../adr/0003-per-provider-elevation.md).

If elevation is unavailable or refused, those providers report as blocked with the
command you'd need to run yourself. The rest of the run proceeds normally.

Mechanism lives in `internal/elevate/doc.go`.

---

## Cross-platform rules

Windows and Linux are both first-class. macOS is planned but not present; see ROADMAP's
Post-1.0 section.

**Build argv arrays. Never build a shell command string.**

```go
exec.CommandContext(ctx, "winget", "upgrade", "--all", "--silent")   // yes
exec.CommandContext(ctx, "sh", "-c", "winget upgrade "+pkg)          // no
```

There is no quoting rule that is correct on both `cmd.exe` and `sh`, and string
interpolation into a shell is an injection surface in a tool that runs elevated.
Passing argv sidesteps both problems completely. This is not a style preference, and
reviews reject violations.

**Platform-specific code goes behind build tags**, in `_windows.go` and `_linux.go`
files. Scattering `runtime.GOOS` branches through shared logic is a review-blocking
defect. It makes every function's behavior depend on where it runs, and it hides
compile errors from the platform you aren't currently on.

**All paths go through `path/filepath`**, and every user-facing directory for config,
data, and cache is resolved by `internal/paths` and nowhere else. Hardcoding
`~/.config` or `%APPDATA%` anywhere but there is a bug.

**Providers degrade silently when absent.** Covered above, and repeated here because it
is where cross-platform code most often gets this wrong.

**CI runs the full suite on `windows-latest` and `ubuntu-latest` for every pull
request.** A change that compiles on only one is not mergeable. This is the rule that
actually enforces the four above, so it is neither optional nor skippable.

---

## Exit codes

The exit code contract is a public interface under semver. Changing one is a breaking
change.

| Code | Meaning |
|---|---|
| `0` | Success. Includes "nothing needed updating" and "user declined at the prompt". |
| `1` | One or more providers failed. Others may have succeeded. |
| `2` | Usage error, bad config, or refusal to run non-interactively without `--yes`. |
| `130` | Interrupted (Ctrl-C). Partial work is reported and journaled. |

`upall plan --exit-code` follows the `diff` and `terraform plan` convention instead, so
it can drive scripts: `0` nothing to update, `1` updates available, `2` error.

---

## Error taxonomy

Five kinds of thing go wrong. Each renders differently and maps to a different exit
code, and the core distinguishes them so frontends don't have to parse strings.

| Kind | Meaning | Rendering | Exit |
|---|---|---|---|
| **absent** | Provider isn't installed | Silent unless asked | `0` |
| **failed** | Provider ran and failed | Named, with captured stderr tail | `1` |
| **needs-elevation** | Blocked, not attempted | Blocked, with the manual command | `1` |
| **timeout** | Exceeded its deadline | Named, with the deadline | `1` |
| **cancelled** | Ctrl-C during the run | What finished, what didn't | `130` |

Captured output is truncated to a tail when rendered. The journal keeps more.

---

## Security model

**upall executes package managers, sometimes as root.** That is its function, so the
security posture needs stating rather than assuming.

**A provider manifest is arbitrary command execution by definition.** A manifest says
"run this command", and that is the entire point of the format. It follows that
manifests are not user-supplied content to be trusted casually.

Built-in manifests ship embedded in the binary via `go:embed`. They are reviewed in
pull requests and cannot be modified without replacing the binary.

User-supplied manifest overrides are off by default. Enabling them is an explicit,
documented decision that grants arbitrary code execution, under elevation if the
manifest asks for it. The documentation says so in those words.

Nothing is fetched at runtime. upall does not download manifests, providers, or updates
to itself during a run. The only network traffic is whatever the underlying package
managers generate.

**Elevation is narrow and visible.** Only providers that declared the need are
elevated, the plan shows which ones before you confirm, and refusing elevation degrades
the run rather than failing it.

**Command construction is argv-only**, which removes shell injection as a category
rather than mitigating it.

To report a vulnerability, see [SECURITY.md](../SECURITY.md).

---

## Repo layout

Each package has one job, stated here in one line and explained properly in its
`doc.go`. Nothing in this table exists yet. The ROADMAP says which milestone introduces
each one.

| Path | Job |
|---|---|
| `cmd/upall/` | CLI entrypoint. Wiring only. |
| `cmd/upall-gui/` | GUI entrypoint. Wiring only. |
| `internal/core/` | Domain types: `Update`, `Plan`, `Result`, `Provider`, `Platform`. No I/O, no dependencies. |
| `internal/provider/` | Registry, manifest loader, output parsers |
| `internal/provider/manifests/` | Embedded declarative providers (`*.toml`) |
| `internal/provider/native/` | Providers needing real logic |
| `internal/pipeline/` | Plan and apply orchestration, progress events |
| `internal/journal/` | Run history on disk |
| `internal/exec/` | Subprocess runner. The seam tests fake. |
| `internal/elevate/` | UAC and sudo |
| `internal/paths/` | Platform config, data, and cache directories |
| `internal/cli/` | Commands, flags, terminal rendering. The only place a terminal exists. |
| `internal/gui/` | Fyne views and bindings |

**Everything is `internal/`.** Nothing here is importable from outside the module, which
is deliberate: it leaves the domain types free to change shape while they are still
settling. Making a package public is a decision with permanent semver consequences and
requires an ADR superseding
[ADR-0004](../adr/0004-internal-first-api-surface.md).

`internal/exec` is worth calling out. Every subprocess in the codebase goes through it,
which is what makes the rest of the system testable. Tests inject a fake runner, and no
test ever shells out to a real package manager.
