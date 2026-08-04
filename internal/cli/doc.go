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
// The exit codes are a public interface under semver. They are declared and
// documented in internal/core, which is also where Result.ExitCode derives the
// ones describing a run. This package chooses among them; it does not define
// them, because a contract kept in two places is a contract free to drift.
//
// The command tree at this milestone can produce two: core.ExitOK when the
// command did what was asked, and core.ExitUsage for an unknown command or a
// flag that would not parse. The rest describe the outcome of a run, so they
// arrive with the pipeline that produces one.
package cli
