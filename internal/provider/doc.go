// Package provider turns the tools on a machine into things upall can run.
//
// A provider is one thing that can update software, and the interface it
// satisfies is [github.com/daniel-kindl/upall/internal/core.Provider]. There are
// two kinds, per ADR-0002: most are a TOML manifest describing commands and a
// named parser, and a few need real code and live in internal/provider/native.
// Nothing downstream can tell them apart.
//
// # The registry
//
// [Registry] is the set of providers a run can draw on, and the only thing that
// knows they all exist. It answers the two questions asked before a run does any
// work: [Registry.Lookup] resolves an ID, and [Registry.For] filters to the
// providers that could run on this platform.
//
// Both are cheap and neither touches the machine. Filtering asks what a provider
// declared, not what is installed — that is
// [github.com/daniel-kindl/upall/internal/core.Provider].Detect's question, one
// subprocess later — so a provider surviving the filter and then reporting
// itself absent is the ordinary case.
//
// Providers come out ordered by ID, and that is a contract rather than an
// implementation detail. Registration order is whatever the embedded manifests
// were walked in, so depending on it would let renaming a file reorder every
// plan. It is not an apply order: scheduling apply is internal/pipeline's, under
// the concurrency bound documented there.
//
// An ID is lowercase letters, digits, and internal hyphens, and [Registry.Add]
// refuses anything else. The set is narrower than it needs to be on purpose,
// because an ID is a TOML key, an --only value, and a JSON output field, all
// public under semver — one that shipped with a capital or a space would have to
// keep it. A duplicate ID is refused for a different reason: two providers
// answering to one name means --only picks whichever won a race.
//
// # The parser catalogue
//
// A [Parser] turns what a command printed into
// [github.com/daniel-kindl/upall/internal/core.Update] values. Parsers are named
// in a manifest and configured beside the name, so one parser serves many
// providers rather than each tool getting a function of its own:
//
//	[plan]
//	command = ["winget", "upgrade"]
//	parser  = "table"
//
//	  [plan.table]
//	  name      = "Name"
//	  id        = "Id"
//	  installed = "Version"
//	  available = "Available"
//
// [NewParser] resolves a name against the catalogue, which is [ParserTable],
// [ParserLines], and [ParserJSON]. The names are part of the manifest schema and
// so are public under semver.
//
// Choosing between them is usually decided by the tool. Reach for [ParserTable]
// when the output is columns under a heading, which is what a tool prints for a
// human to read. Reach for [ParserJSON] when it offers structured output, which
// is worth asking for wherever it exists because it cannot be misaligned.
// [ParserLines] is the rest, and most of the rest: a regexp per line, which is
// how apt, pacman, and anything else that prints one record per line is read.
//
// # What every parser agrees on
//
// No updates available is an empty slice and a nil error. A tool with nothing to
// report usually prints prose rather than an empty table, so a parser that finds
// nothing to read reports nothing to update. This is the same principle as a
// provider that is not installed not being an error, and it is the common case
// rather than a degraded one.
//
// Truncated output is [ErrTruncated]. A parser handed the first half of a table
// would report on the packages it happened to see as though they were all of
// them, which is worse than failing, because nothing in the output says anything
// is missing.
//
// A parsed update with no Name takes its ID. Most tools have exactly one name
// for a package, and apt's "libc6" is both the identifier and what a human calls
// it; the alternative is a plan rendered as blank rows. An update that came out
// entirely empty is dropped rather than reported.
//
// # Fixtures
//
// Every parser is tested against output captured from the real tool, in
// testdata. This is required rather than encouraged: a shared catalogue means a
// change made for one provider can break another that nobody touched, and
// ADR-0002 names fixture tests as the mitigation. Fixtures keep the line endings
// the tool wrote, which is why .gitattributes exempts them from normalization —
// winget writes CRLF and apt writes LF, and a parser has to read both.
package provider
