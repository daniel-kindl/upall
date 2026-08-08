package provider

import (
	"fmt"
	"strings"

	"github.com/mattn/go-runewidth"

	"github.com/daniel-kindl/upall/internal/core"
	"github.com/daniel-kindl/upall/internal/exec"
)

// TableConfig configures [ParserTable]. Each field names the column heading its
// value sits under.
//
//	[plan.table]
//	name      = "Name"
//	id        = "Id"
//	installed = "Version"
//	available = "Available"
//
// Headings are matched exactly, including case, because a tool that renamed a
// column changed its output and the manifest should say so rather than guess.
type TableConfig struct {
	Fields
}

// tableParser reads columns aligned under a header row.
//
// The alignment is the whole mechanism: a column's contents begin at the display
// column its heading begins at, so the header line is what says where to cut
// every row below it. Splitting rows on whitespace instead would be simpler and
// wrong — winget pads its Name column to the longest name, so
// "Epic Online Services EpicGames.EpicOnlineServices" has exactly one space
// between two columns while "Epic Online Services" has two internal ones.
type tableParser struct {
	fields Fields
}

// newTableParser validates cfg and returns the parser it describes.
func newTableParser(cfg TableConfig) (Parser, error) {
	if cfg.mapped() == 0 {
		return nil, fmt.Errorf("parser %q maps no columns; set at least one of name, id, installed, available", ParserTable)
	}
	return &tableParser{fields: cfg.Fields}, nil
}

// column is one heading and where it sits, in display columns.
type column struct {
	title string
	start int
	end   int
}

// Parse implements [Parser].
func (p *tableParser) Parse(out exec.Output) ([]core.Update, error) {
	if out.Truncated {
		return nil, ErrTruncated
	}

	lines := splitLines(out.Stdout)

	header, columns, found := p.findHeader(lines)
	if !found {
		// No header means no table, which nearly always means no updates: a
		// tool with nothing to report prints prose rather than an empty grid.
		// winget says "No installed package found matching input criteria."
		// See [Parser.Parse] for why that is not an error.
		return nil, nil
	}

	// Where the table's final column begins. See isRow for what it decides.
	lastColumn := columns[len(columns)-1].start

	var updates []core.Update
	for _, line := range lines[header+1:] {
		if !isRow(line, lastColumn) {
			continue
		}

		u := core.Update{
			Name:      p.cut(line, columns, p.fields.Name),
			ID:        p.cut(line, columns, p.fields.ID),
			Installed: p.cut(line, columns, p.fields.Installed),
			Available: p.cut(line, columns, p.fields.Available),
		}

		if u, ok := finish(u); ok {
			updates = append(updates, u)
		}
	}

	return updates, nil
}

// findHeader returns the index of the header line and the columns it defines.
//
// The header is the first line that carries every heading the configuration
// asked for. Searching rather than taking the first line matters because tools
// print preamble above their tables, and apt's "Listing..." would otherwise be
// read as the headings.
func (p *tableParser) findHeader(lines []string) (int, []column, bool) {
	for i, line := range lines {
		columns := headerColumns(line)
		if len(columns) == 0 {
			continue
		}
		if p.complete(columns) {
			return i, columns, true
		}
	}
	return 0, nil, false
}

// complete reports whether columns carries every heading the configuration
// named.
func (p *tableParser) complete(columns []column) bool {
	for _, want := range []string{p.fields.Name, p.fields.ID, p.fields.Installed, p.fields.Available} {
		if want == "" {
			continue
		}
		if find(columns, want) == nil {
			return false
		}
	}
	return true
}

// cut returns the value under the heading title, or "" when the configuration
// did not ask for that field.
func (p *tableParser) cut(line string, columns []column, title string) string {
	if title == "" {
		return ""
	}
	c := find(columns, title)
	if c == nil {
		return ""
	}
	return sliceColumns(line, c.start, c.end)
}

// isRow reports whether a line is a row of the table, given where the table's
// last column begins.
//
// Three things below a header are not rows. Blank lines. Rules, meaning the line
// of dashes a tool draws under its headings, recognised by consisting of nothing
// but the characters a box is drawn from — no package name does.
//
// And trailers, which are the interesting one. winget ends its table with
// "2 upgrades available.", and slicing that at the header's offsets yields a
// package called "2 upgrades" at version "available." A trailer is prose: it
// starts at column zero and stops when the sentence does. A row is aligned, and
// alignment is exactly the property that makes it reach further — for the last
// column to line up down the table, every row must carry content or padding all
// the way to where that column starts. So the test is width, and the threshold
// is that column's offset.
//
// The known cost is a row whose last column is empty, on a tool that also trims
// the trailing spaces off it. Such a row stops short and is read as a trailer. A
// manifest hitting that should map a column the tool always fills, or use
// [ParserLines], which has no geometry to be confused by.
func isRow(line string, lastColumn int) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	if strings.Trim(trimmed, "-=+|_ ") == "" {
		return false
	}
	return runewidth.StringWidth(line) > lastColumn
}

// find returns the column with this heading, or nil.
func find(columns []column, title string) *column {
	for i := range columns {
		if columns[i].title == title {
			return &columns[i]
		}
	}
	return nil
}

// headerColumns reads a header line into the columns it defines.
//
// A column starts where its heading starts and runs until the next one starts.
// Headings are separated by two or more spaces, which is what lets a heading
// contain one: "Available Version" is a single column, and winget's
// "Name" and "Id" are two.
//
// Positions are display columns rather than byte or rune offsets. A tool aligns
// its table by how wide a character prints, so a package name containing CJK —
// two columns per rune, one rune, three bytes in UTF-8 — puts all three counts
// in disagreement, and only the display one lines up with what the tool did.
func headerColumns(line string) []column {
	var columns []column

	col, start, spaces := 0, -1, 0
	var title strings.Builder

	closeColumn := func() {
		if start < 0 {
			return
		}
		columns = append(columns, column{
			title: strings.TrimRight(title.String(), " "),
			start: start,
		})
		start = -1
		title.Reset()
	}

	for _, r := range line {
		switch r {
		case ' ':
			spaces++
			if spaces >= 2 {
				closeColumn()
			} else if start >= 0 {
				// One space might be inside a heading or might be the start of
				// a separator. It is kept and trimmed off if the next character
				// settles it the other way.
				title.WriteRune(r)
			}
		default:
			spaces = 0
			if start < 0 {
				start = col
			}
			title.WriteRune(r)
		}
		col += runewidth.RuneWidth(r)
	}
	closeColumn()

	// Each column runs to the start of the next. The last runs to the end of
	// whatever row it is applied to, which is not known here.
	for i := range columns {
		if i+1 < len(columns) {
			columns[i].end = columns[i+1].start
		} else {
			columns[i].end = -1
		}
	}
	return columns
}

// sliceColumns returns the text of line lying in display columns [start, end),
// trimmed. A negative end means the rest of the line.
//
// A rune is taken when it begins inside the range, so a wide character straddling
// the boundary belongs to the column it started in. That is the same rule the
// tool's own renderer had to use to align the table in the first place.
func sliceColumns(line string, start, end int) string {
	var b strings.Builder

	col := 0
	for _, r := range line {
		if end >= 0 && col >= end {
			break
		}
		if col >= start {
			b.WriteRune(r)
		}
		col += runewidth.RuneWidth(r)
	}
	return strings.TrimSpace(b.String())
}
