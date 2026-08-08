// Package native holds the providers that a manifest cannot express.
//
// A provider is one thing that can update software, and ADR-0002 says there are
// two kinds of them. Most are a TOML manifest in internal/provider/manifests:
// commands to run and a named parser to read them with, no Go at all. The ones
// here are the exceptions, and they implement
// [github.com/daniel-kindl/upall/internal/core.Provider] directly, the same
// interface the manifest adapter satisfies. Nothing downstream can tell which it
// has — the registry, the pipeline, the config layer, and both frontends see one
// kind of thing.
//
// # When to write one
//
// Almost never, and the bar is deliberately awkward:
//
//	Write a manifest. Reach for a native provider only when the work needs an
//	API, a non-textual protocol, or logic no parser can express.
//
// The pressure runs the other way, so the case worth naming is the provider that
// is *almost* expressible and tempts someone to add one field to the manifest
// schema to close the gap. Adding that field is how a schema becomes a
// programming language with no debugger, no type checking, and no tests. Write
// it here instead, where it is ordinary Go and can be tested like anything else.
//
// Each native provider's own doc.go states, in as many words, why a manifest
// could not express it. That is a requirement rather than a courtesy: it is the
// record that the bar was cleared, and it is what a future reader checks when
// the underlying tool has grown the JSON output that would have made a manifest
// work all along.
//
// # What is here
//
// Nothing yet. The two that are coming are known:
//
//   - Windows Update, at M8. It is a COM API, so there is no command to run and
//     no output to parse. It also has to report a pending reboot, which no
//     package manager's output has a place for.
//   - Docker and Compose, at M9. The socket speaks JSON, and comparing image
//     digests against the tag in use, then recreating only the services whose
//     image actually changed, is a decision rather than a parse.
//
// # The rules still apply
//
// A native provider is below internal/cli, so it may not write to a terminal,
// prompt, or check for one. Every subprocess it starts goes through
// internal/exec, with the runner injected, so no test invokes a real package
// manager. Platform-specific code goes behind a build tag rather than a
// runtime.GOOS branch.
package native
