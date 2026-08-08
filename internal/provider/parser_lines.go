package provider

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/daniel-kindl/upall/internal/core"
	"github.com/daniel-kindl/upall/internal/exec"
)

// LinesConfig configures [ParserLines].
//
//	[plan.lines]
//	pattern = '^(?P<id>[^/\s]+)/\S+ (?P<available>\S+) \S+ \[upgradable from: (?P<installed>[^\]]+)\]'
//
// Use TOML's literal strings, quoted with a single ', for patterns. A basic
// string would need every backslash doubled.
type LinesConfig struct {
	// Pattern is a Go regexp applied to each line of output. Its named capture
	// groups fill a [core.Update], and the group names are the field names:
	// name, id, installed, available. Any other group name is an error, because
	// a typo would otherwise capture into nothing and produce a plan that is
	// quietly missing a column.
	//
	// A line the pattern does not match is skipped rather than being a failure.
	// That is what discards the preamble tools print — apt's "Listing..." — and
	// it is why the pattern should be anchored: an unanchored one that matches
	// the preamble reports it as a package.
	Pattern string `toml:"pattern"`
}

// linesParser reads one update per line with a regexp.
//
// It is the catalogue's escape hatch for output that is neither aligned nor
// structured, which is most of what package managers print when asked for a
// list. The regexp is in the manifest rather than here because the alternative
// is a parser per tool, and a regexp is data.
type linesParser struct {
	re *regexp.Regexp

	// groups maps a subexpression index onto the [core.Update] field it fills.
	// Indexed by subexpression rather than by name so that Parse does not look
	// names up per line.
	groups map[int]string
}

// updateFields are the group names a pattern may use, which are the fields of a
// [core.Update] that a parser fills.
var updateFields = []string{"name", "id", "installed", "available"}

// newLinesParser validates cfg and returns the parser it describes.
func newLinesParser(cfg LinesConfig) (Parser, error) {
	if strings.TrimSpace(cfg.Pattern) == "" {
		return nil, fmt.Errorf("parser %q needs a pattern", ParserLines)
	}

	re, err := regexp.Compile(cfg.Pattern)
	if err != nil {
		return nil, fmt.Errorf("parser %q: pattern does not compile: %w", ParserLines, err)
	}

	groups := make(map[int]string)
	for i, name := range re.SubexpNames() {
		if name == "" {
			continue
		}
		if !slices.Contains(updateFields, name) {
			return nil, fmt.Errorf("parser %q: pattern captures %q, which is not an update field; use one of %s",
				ParserLines, name, strings.Join(updateFields, ", "))
		}
		groups[i] = name
	}

	if len(groups) == 0 {
		return nil, fmt.Errorf("parser %q: pattern has no named capture groups, so it fills nothing; name one of %s",
			ParserLines, strings.Join(updateFields, ", "))
	}

	return &linesParser{re: re, groups: groups}, nil
}

// Parse implements [Parser].
func (p *linesParser) Parse(out exec.Output) ([]core.Update, error) {
	if out.Truncated {
		return nil, ErrTruncated
	}

	var updates []core.Update
	for _, line := range splitLines(out.Stdout) {
		match := p.re.FindStringSubmatch(line)
		if match == nil {
			continue
		}

		var u core.Update
		for i, field := range p.groups {
			set(&u, field, match[i])
		}
		if u, ok := finish(u); ok {
			updates = append(updates, u)
		}
	}

	return updates, nil
}

// set writes value into the named field of u. The name has already been checked
// against updateFields, so an unknown one here is unreachable.
func set(u *core.Update, field, value string) {
	switch field {
	case "name":
		u.Name = value
	case "id":
		u.ID = value
	case "installed":
		u.Installed = value
	case "available":
		u.Available = value
	}
}
