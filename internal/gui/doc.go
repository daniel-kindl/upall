// Package gui is the desktop client: every Fyne widget, window, and binding in
// upall lives here and nowhere else.
//
// The containment is the point. Fyne draws its own widgets in its own idiom, and
// letting that idiom leak into the packages beneath would tie the core to a
// toolkit the CLI has no use for. Keeping the import in one package means the
// GUI stays a frontend rather than a second implementation, and means
// [ADR-0005](../../adr/0005-fyne-for-the-gui-client.md) could be revisited
// without touching anything else.
//
// cmd/upall-gui is wiring only. It calls [Run] and imports no Fyne.
//
// # What is here so far
//
// A window that opens and closes. Providers, plans, and progress arrive at M11
// and M12, driven by the same event channel the CLI consumes, so that the GUI
// adds no pipeline logic of its own.
//
// # Building
//
// Fyne renders through OpenGL, which needs cgo, so this is the one package in
// upall that does not cross-compile for free. Linux needs X11 and OpenGL
// development headers to build and Windows needs a C toolchain, both at build
// time only. A shipped binary needs neither, which is the entire reason for the
// toolkit choice. CONTRIBUTING.md lists the packages per platform.
package gui
