package provider

import (
	"errors"
	"fmt"
	"strings"

	"github.com/daniel-kindl/upall/internal/core"
	"github.com/daniel-kindl/upall/internal/exec"
)

// ErrTruncated reports that the command produced more output than
// [exec.MaxCapture] and some of it was dropped before a parser saw it.
//
// It is an error rather than a shorter plan. A parser handed the first half of a
// table would report on the packages it happened to see as though they were all
// of them, and the run would then update those and call itself a success. That
// is a tool describing a machine state it did not look at, which is worse than a
// failure, because nothing about the output says anything is missing.
var ErrTruncated = errors.New("captured output was truncated")

// Parser turns what a command printed into the updates it describes.
//
// A parser is named in a manifest and configured by a block beside the name, so
// one parser serves many providers: [ParserTable] reads winget's output and
// scoop's, [ParserLines] reads apt's and pacman's. That reuse is the point of
// having a catalogue rather than a function per tool, and it is also the
// catalogue's main hazard, since a change here can break a provider nobody
// touched. Fixture tests against captured real output are the mitigation, and
// ADR-0002 requires them.
//
// Implementations are stateless once built, and one may be shared across
// goroutines.
type Parser interface {
	// Parse reads out.Stdout and returns what it says is out of date.
	//
	// No updates available is an empty slice and a nil error, and it is the
	// common case rather than a degenerate one: most tools print prose, or
	// nothing at all, instead of an empty table. A parser that cannot find
	// anything to read therefore reports nothing to update, not a failure.
	//
	// Output that was truncated is an error. See [ErrTruncated].
	Parse(out exec.Output) ([]core.Update, error)
}

// The parsers in the catalogue. These are the values a manifest's parser field
// may take, and they are part of the manifest schema, which is public under
// semver: renaming one is a breaking change.
const (
	// ParserTable reads columns aligned under a header row, which is what a
	// tool prints when it expects a human to read it. See [TableConfig].
	ParserTable = "table"

	// ParserLines reads one item per line with a regexp. See [LinesConfig].
	ParserLines = "lines"

	// ParserJSON reads a JSON document or a stream of them. See [JSONConfig].
	ParserJSON = "json"
)

// ParserNames returns the catalogue's names in the order above.
//
// It exists so that an error about an unknown parser can list the known ones
// rather than send the reader to the documentation, and so that there is one
// place to add a name to when the catalogue grows.
func ParserNames() []string {
	return []string{ParserTable, ParserLines, ParserJSON}
}

// Fields says where a parser finds each field of a [core.Update].
//
// What the values mean depends on the parser: they are column headings for
// [ParserTable] and object keys for [ParserJSON]. An empty value means the tool
// does not report that field, which is normal — [core.Update] documents every
// field but Name as optional, because package managers disagree about how much
// they will tell you.
type Fields struct {
	// Name is the human-readable name, such as "Mozilla Firefox".
	//
	// A parser that produces no Name copies ID into it, because most tools have
	// only one name for a package and apt's "libc6" is both. Leaving Name empty
	// instead would render a plan of blank rows.
	Name string `toml:"name"`

	// ID is what the tool calls the package, such as "Mozilla.Firefox".
	ID string `toml:"id"`

	// Installed is the version on the machine now.
	Installed string `toml:"installed"`

	// Available is the version that would replace it.
	Available string `toml:"available"`
}

// mapped reports how many fields were given a source, which decides how much of
// a row has to be present before it counts as one. See [TableConfig].
func (f Fields) mapped() int {
	n := 0
	for _, v := range []string{f.Name, f.ID, f.Installed, f.Available} {
		if v != "" {
			n++
		}
	}
	return n
}

// ParserConfig is the configuration a manifest supplies for its parser.
//
// The block matching the chosen parser is the one that is read; the others are
// nil and ignored. It is one struct rather than three separate ones so that the
// manifest decodes into a single value and [NewParser] is the only place that
// knows which block belongs to which name.
type ParserConfig struct {
	// Table configures [ParserTable].
	Table *TableConfig `toml:"table"`

	// Lines configures [ParserLines].
	Lines *LinesConfig `toml:"lines"`

	// JSON configures [ParserJSON].
	JSON *JSONConfig `toml:"json"`
}

// NewParser returns the parser called name, configured by cfg.
//
// This is the whole catalogue. A manifest names a parser and this resolves it,
// so an unknown name fails here, once, with the known names in the message,
// rather than in a switch that each caller writes for itself.
//
// The configuration block must be the one the name calls for. A missing block is
// an error rather than a default, because a parser with no configuration has
// nothing to map onto a [core.Update] and would report an empty plan on a machine
// that had updates waiting — a silence indistinguishable from good news.
func NewParser(name string, cfg ParserConfig) (Parser, error) {
	switch name {
	case ParserTable:
		if cfg.Table == nil {
			return nil, fmt.Errorf("parser %q needs a [table] configuration block", name)
		}
		return newTableParser(*cfg.Table)
	case ParserLines:
		if cfg.Lines == nil {
			return nil, fmt.Errorf("parser %q needs a [lines] configuration block", name)
		}
		return newLinesParser(*cfg.Lines)
	case ParserJSON:
		if cfg.JSON == nil {
			return nil, fmt.Errorf("parser %q needs a [json] configuration block", name)
		}
		return newJSONParser(*cfg.JSON)
	default:
		return nil, fmt.Errorf("unknown parser %q; known parsers are %s",
			name, strings.Join(ParserNames(), ", "))
	}
}

// splitLines splits captured output into lines, accepting either line ending.
//
// Both are real: winget writes CRLF and apt writes LF, and upall reads each on
// the platform the other is absent from only when a fixture test does it. The
// carriage return is dropped rather than trimmed later so that no parser has to
// remember it exists.
//
// A trailing newline does not produce a final empty line.
func splitLines(b []byte) []string {
	text := strings.ReplaceAll(string(b), "\r\n", "\n")
	text = strings.TrimSuffix(text, "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

// finish applies the rules every parser shares to one row's worth of fields and
// reports whether it is an update at all.
//
// Two things happen here rather than three times over. Name falls back to ID,
// for the reason on [Fields.Name]. A row that produced nothing at all is
// dropped, because a parser that emits blank updates turns a misread table into
// a plan full of empty rows instead of an obvious failure.
func finish(u core.Update) (core.Update, bool) {
	if u.Name == "" {
		u.Name = u.ID
	}
	if u.Name == "" && u.ID == "" && u.Installed == "" && u.Available == "" {
		return core.Update{}, false
	}
	return u, true
}
