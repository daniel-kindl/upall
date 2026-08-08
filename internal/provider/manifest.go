package provider

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/daniel-kindl/upall/internal/core"
)

// Manifest is a provider described as data rather than as code.
//
// It is the common case and the one to reach for first: adding a typical
// provider should be a TOML file and a fixture test, with no new Go and nothing
// to review beyond the commands themselves. ADR-0002 has the decision rule, and
// it is worth repeating because the pressure runs the other way. Write a
// manifest unless the provider needs an API, a non-textual protocol, or logic no
// parser can express — and when a provider is *almost* expressible, write it
// native rather than adding the field that would make it fit. That field is how
// a schema turns into a programming language with no debugger and no tests.
//
// A manifest is arbitrary command execution by definition, since saying "run
// this" is the entire format. Built-in manifests are embedded in the binary and
// reviewed in pull requests; user-supplied overrides are off by default. See the
// security model in docs/ARCHITECTURE.md.
//
// The schema is public under semver. Renaming a field is a breaking change.
//
//	id        = "winget"
//	platforms = ["windows"]
//	elevate   = false
//
//	[detect]
//	command = ["winget", "--version"]
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
//	[apply]
//	command = ["winget", "upgrade", "--all", "--silent"]
type Manifest struct {
	// ID is the provider's stable short name. It is what config, --only, and
	// --except call this provider, and [Registry.Add] decides whether it is
	// usable — this package validates that the field is present, and the
	// registry validates its shape, so there is one rule rather than two that
	// can disagree.
	ID string `toml:"id"`

	// Platforms is where this provider can run, spelled as GOOS does:
	// "windows", "linux", "darwin". An unknown value is an error rather than a
	// platform that never matches, because a typo that silently disabled a
	// provider everywhere would look exactly like the tool being absent.
	Platforms []string `toml:"platforms"`

	// Elevate says whether [Manifest.Apply] needs admin or root. Planning never
	// does.
	//
	// The field is here from M4 because the adapter has to answer
	// [core.Provider].NeedsElevation from the moment it exists. What upall does
	// about it — marking the plan, and elevating only these providers — arrives
	// at M7.
	Elevate bool `toml:"elevate"`

	// Detect is the command that answers whether the tool is installed.
	Detect Step `toml:"detect"`

	// Plan is the read-only command that lists what would be updated, and how
	// to read what it printed.
	Plan PlanStep `toml:"plan"`

	// Apply is the command that installs the updates.
	Apply Step `toml:"apply"`
}

// Step is one command a manifest runs.
type Step struct {
	// Command is the program and its arguments, as an array.
	//
	// It is an array in the file for the same reason it is a []string in
	// [github.com/daniel-kindl/upall/internal/exec.Command]: there is no
	// quoting that is correct on both cmd.exe and sh, and a manifest that took
	// a command line as a string would be an injection surface in a file that
	// sometimes runs elevated. There is no string form to fall back to.
	Command []string `toml:"command"`

	// Env is added to the environment the command inherits, in "KEY=value"
	// form. It overlays rather than replaces, so a tool can still find what it
	// shells out to.
	//
	// It exists for the settings that make a package manager safe to run
	// unattended, such as DEBIAN_FRONTEND=noninteractive. It is not a place for
	// credentials: a manifest is a file in the repository.
	Env []string `toml:"env"`
}

// PlanStep is the plan command together with how to read its output.
type PlanStep struct {
	Step

	// Parser names the parser that reads this command's output. It must be one
	// of [ParserNames].
	Parser string `toml:"parser"`

	// ParserConfig is the block configuring that parser, which is [table],
	// [lines], or [json] nested under [plan].
	ParserConfig
}

// ManifestError reports a manifest that could not be loaded.
//
// It names the file and, where there is one, the field, because the reader is
// usually looking at a file they just wrote and the useful part of the message
// is which line to go to. The criterion at M4 asks for exactly this.
type ManifestError struct {
	// File is the manifest the problem is in.
	File string

	// Field is the TOML path to the problem, such as "plan.parser". It is empty
	// when the problem is with the file as a whole.
	Field string

	// Err is what was wrong.
	Err error
}

// Error names the file, the field, and the problem.
func (e *ManifestError) Error() string {
	if e.Field == "" {
		return fmt.Sprintf("%s: %v", e.File, e.Err)
	}
	return fmt.Sprintf("%s: %s: %v", e.File, e.Field, e.Err)
}

// Unwrap returns the underlying problem.
func (e *ManifestError) Unwrap() error { return e.Err }

// Load decodes and validates one manifest.
//
// It takes bytes and a name rather than a path because manifests are embedded
// with go:embed and have no path on disk. The name is what appears in errors.
//
// Validation is strict, and deliberately so. A manifest that loads is one the
// rest of the system can rely on, which is what lets the adapter built from it
// be as thin as it is. The failure being loud also matters more here than it
// looks: a manifest with a mistyped field would otherwise report an up-to-date
// machine, and silence is indistinguishable from good news.
func Load(name string, data []byte) (*Manifest, error) {
	var m Manifest

	md, err := toml.Decode(string(data), &m)
	if err != nil {
		return nil, &ManifestError{File: name, Err: err}
	}

	// Keys the schema has no field for. This is where a typo is caught, and it
	// catches more than misspellings: "parser" under [detect] is rejected here
	// too, because only [plan] has one, so a manifest cannot quietly configure
	// something that will never be read.
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		keys := make([]string, len(undecoded))
		for i, key := range undecoded {
			keys[i] = key.String()
		}
		slices.Sort(keys)
		return nil, &ManifestError{
			File:  name,
			Field: keys[0],
			Err:   fmt.Errorf("unknown field; the manifest has %d unknown: %s", len(keys), strings.Join(keys, ", ")),
		}
	}

	if err := m.validate(name); err != nil {
		return nil, err
	}
	return &m, nil
}

// errMissing is what a required field that was left out reports.
var errMissing = errors.New("required field is missing")

// validate checks everything decoding cannot.
func (m *Manifest) validate(file string) error {
	fail := func(field string, err error) error {
		return &ManifestError{File: file, Field: field, Err: err}
	}

	if m.ID == "" {
		return fail("id", errMissing)
	}
	if len(m.Platforms) == 0 {
		return fail("platforms", errMissing)
	}
	for _, p := range m.Platforms {
		if !slices.Contains(knownPlatforms, core.Platform(p)) {
			return fail("platforms", fmt.Errorf("unknown platform %q; known platforms are %s",
				p, strings.Join(platformNames(), ", ")))
		}
	}

	steps := []struct {
		field   string
		command []string
	}{
		{field: "detect.command", command: m.Detect.Command},
		{field: "plan.command", command: m.Plan.Command},
		{field: "apply.command", command: m.Apply.Command},
	}
	for _, step := range steps {
		if len(step.command) == 0 {
			return fail(step.field, errMissing)
		}
		for i, arg := range step.command {
			if arg == "" {
				return fail(step.field, fmt.Errorf("argument %d is empty", i))
			}
		}
	}

	if m.Plan.Parser == "" {
		return fail("plan.parser", errMissing)
	}
	// Building the parser here rather than at plan time is what makes an
	// unknown parser name, or a configuration block that maps nothing, a load
	// failure naming the file. Discovering it during a run would mean a
	// provider that reports no updates for a reason nobody can see.
	if _, err := m.Parser(); err != nil {
		return fail("plan.parser", err)
	}

	return nil
}

// knownPlatforms is what the platforms field may name.
var knownPlatforms = []core.Platform{core.Windows, core.Linux, core.Darwin}

// platformNames is knownPlatforms as strings, for an error message.
func platformNames() []string {
	names := make([]string, len(knownPlatforms))
	for i, p := range knownPlatforms {
		names[i] = string(p)
	}
	return names
}

// PlatformSet returns the platforms this manifest declared.
//
// The values are validated at load, so anything that reaches here is one
// [core.PlatformSet] understands.
func (m *Manifest) PlatformSet() core.PlatformSet {
	set := make(core.PlatformSet, len(m.Platforms))
	for i, p := range m.Platforms {
		set[i] = core.Platform(p)
	}
	return set
}

// Parser builds the parser the plan step names.
//
// It is called once during [Load], to fail a bad manifest there, and again when
// a provider is built from this manifest. Parsers are cheap to construct and
// stateless once built, so building one twice costs nothing and saves this type
// from carrying a field that only some manifests have filled in.
func (m *Manifest) Parser() (Parser, error) {
	return NewParser(m.Plan.Parser, m.Plan.ParserConfig)
}
