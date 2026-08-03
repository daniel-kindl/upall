// Package cli implements the upall command-line interface: the command tree,
// the flags hanging off it, and the rendering of results as text.
//
// This is the only package in upall permitted to assume a terminal. Everything
// beneath it returns values and emits progress events rather than printing,
// which is what lets internal/gui drive the same core without a rewrite. The
// rule and its reasoning are in docs/ARCHITECTURE.md.
//
// cmd/upall is wiring only: it calls [Execute] and exits with the code it
// returns.
//
// # Exit codes
//
// The exit codes are a public interface under semver, and the full contract is
// in docs/ARCHITECTURE.md. The command tree at this milestone can produce two
// of them:
//
//	0  the command did what was asked
//	2  usage error: an unknown command, or a flag that would not parse
//
// The remaining codes describe the outcome of a run, so they arrive with the
// pipeline that produces one.
package cli
