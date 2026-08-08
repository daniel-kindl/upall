package provider

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/daniel-kindl/upall/internal/core"
	"github.com/daniel-kindl/upall/internal/exec"
)

// fixture reads captured tool output from testdata.
//
// The bytes are used exactly as they were captured, line endings included.
// .gitattributes keeps git from normalizing them, and reading them raw here is
// the other half of that: winget's CRLF is part of what these tests cover.
func fixture(t *testing.T, name string) exec.Output {
	t.Helper()

	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("reading the fixture: %v", err)
	}
	return exec.Output{Stdout: b}
}

// wingetFields is the mapping the winget manifest will use at M4's provider PR,
// kept here so the parser is tested against the configuration it will really be
// given rather than one invented for the test.
var wingetFields = Fields{
	Name:      "Name",
	ID:        "Id",
	Installed: "Version",
	Available: "Available",
}

// aptPattern is the mapping the apt manifest will use, for the same reason.
const aptPattern = `^(?P<id>[^/\s]+)/\S+ (?P<available>\S+) \S+ \[upgradable from: (?P<installed>[^\]]+)\]`

func mustParser(t *testing.T, name string, cfg ParserConfig) Parser {
	t.Helper()

	p, err := NewParser(name, cfg)
	if err != nil {
		t.Fatalf("building the %s parser: %v", name, err)
	}
	return p
}

// TestTableParsesRealWingetOutput reads output captured from winget 1.29.280 on
// Windows.
//
// The fixture earns its place on two details that a hand-written one would have
// smoothed over. "Epic Online Services" is followed by exactly one space before
// the next column, because winget pads Name to the longest name in the table, so
// anything splitting on whitespace reports a package called "Epic" — this is the
// case that decides the parser reads column offsets. And the table ends with
// "2 upgrades available.", a trailer that slicing at those offsets reads as a
// package unless something rejects it.
func TestTableParsesRealWingetOutput(t *testing.T) {
	p := mustParser(t, ParserTable, ParserConfig{Table: &TableConfig{Fields: wingetFields}})

	got, err := p.Parse(fixture(t, "winget-upgrade.txt"))
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}

	want := []core.Update{
		{
			Name:      "Epic Online Services",
			ID:        "EpicGames.EpicOnlineServices",
			Installed: "4.1.0",
			Available: "4.3.1",
		},
		{
			Name:      "Logitech G HUB",
			ID:        "Logitech.GHUB",
			Installed: "2025.9.814157",
			Available: "2026.4.919028",
		},
	}

	if !slices.Equal(got, want) {
		t.Errorf("parsed the table as\n%+v\nwant\n%+v", got, want)
	}
}

// TestTableOnOutputWithNoTable covers what winget prints when it has nothing to
// say, which is a sentence rather than an empty table.
//
// Nothing to update is an empty result and no error. A parser that failed here
// would turn the ordinary case of an up-to-date machine into a failed run.
func TestTableOnOutputWithNoTable(t *testing.T) {
	p := mustParser(t, ParserTable, ParserConfig{Table: &TableConfig{Fields: wingetFields}})

	got, err := p.Parse(fixture(t, "winget-no-updates.txt"))
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("parsed %d updates from output with no table: %+v", len(got), got)
	}
}

// TestLinesParsesRealAptOutput reads output captured from apt 2.4.11 inside
// ubuntu:jammy-20240111, a frozen image with genuinely outdated packages.
func TestLinesParsesRealAptOutput(t *testing.T) {
	p := mustParser(t, ParserLines, ParserConfig{Lines: &LinesConfig{Pattern: aptPattern}})

	got, err := p.Parse(fixture(t, "apt-upgradable.txt"))
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}

	if len(got) < 20 {
		t.Fatalf("parsed %d updates, want the whole list; the pattern or the fixture is wrong", len(got))
	}

	// The preamble apt prints before the list. An unanchored pattern would
	// report it as a package, so it is checked for by name.
	for _, u := range got {
		if u.ID == "Listing..." || u.Name == "Listing..." {
			t.Error("parsed apt's preamble as a package")
		}
	}

	// The first entry, spelled out. apt gives one name per package, so Name
	// falls back to ID.
	want := core.Update{Name: "apt", ID: "apt", Installed: "2.4.11", Available: "2.4.14"}
	if got[0] != want {
		t.Errorf("first update is %+v, want %+v", got[0], want)
	}

	// A package whose suite field carries two values, which is where a pattern
	// that assumed one would stop matching.
	i := slices.IndexFunc(got, func(u core.Update) bool { return u.ID == "bash" })
	if i < 0 {
		t.Fatal("bash is in the fixture but was not parsed; its two-suite line did not match")
	}
	want = core.Update{Name: "bash", ID: "bash", Installed: "5.1-6ubuntu1", Available: "5.1-6ubuntu1.1"}
	if got[i] != want {
		t.Errorf("bash parsed as %+v, want %+v", got[i], want)
	}
}

// TestLinesOnOutputWithNoItems covers apt on a machine with nothing to upgrade,
// captured from ubuntu:22.04, which the official image rebuilds with its updates
// already applied. The output is the preamble and nothing else.
func TestLinesOnOutputWithNoItems(t *testing.T) {
	p := mustParser(t, ParserLines, ParserConfig{Lines: &LinesConfig{Pattern: aptPattern}})

	got, err := p.Parse(fixture(t, "apt-no-updates.txt"))
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("parsed %d updates from output with no items: %+v", len(got), got)
	}
}

// TestJSONParsesRealDockerOutput reads `docker image ls --format json`, which is
// JSON Lines: one complete object per line with no enclosing array.
//
// That shape is the reason the parser decodes a stream rather than a document,
// so a fixture in the other shape would not have exercised the decision.
func TestJSONParsesRealDockerOutput(t *testing.T) {
	p := mustParser(t, ParserJSON, ParserConfig{JSON: &JSONConfig{
		Fields: Fields{Name: "Repository", ID: "ID", Installed: "Tag"},
	}})

	got, err := p.Parse(fixture(t, "docker-images.json"))
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}

	want := []core.Update{
		{Name: "ubuntu", ID: "3b06811b2afd", Installed: "22.04"},
		{Name: "debian", ID: "171478fbe347", Installed: "bullseye-20240211"},
		{Name: "ubuntu", ID: "e6173d4dc55e", Installed: "jammy-20240111"},
	}

	if !slices.Equal(got, want) {
		t.Errorf("parsed the stream as\n%+v\nwant\n%+v", got, want)
	}
}

// TestJSONShapes covers the document shapes the same parser has to read, since
// the fixture only proves one of them.
func TestJSONShapes(t *testing.T) {
	fields := Fields{ID: "id", Available: "version"}

	tests := []struct {
		name  string
		items string
		in    string
		want  []core.Update
	}{
		{
			name: "a top-level array",
			in:   `[{"id":"a","version":"2"},{"id":"b","version":"3"}]`,
			want: []core.Update{{Name: "a", ID: "a", Available: "2"}, {Name: "b", ID: "b", Available: "3"}},
		},
		{
			name:  "an array under a path",
			items: "result.packages",
			in:    `{"result":{"packages":[{"id":"a","version":"2"}]}}`,
			want:  []core.Update{{Name: "a", ID: "a", Available: "2"}},
		},
		{
			name:  "a path the tool omitted, which is no updates",
			items: "packages",
			in:    `{"other":1}`,
			want:  nil,
		},
		{
			name: "an empty array, which is no updates",
			in:   `[]`,
			want: nil,
		},
		{
			name: "a number, kept as it was written",
			in:   `[{"id":"a","version":1.110}]`,
			want: []core.Update{{Name: "a", ID: "a", Available: "1.110"}},
		},
		{
			name: "a key the tool omitted, which is an empty field",
			in:   `[{"id":"a"}]`,
			want: []core.Update{{Name: "a", ID: "a"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := mustParser(t, ParserJSON, ParserConfig{JSON: &JSONConfig{Items: tt.items, Fields: fields}})

			got, err := p.Parse(exec.Output{Stdout: []byte(tt.in)})
			if err != nil {
				t.Fatalf("parsing: %v", err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("parsed %+v, want %+v", got, tt.want)
			}
		})
	}
}

// TestJSONRejectsUnreadableValues covers the cases where being quiet would put
// something untrue in front of the user.
func TestJSONRejectsUnreadableValues(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{name: "a nested object where a version belongs", in: `[{"id":"a","version":{"major":2}}]`},
		{name: "an array where a version belongs", in: `[{"id":"a","version":[2]}]`},
		{name: "an item that is not an object", in: `["a"]`},
		{name: "output that is not JSON at all", in: `error: could not connect`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := mustParser(t, ParserJSON, ParserConfig{JSON: &JSONConfig{
				Fields: Fields{ID: "id", Available: "version"},
			}})

			if _, err := p.Parse(exec.Output{Stdout: []byte(tt.in)}); err == nil {
				t.Error("parsed without an error; a misread value reaches the user as fact")
			}
		})
	}
}

// TestEveryParserRejectsTruncatedOutput is the guard against a run reporting on
// part of a machine as though it were all of it.
func TestEveryParserRejectsTruncatedOutput(t *testing.T) {
	parsers := map[string]ParserConfig{
		ParserTable: {Table: &TableConfig{Fields: wingetFields}},
		ParserLines: {Lines: &LinesConfig{Pattern: aptPattern}},
		ParserJSON:  {JSON: &JSONConfig{Fields: Fields{ID: "id"}}},
	}

	for name, cfg := range parsers {
		t.Run(name, func(t *testing.T) {
			p := mustParser(t, name, cfg)

			out := fixture(t, "winget-upgrade.txt")
			out.Truncated = true

			_, err := p.Parse(out)
			if !errors.Is(err, ErrTruncated) {
				t.Errorf("parsing truncated output returned %v, want ErrTruncated", err)
			}
		})
	}
}

// TestNewParserRejectsBadConfiguration checks that a manifest which would parse
// nothing fails at load rather than reporting an up-to-date machine.
func TestNewParserRejectsBadConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		parser string
		cfg    ParserConfig
	}{
		{
			name:   "an unknown parser name",
			parser: "yaml",
			cfg:    ParserConfig{},
		},
		{
			name:   "a table with no block",
			parser: ParserTable,
			cfg:    ParserConfig{},
		},
		{
			name:   "a table mapping no columns",
			parser: ParserTable,
			cfg:    ParserConfig{Table: &TableConfig{}},
		},
		{
			name:   "lines with no block",
			parser: ParserLines,
			cfg:    ParserConfig{},
		},
		{
			name:   "lines with no pattern",
			parser: ParserLines,
			cfg:    ParserConfig{Lines: &LinesConfig{}},
		},
		{
			name:   "a pattern that does not compile",
			parser: ParserLines,
			cfg:    ParserConfig{Lines: &LinesConfig{Pattern: `(?P<id>`}},
		},
		{
			name:   "a pattern capturing a field that does not exist",
			parser: ParserLines,
			cfg:    ParserConfig{Lines: &LinesConfig{Pattern: `(?P<version>\S+)`}},
		},
		{
			name:   "a pattern with no named groups",
			parser: ParserLines,
			cfg:    ParserConfig{Lines: &LinesConfig{Pattern: `(\S+)`}},
		},
		{
			name:   "json with no block",
			parser: ParserJSON,
			cfg:    ParserConfig{},
		},
		{
			name:   "json mapping no keys",
			parser: ParserJSON,
			cfg:    ParserConfig{JSON: &JSONConfig{}},
		},
		{
			name:   "an items path with an empty segment",
			parser: ParserJSON,
			cfg:    ParserConfig{JSON: &JSONConfig{Items: "a..b", Fields: Fields{ID: "id"}}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewParser(tt.parser, tt.cfg); err == nil {
				t.Error("built a parser that cannot work; the manifest would report no updates on a machine that had them")
			}
		})
	}
}

// TestUnknownParserNamesTheKnownOnes keeps the error useful. A manifest author
// who typed the wrong name should be told the right ones rather than sent to the
// documentation.
func TestUnknownParserNamesTheKnownOnes(t *testing.T) {
	_, err := NewParser("yaml", ParserConfig{})
	if err == nil {
		t.Fatal("an unknown parser name was accepted")
	}

	for _, name := range ParserNames() {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("the error for an unknown parser does not mention %q: %v", name, err)
		}
	}
}
