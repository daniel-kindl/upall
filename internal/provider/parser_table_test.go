package provider

import (
	"slices"
	"testing"

	"github.com/daniel-kindl/upall/internal/core"
	"github.com/daniel-kindl/upall/internal/exec"
)

// TestTableAlignsByDisplayWidth is why this package depends on go-runewidth.
//
// A tool aligns its table by how wide a character prints. A package name in CJK
// is one rune, two display columns, and three bytes in UTF-8, so all three ways
// of counting disagree and only the display one lands where the tool put the
// next column. The table below is padded the way a renderer pads it: every
// column starts at a fixed display offset.
//
// Parsing this with byte offsets shifts every field on the row after the wide
// one; parsing it with rune counts shifts them the other way. Both produce
// versions with fragments of an identifier in them rather than an obvious
// failure, which is what makes this worth a test and a dependency.
func TestTableAlignsByDisplayWidth(t *testing.T) {
	//              0               16              32        42
	const table = "Name            Id              Version   Available\n" +
		"日本語パッケー  Example.Jp      1.0       2.0\n" +
		"Plain           Example.Plain   3.0       4.0\n"

	p := mustParser(t, ParserTable, ParserConfig{Table: &TableConfig{Fields: wingetFields}})

	got, err := p.Parse(exec.Output{Stdout: []byte(table)})
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}

	want := []core.Update{
		{Name: "日本語パッケー", ID: "Example.Jp", Installed: "1.0", Available: "2.0"},
		{Name: "Plain", ID: "Example.Plain", Installed: "3.0", Available: "4.0"},
	}

	if !slices.Equal(got, want) {
		t.Errorf("parsed\n%+v\nwant\n%+v", got, want)
	}
}

// TestTableFindsTheHeaderBelowPreamble covers output where the table is not the
// first thing printed, which is most of it: tools announce what they are doing,
// warn about their own CLI, or print a blank line first.
//
// Taking the first line as the header would read "Listing..." as the headings and
// then find no columns at all.
func TestTableFindsTheHeaderBelowPreamble(t *testing.T) {
	const table = "Checking for updates...\n" +
		"\n" +
		"Name       Id          Version   Available\n" +
		"---------------------------------------------\n" +
		"Firefox    Mozilla.FF  1.0       2.0\n"

	p := mustParser(t, ParserTable, ParserConfig{Table: &TableConfig{Fields: wingetFields}})

	got, err := p.Parse(exec.Output{Stdout: []byte(table)})
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}

	want := []core.Update{{Name: "Firefox", ID: "Mozilla.FF", Installed: "1.0", Available: "2.0"}}
	if !slices.Equal(got, want) {
		t.Errorf("parsed\n%+v\nwant\n%+v", got, want)
	}
}

// TestTableHeadingsMayContainASpace checks the two-space rule that separates
// headings from each other. "Installed Version" is one column, not two.
func TestTableHeadingsMayContainASpace(t *testing.T) {
	const table = "Package Name    Installed Version   Latest Version\n" +
		"firefox         1.0                 2.0\n"

	p := mustParser(t, ParserTable, ParserConfig{Table: &TableConfig{Fields: Fields{
		Name:      "Package Name",
		Installed: "Installed Version",
		Available: "Latest Version",
	}}})

	got, err := p.Parse(exec.Output{Stdout: []byte(table)})
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}

	want := []core.Update{{Name: "firefox", Installed: "1.0", Available: "2.0"}}
	if !slices.Equal(got, want) {
		t.Errorf("parsed\n%+v\nwant\n%+v", got, want)
	}
}

// TestTableIgnoresATableItDoesNotRecognise covers a manifest naming a column the
// tool does not print, which is what a renamed column looks like after an
// upgrade of the tool.
//
// The result is no updates rather than updates with an empty field. Reporting
// nothing is wrong, but it is visibly wrong; reporting a plan with a blank
// version column looks like the tool's own answer.
func TestTableIgnoresATableItDoesNotRecognise(t *testing.T) {
	const table = "Name       Id          Version   Newest\n" +
		"Firefox    Mozilla.FF  1.0       2.0\n"

	// wingetFields asks for "Available", which this table calls "Newest".
	p := mustParser(t, ParserTable, ParserConfig{Table: &TableConfig{Fields: wingetFields}})

	got, err := p.Parse(exec.Output{Stdout: []byte(table)})
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("parsed %d updates from a table missing a configured column: %+v", len(got), got)
	}
}

// TestTableSkipsTrailers is the rule stated on isRow, checked against trailers
// other tools write. The winget fixture covers the real one; these cover the
// shape rather than the instance.
//
// The narrow table matters here. winget's Name column is as wide as its longest
// package name, so its own trailer happens to fall inside that one column; under
// a narrower table the same sentence spills across two and is read as a package
// with a version. That is why the test is geometry rather than "did two fields
// come out non-empty".
func TestTableSkipsTrailers(t *testing.T) {
	trailers := []string{
		"2 upgrades available.",
		"Done",
		"3 packages have pins.",
	}

	for _, trailer := range trailers {
		t.Run(trailer, func(t *testing.T) {
			table := "Name       Id          Version   Available\n" +
				"Firefox    Mozilla.FF  1.0       2.0\n" +
				trailer + "\n"

			p := mustParser(t, ParserTable, ParserConfig{Table: &TableConfig{Fields: wingetFields}})

			got, err := p.Parse(exec.Output{Stdout: []byte(table)})
			if err != nil {
				t.Fatalf("parsing: %v", err)
			}

			want := []core.Update{{Name: "Firefox", ID: "Mozilla.FF", Installed: "1.0", Available: "2.0"}}
			if !slices.Equal(got, want) {
				t.Errorf("parsed\n%+v\nwant\n%+v", got, want)
			}
		})
	}
}

// TestTableAcceptsEitherLineEnding checks the one difference between the two
// platforms' captured output. winget writes CRLF and apt writes LF, and a
// carriage return left on the end of a row becomes part of the last column's
// value, so a version reads as "2.0\r" and never equals the one the journal
// recorded.
func TestTableAcceptsEitherLineEnding(t *testing.T) {
	rows := []string{
		"Name       Id          Version   Available",
		"Firefox    Mozilla.FF  1.0       2.0",
	}
	want := []core.Update{{Name: "Firefox", ID: "Mozilla.FF", Installed: "1.0", Available: "2.0"}}

	for _, ending := range []struct{ name, sep string }{
		{name: "LF", sep: "\n"},
		{name: "CRLF", sep: "\r\n"},
	} {
		t.Run(ending.name, func(t *testing.T) {
			p := mustParser(t, ParserTable, ParserConfig{Table: &TableConfig{Fields: wingetFields}})

			table := rows[0] + ending.sep + rows[1] + ending.sep
			got, err := p.Parse(exec.Output{Stdout: []byte(table)})
			if err != nil {
				t.Fatalf("parsing: %v", err)
			}
			if !slices.Equal(got, want) {
				t.Errorf("parsed\n%+v\nwant\n%+v", got, want)
			}
		})
	}
}
