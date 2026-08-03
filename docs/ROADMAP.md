# Roadmap

Fourteen milestones from empty repo to v1.0.

## How to use this file

**Find the first milestone with unchecked boxes. That is the current work.**

Milestones are sequential. Do not start one until every box above it is ticked, because
each is built on the one before, and skipping ahead means building on something that
isn't finished.

A milestone ends at a merge to `dev` where:

- every acceptance criterion below it is ticked,
- CI is green on `windows-latest` **and** `ubuntu-latest`,
- and the binaries do something you can demonstrate.

Acceptance criteria are written to be objectively checkable: a command to run, an
observable output, a file that exists. If a criterion can be argued about, it is
written wrong. "Works well" and "is robust" are not criteria.

Terms and rules referenced throughout are defined in
[ARCHITECTURE.md](ARCHITECTURE.md). Contribution mechanics are in
[../AGENTS.md](../AGENTS.md).

---

## M0. Foundation and governance

**Goal:** A cold session can open this repo, read three files, and know what to build
and why.

No application code. Documents, decision records, community standards, and the CI
scaffolding that lights up at M1.

- [x] `docs/ARCHITECTURE.md` covers the provider model, the plan/apply pipeline, the
      frontend boundary, cross-platform rules, exit codes, error taxonomy, and the
      security model.
- [x] `docs/ROADMAP.md` (this file) has M0 through M13, each with checkbox criteria,
      plus a Post-1.0 section pairing every deferred item with its revisit trigger.
- [x] `adr/README.md` explains the process; `adr/0000-template.md` is the template;
      ADRs 0001 through 0007 record every decision settled before code, each naming the
      alternatives rejected and why.
- [x] `CLAUDE.md` fits one screen and `@`-imports ARCHITECTURE, ROADMAP, and AGENTS.
- [x] `AGENTS.md` specifies conventional commits, semver, the branching model, the
      godoc policy, and PR scoping.
- [x] Community standards complete: `README.md`, `LICENSE` (Apache-2.0),
      `CODE_OF_CONDUCT.md`, `CONTRIBUTING.md`, `SECURITY.md`, issue templates, PR
      template.
- [x] `.gitattributes` contains `* text=auto eol=lf`, verified with
      `git check-attr text eol -- README.md`.
- [x] `.github/dependabot.yml`, `.github/workflows/ci.yml`, and
      `.github/workflows/codeql.yml` exist with `paths:` filters, so a doc-only commit
      triggers no Go job and leaves no red check.
- [x] Every `@`-import in CLAUDE.md and every relative markdown link resolves.

**Landing** (needs network, done once the above is reviewed):

- [x] M0 commit on `main`, `dev` branched from it, both pushed.
- [x] Branch protection on `main` and `dev`: no direct pushes, PR required.
- [x] Repo description and topics set.
- [x] `gh api repos/daniel-kindl/upall/community/profile` reports the community
      checklist complete, at 100%.

> **Bootstrap note.** This commit was pushed directly to `main` and `dev` before branch
> protection was enabled, because protecting the branches first would have blocked the
> commit that records the protection. It is the only commit exempt from
> [AGENTS.md](../AGENTS.md#branching). Every commit after it goes through a pull
> request into `dev`.

---

## M1. Skeleton and pipelines

**Goal:** Both binaries build and run on both OSes, and every automated check works.

- [x] `go.mod` declares the module path and pins the current stable Go release.
- [ ] `go build ./cmd/...` produces `upall` and `upall-gui` on Windows and Linux.
- [ ] `upall version` prints a version, commit SHA, and build date injected at build
      time via `-ldflags`, not hardcoded.
- [ ] `upall-gui` opens an empty window and closes cleanly.
- [x] `.golangci.yml` enables `revive` with the `exported` rule, so a missing doc
      comment on an exported identifier fails the build. It must use the
      **golangci-lint v2 schema**, because `golangci-lint-action` v7 and later
      support v2 only. Most examples online still show the incompatible v1 syntax.
- [x] `go vet ./...`, `golangci-lint run`, and `go test ./...` all pass.
- [ ] CI runs build, vet, lint, and test on `windows-latest` and `ubuntu-latest` for
      every PR, and the matrix is required for merge.
- [ ] CodeQL runs on PRs and weekly, and reports no errors.
- [ ] Dependabot opens PRs for `gomod` and `github-actions`.

---

## M2. Core domain

**Goal:** The vocabulary the whole system speaks, with no way to do I/O.

`internal/core`: `Update`, `Plan`, `Result`, `Provider`, `Platform`.

- [ ] `internal/core` imports nothing outside the standard library, and nothing from
      elsewhere in this module.
- [ ] The package compiles with no reference to `os/exec`, `os.Stdout`, or `fmt.Print*`,
      so the types cannot perform I/O.
- [ ] `Provider` declares `ID`, `Platforms`, `NeedsElevation`, `Detect`, `Plan`, and
      `Apply` as described in ARCHITECTURE.md.
- [ ] `Platform` gating answers "can this provider run here" for windows and linux,
      with darwin representable but unused.
- [ ] `internal/core/doc.go` explains the type relationships and the lifecycle of an
      `Update` through a run.
- [ ] Table-driven tests cover plan aggregation, result merging, and exit-code
      derivation, including the empty-plan and all-failed cases.
- [ ] `go doc ./internal/core` reads as usable reference documentation.

---

## M3. Execution substrate

**Goal:** One way to run a subprocess, and one place tests replace it.

- [ ] `internal/exec` exposes a runner interface plus a real implementation, and all
      subprocess execution in the codebase goes through it.
- [ ] The runner takes argv as `[]string`. There is no API that accepts a command
      string, so no caller can build one.
- [ ] Every call takes a `context.Context`; cancelling it kills the process, and a test
      proves the process is gone.
- [ ] Per-command timeouts are supported and surface as the `timeout` error kind.
- [ ] stdout and stderr are captured separately and returned. Neither is written to the
      terminal by this package.
- [ ] A fake runner ships for tests, with canned output, canned exit codes, and
      recorded invocations.
- [ ] Structured logging records argv, duration, and exit code at debug level, with no
      secrets and no full environment.
- [ ] Tests pass on both OSes using a command that exists on both.

---

## M4. Provider registry

**Goal:** Both kinds of provider exist, are indistinguishable to callers, and one works
end to end on each OS.

- [ ] `internal/provider` holds a registry that resolves providers by ID and filters by
      platform.
- [ ] Native providers implement `core.Provider` directly.
- [ ] TOML manifests are loaded into an adapter satisfying the same interface, and a
      test asserts the registry cannot distinguish the two.
- [ ] Manifests are embedded with `go:embed`, so the binary needs no files on disk.
- [ ] At least three named output parsers exist (table, JSON, line-per-item), each
      tested against captured real-world fixture output.
- [ ] Manifest validation rejects unknown fields, missing required fields, and unknown
      parser names, with an error naming the file and field.
- [ ] `winget` works end to end on Windows and `apt` on Linux: detect, plan, apply.
- [ ] `Detect` returns false rather than an error when the tool is absent, proven by a
      test on the OS where the tool does not exist.
- [ ] `internal/provider/doc.go` documents the manifest schema and parser catalogue.

---

## M5. Plan pipeline

**Goal:** `upall plan` tells you what would change, and changes nothing.

- [ ] `internal/pipeline` runs discover, detect, plan, and aggregate.
- [ ] Detect and plan run concurrently across providers. A test with deliberately slow
      fakes proves total time is bounded by the slowest rather than the sum.
- [ ] One provider failing to plan does not prevent others from planning.
- [ ] The pipeline emits typed progress events on a channel, and no package below
      `internal/cli` writes to stdout. Enforced by a test or lint rule, not by review.
- [ ] `upall plan` renders providers, counts, and per-update detail, marking entries
      that will need elevation.
- [ ] `upall plan` on a machine with nothing to update says so plainly and exits 0.
- [ ] `upall plan --exit-code` exits 0 with no updates, 1 with updates, 2 on error.
- [ ] No provider `Apply` is reachable from the plan path. A fake that fails on `Apply`
      proves it is never called.

---

## M6. Apply pipeline

**Goal:** `upall` updates the machine, and tells the truth about what happened.

- [ ] `upall apply --yes` applies every planned update and exits 0 when all succeed.
- [ ] One provider failing does not abort the others. The run exits 1 and the summary
      names the failed provider with its captured stderr tail.
- [ ] Apply is never concurrent within a single provider, and the concurrency bound
      across providers is configurable with a documented default.
- [ ] Bare `upall` prints the plan, prompts once, and on any answer but y/yes exits 0
      having caused no side effects.
- [ ] With stdin not a terminal and no `--yes`, the run refuses with exit 2 and a
      message naming `--yes`. It never silently applies.
- [ ] Ctrl-C cancels the in-flight provider context, waits for it to unwind, reports
      what completed, and exits 130.
- [ ] Confirmation is an injected interface satisfied by the CLI, not a prompt the
      pipeline performs itself.
- [ ] `go test ./internal/pipeline/...` covers all-ok, partial-failure, refusal, and
      cancellation against fakes. No test invokes a real package manager.

---

## M7. Elevation

**Goal:** Do the privileged parts, without running everything as root.

- [ ] Providers declare `NeedsElevation`, and manifests express it as a field.
- [ ] The rendered plan marks which entries require elevation, before the prompt.
- [ ] Only providers that declared the need are elevated. A test asserts unprivileged
      providers are invoked without any elevation wrapper.
- [ ] Linux elevates via `sudo`; Windows via a UAC re-exec of a helper.
- [ ] Elevation refused or unavailable marks those providers blocked, prints the exact
      command to run manually, and lets the rest of the run proceed.
- [ ] A non-interactive run that would need elevation and cannot get it fails those
      providers rather than hanging on an invisible prompt.
- [ ] Already running as root or Administrator skips the elevation machinery.
- [ ] `internal/elevate/doc.go` documents the mechanism per platform.

---

## M8. Provider breadth

**Goal:** Enough providers that upall is worth running.

Windows: `winget`, `scoop`, `chocolatey`, Windows Update.
Linux: `apt`, `dnf`, `pacman`, `snap`, `flatpak`.

- [ ] Every provider above detects, plans, and applies on a machine that has it.
- [ ] Every provider that can be a manifest is a manifest. Each native provider's
      `doc.go` states why a manifest could not express it.
- [ ] Each provider has parser tests against captured real output, including the
      no-updates-available case and at least one error case.
- [ ] Windows Update reports pending reboots. The run summary surfaces that a reboot is
      required, and never reboots on its own.
- [ ] A machine with none of a platform's providers installed produces a clean run that
      exits 0, not an error.
- [ ] `upall --only` and `--except` accept these IDs. Full support lands at M10.

---

## M9. Containers

**Goal:** Container images and compose stacks come along for the ride.

- [ ] Docker and Podman are both detected, and each is absent-tolerant independently.
- [ ] Plan lists images with a newer digest available for the tag currently in use.
- [ ] Apply pulls those images.
- [ ] Compose projects are discovered, their images updated, and only affected services
      recreated.
- [ ] Containers whose image did not change are not recreated. A test proves it.
- [ ] Compose files are never rewritten. upall does not edit your configuration.
- [ ] Digest-pinned and locally built images are skipped with a stated reason rather
      than failed.
- [ ] A stopped container, or a compose project that fails to come back up, is reported
      as a failure with its logs rather than silently left down.

---

## M10. Config, filtering, and output

**Goal:** Make it yours, and make it scriptable.

- [ ] Config is TOML, at the platform config directory resolved by `internal/paths`,
      overridable with `--config`.
- [ ] Providers can be enabled or disabled in config, and the concurrency bound and
      per-provider timeouts are configurable.
- [ ] `--only` and `--except` filter by provider ID, and reject unknown IDs with a
      message listing the valid ones.
- [ ] `upall providers` lists every known provider with platform, detected status, and
      whether it needs elevation.
- [ ] A missing config file is normal and produces documented defaults. A malformed one
      exits 2, naming the file, line, and problem.
- [ ] `--json` on `plan` and `apply` emits a documented, versioned schema, and emits
      nothing else on stdout.
- [ ] JSON output is valid and parseable even when the run fails or is cancelled.
- [ ] Every run appends to a journal: timestamp, providers, planned, applied, failed,
      durations, exit code.
- [ ] `upall history` lists recent runs; `upall history <id>` shows one in full.
- [ ] The journal rotates or bounds its size. It cannot grow without limit.

---

## M11. GUI foundation

**Goal:** `upall-gui` shows you the machine and what it would do.

- [ ] `upall-gui` builds and runs on Windows and Linux from `go build ./cmd/...`, with
      no libraries installed beyond what a stock desktop has.
- [ ] It lists providers with detected status and elevation requirement.
- [ ] It runs detect and plan, and displays the plan grouped by provider.
- [ ] Progress is driven by the pipeline's event channel, the same events the CLI
      consumes. The GUI adds no pipeline logic of its own.
- [ ] The window stays responsive throughout, and no pipeline work runs on the UI
      thread.
- [ ] A provider failing during plan shows as failed in place, without taking down the
      window.
- [ ] `internal/gui` contains all Fyne code. No Fyne import appears anywhere else.

---

## M12. GUI completion

**Goal:** The GUI can do everything the CLI can.

- [ ] Apply runs from the GUI with per-provider progress and live status.
- [ ] Confirmation is a dialog satisfying the same confirmer interface the CLI's prompt
      satisfies.
- [ ] Elevation prompts surface as dialogs. Refusing leaves the app usable and the rest
      of the run intact.
- [ ] Config is viewable and editable, writing the same TOML the CLI reads.
- [ ] Run history is browsable, backed by the same journal.
- [ ] Cancelling mid-apply behaves as Ctrl-C does: unwind, report, stay usable.
- [ ] The app has an icon and window title, and packages cleanly for both OSes.
- [ ] Anything the CLI can do, the GUI can do. Differences are listed in
      `internal/gui/doc.go` as deliberate.

---

## M13. Release and v1.0

**Goal:** Someone who has never seen this repo installs it and it works.

- [ ] Tagging on `main` publishes a stable release, and merging to `dev` publishes a
      `-dev.N` prerelease. Both are semver, per
      [ADR-0006](../adr/0006-branching-model-and-release-channels.md).
- [ ] Releases carry both binaries for windows/amd64, windows/arm64, linux/amd64, and
      linux/arm64.
- [ ] Every artifact has a checksum, and the checksum file is signed.
- [ ] Release notes are generated from conventional commits.
- [ ] `install.sh` and `install.ps1` install the latest stable release, verify the
      checksum, and support pinning a version.
- [ ] Shell completions ship for bash, zsh, fish, and PowerShell.
- [ ] `--help` is a real deliverable. Every command and flag description reads as a
      sentence, `upall --help` shows a worked example, and `upall <cmd> --help` states
      the exit codes that command can return.
- [ ] The README gets a new user from "never heard of this" to a successful run.
- [ ] A clean Windows VM and a clean Linux VM each install from the published artifacts
      and complete a real run.
- [ ] `v1.0.0` tagged on `main`.

---

## Post-1.0

Deferred deliberately, each with the condition that should bring it back. These are
triggers rather than wishes, so that a future session can tell whether the moment has
arrived instead of guessing.

| Deferred | Revisit when |
|---|---|
| **Public Go API on pkg.go.dev** | Someone concrete wants to import upall as a library. See [ADR-0004](../adr/0004-internal-first-api-surface.md). |
| **Documentation site** | The manifest schema is stable **and** someone outside the project has authored a provider. That is a real authoring surface, and the only thing here that outgrows a README. |
| **macOS support** | Windows and Linux are stable at v1.0. Homebrew, MAS, and softwareupdate are the providers, and the manifest model should already cover the first two. |
| **Self-update** | The release channels have proven out over several real releases. Shipping a self-updater before the thing it updates to is reliable is backwards. |
| **Toolchain providers** (npm, pipx, cargo, rustup, gem) | The OS package managers are complete and stable. These are easy manifests, deferred for scope rather than difficulty. |
| **Scheduling and daemon mode** | No trigger. Out of scope, because systemd timers and Task Scheduler already do this. |
| **Remote and multi-host** | No trigger. Out of scope, because this is Ansible's job. |

The last two are recorded so a future session knows they were considered and declined
rather than overlooked.
