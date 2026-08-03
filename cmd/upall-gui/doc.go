// Command upall-gui is the desktop client for upall.
//
// It is a separate binary from `upall` so that the CLI stays lean on servers and
// in CI, where a rendering stack would be dead weight. Both are built from the
// same core and behave the same way; see
// [ADR-0005](../../adr/0005-fyne-for-the-gui-client.md).
//
// This package is wiring and nothing else. Every widget lives in internal/gui,
// which is the only package in upall that imports Fyne.
package main
