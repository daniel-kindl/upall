package provider

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/daniel-kindl/upall/internal/core"
	"github.com/daniel-kindl/upall/internal/exec"
)

// JSONConfig configures [ParserJSON].
//
//	[plan.json]
//	items     = "packages"
//	id        = "Repository"
//	available = "Tag"
//
// The field values are object keys rather than column headings, and are matched
// exactly.
type JSONConfig struct {
	// Items is a dotted path to the array of items inside each decoded value,
	// such as "result.packages". Leave it empty when the items are at the top
	// level, which covers both a document that is itself an array and the
	// one-object-per-line form docker writes.
	Items string `toml:"items"`

	// Fields names the object key each update field is read from.
	Fields
}

// jsonParser reads structured output.
//
// It decodes a stream of values rather than one document, which is what makes it
// read JSON Lines — one complete object per line, no enclosing array, which is
// what `docker image ls --format json` emits — with no separate mode for it. A
// single document is a stream of one.
type jsonParser struct {
	items  []string
	fields Fields
}

// newJSONParser validates cfg and returns the parser it describes.
func newJSONParser(cfg JSONConfig) (Parser, error) {
	if cfg.mapped() == 0 {
		return nil, fmt.Errorf("parser %q maps no keys; set at least one of name, id, installed, available", ParserJSON)
	}

	var items []string
	if cfg.Items != "" {
		items = strings.Split(cfg.Items, ".")
		for _, segment := range items {
			if segment == "" {
				return nil, fmt.Errorf("parser %q: items path %q has an empty segment", ParserJSON, cfg.Items)
			}
		}
	}

	return &jsonParser{items: items, fields: cfg.Fields}, nil
}

// Parse implements [Parser].
func (p *jsonParser) Parse(out exec.Output) ([]core.Update, error) {
	if out.Truncated {
		return nil, ErrTruncated
	}

	dec := json.NewDecoder(bytes.NewReader(out.Stdout))

	// Numbers stay as their literal text. A version read through float64 comes
	// back as 1.11 having been 1.110, or in exponent notation, and a version
	// upall reports has to be the one the tool printed.
	dec.UseNumber()

	var updates []core.Update
	for {
		var value any
		if err := dec.Decode(&value); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("parser %q: %w", ParserJSON, err)
		}

		items, err := p.locate(value)
		if err != nil {
			return nil, err
		}

		for _, item := range items {
			u, err := p.update(item)
			if err != nil {
				return nil, err
			}
			if u, ok := finish(u); ok {
				updates = append(updates, u)
			}
		}
	}

	return updates, nil
}

// locate walks to the items inside one decoded value.
//
// With no path configured, an array is the items and an object is a single item,
// which is the JSON Lines case. With a path, each segment indexes an object and
// the end of it must be an array.
func (p *jsonParser) locate(value any) ([]any, error) {
	for i, segment := range p.items {
		object, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("parser %q: items path %q: %q is not inside an object",
				ParserJSON, strings.Join(p.items, "."), strings.Join(p.items[:i+1], "."))
		}
		next, found := object[segment]
		if !found {
			// A path that is absent from this value is not a failure. A tool
			// with nothing to report can omit the array entirely, and that is
			// the no-updates case rather than a malformed document.
			return nil, nil
		}
		value = next
	}

	switch v := value.(type) {
	case []any:
		return v, nil
	case map[string]any:
		if len(p.items) > 0 {
			return nil, fmt.Errorf("parser %q: items path %q holds an object, not an array",
				ParserJSON, strings.Join(p.items, "."))
		}
		return []any{v}, nil
	case nil:
		return nil, nil
	default:
		return nil, fmt.Errorf("parser %q: expected an array or an object, found %T", ParserJSON, value)
	}
}

// update reads one item into a [core.Update].
func (p *jsonParser) update(item any) (core.Update, error) {
	object, ok := item.(map[string]any)
	if !ok {
		return core.Update{}, fmt.Errorf("parser %q: an item is %T, not an object", ParserJSON, item)
	}

	var u core.Update
	for field, key := range map[string]string{
		"name":      p.fields.Name,
		"id":        p.fields.ID,
		"installed": p.fields.Installed,
		"available": p.fields.Available,
	} {
		if key == "" {
			continue
		}
		text, err := scalar(object[key], key)
		if err != nil {
			return core.Update{}, err
		}
		set(&u, field, text)
	}

	return u, nil
}

// scalar renders a JSON value as the string an update field holds.
//
// A key that is absent or null is the empty string, because [core.Update]
// documents its fields as optional and a tool omitting one is saying it does not
// know. An array or object is an error instead: rendering one would put
// "map[...]" in a version column, which is a lie that survives all the way to
// the user rather than a failure anyone can act on.
func scalar(value any, key string) (string, error) {
	switch v := value.(type) {
	case nil:
		return "", nil
	case string:
		return v, nil
	case json.Number:
		return v.String(), nil
	case bool:
		return strconv.FormatBool(v), nil
	default:
		return "", fmt.Errorf("parser %q: key %q holds %T, which is not a value an update field can carry",
			ParserJSON, key, value)
	}
}
