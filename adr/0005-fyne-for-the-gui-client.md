# 0005. Fyne for the GUI client

**Status:** Accepted
**Date:** 2026-08-02

## Context

upall ships a desktop client alongside the CLI. The CLI is the primary interface and
the only one that matters on servers, but the everyday audience for "update my
machine" is someone at a desktop who would rather click than remember a command.

The project's headline promise is that a downloaded binary runs with no prerequisites.
That promise is what makes upall worth installing over a shell alias, and it is the
constraint every other decision here has bent around.

Desktop GUI toolkits differ on precisely that point, and the difference is not
cosmetic. Webview-based toolkits render HTML in an OS-provided browser engine. On
Windows that engine ships with the OS. On Linux it does not: WebKitGTK is a package the
user must install, and which one differs by distribution and version.

The GUI arrives at M11 and M12, well after the core is built, so this decision
constrains what the core may look like long before the GUI exists.

## Decision

Fyne, shipped as a separate `upall-gui` binary.

Fyne is pure Go and renders through OpenGL rather than a system webview. Both binaries
build from `go build ./cmd/...` on both platforms and run on a stock desktop with
nothing installed.

Two binaries rather than one, so `upall` stays lean for servers and CI and does not
carry GUI weight for users who will never open a window.

## Consequences

- **The no-prerequisites promise survives on both platforms.** This is the entire
  reason for the choice.
- One toolchain. `go build` produces the GUI. There is no separate frontend build, no
  Node, no bundler, and no second dependency tree to keep patched.
- The GUI is written in Go against the same core, so there is no serialization
  boundary and no IPC protocol to version.
- **The UI will not look as good.** Fyne draws its own widgets in its own idiom, so it
  looks like Fyne rather than like Windows or GNOME. Layout and styling are a Go widget
  tree, which is more laborious and less expressive than CSS for anything visually
  ambitious. This is a real, permanent, user-visible cost.
- **Binaries are large**, 25 to 40 MB for the GUI, because the rendering stack is
  compiled in rather than borrowed from the OS.
- **cgo is required** for the graphics and windowing layer, which complicates the
  cross-compilation story [ADR-0001](0001-go-as-implementation-language.md) relies on.
  The CLI stays pure Go and cross-compiles trivially; the GUI needs a per-target
  toolchain in CI. Splitting the binaries contains this to one of the two.
- Linux GUI builds need X11 and Wayland development headers at build time. Nothing is
  needed at run time, which is what the promise is about, but CI images must carry
  them.
- Accessibility support is weaker than a native or webview UI. For a tool whose full
  functionality is available from a keyboard-driven CLI this is tolerable, but it
  should be stated rather than discovered.

## Alternatives considered

### Wails v3 (webview)

A Go backend with an HTML, CSS, and JavaScript frontend in the OS webview. By a wide
margin the nicest option to build and iterate on: real CSS, real layout, a browser
devtools inspector, and a UI that could actually look good. For a dashboard-shaped
interface, which is what this is, it is the natural fit.

Rejected because Linux requires WebKitGTK to be installed. That is not a large
inconvenience in isolation, but it directly breaks the promise this project is built
on, and it breaks it on the platform where "no dependencies" is hardest to achieve and
most appreciated. Accepting a package-manager prerequisite for the GUI of a tool whose
purpose is managing package managers is the wrong trade.

This is the most expensive rejection in this set of ADRs. It should be revisited if
Linux webview distribution ever stops being a per-distribution problem.

### Embedded web UI served on localhost

`upall gui` serves an HTML interface from assets embedded with `go:embed` and opens the
default browser, which is Syncthing's model. Zero GUI dependencies, one binary, HTML
for the UI, and it works over SSH and on headless machines for free.

Genuinely strong, and the runner-up. Rejected because a browser tab is not a desktop
client. There is no window of its own, no icon, no taskbar presence, a URL to keep
track of, and a local HTTP server with an origin and authentication story to get right.
The request was for a GUI client, and this is something else that happens to have a
graphical interface.

Worth reconsidering later as an additional interface for headless and remote use, where
it has no competition. It is not a replacement for the desktop client.
