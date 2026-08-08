package provider

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/daniel-kindl/upall/internal/core"
)

// validManifest is a complete, correct manifest. The rejection tests below break
// exactly one thing about it at a time, so that what a case proves is the change
// it made rather than the file it happened to write.
const validManifest = `
id        = "winget"
platforms = ["windows"]
elevate   = false

[detect]
command = ["winget", "--version"]

[plan]
command = ["winget", "upgrade"]
parser  = "table"

  [plan.table]
  name      = "Name"
  id        = "Id"
  installed = "Version"
  available = "Available"

[apply]
command = ["winget", "upgrade", "--all", "--silent"]
`

func TestLoadReadsAWholeManifest(t *testing.T) {
	m, err := Load("winget.toml", []byte(validManifest))
	if err != nil {
		t.Fatalf("loading a valid manifest: %v", err)
	}

	if m.ID != "winget" {
		t.Errorf("id is %q, want winget", m.ID)
	}
	if got := m.PlatformSet(); !slices.Equal(got, core.PlatformSet{core.Windows}) {
		t.Errorf("platforms are %v, want [windows]", got)
	}
	if m.Elevate {
		t.Error("elevate is true, want false")
	}

	if got := m.Detect.Command; !slices.Equal(got, []string{"winget", "--version"}) {
		t.Errorf("detect.command is %v", got)
	}
	if got := m.Plan.Command; !slices.Equal(got, []string{"winget", "upgrade"}) {
		t.Errorf("plan.command is %v", got)
	}
	if got := m.Apply.Command; !slices.Equal(got, []string{"winget", "upgrade", "--all", "--silent"}) {
		t.Errorf("apply.command is %v", got)
	}

	// The parser block nested under [plan] reached the embedded ParserConfig,
	// which is the part of the schema that depends on struct embedding rather
	// than on a field name.
	if m.Plan.Parser != ParserTable {
		t.Errorf("plan.parser is %q, want %q", m.Plan.Parser, ParserTable)
	}
	if m.Plan.Table == nil {
		t.Fatal("the [plan.table] block did not decode")
	}
	if m.Plan.Table.Name != "Name" || m.Plan.Table.Available != "Available" {
		t.Errorf("[plan.table] decoded as %+v", *m.Plan.Table)
	}

	if _, err := m.Parser(); err != nil {
		t.Errorf("building the parser a loaded manifest names: %v", err)
	}
}

// TestLoadElevateAndEnv covers the two fields the valid manifest leaves at their
// zero values, since a field that is only ever tested unset is not tested.
func TestLoadElevateAndEnv(t *testing.T) {
	const manifest = `
id        = "apt"
platforms = ["linux"]
elevate   = true

[detect]
command = ["apt-get", "--version"]

[plan]
command = ["apt", "list", "--upgradable"]
parser  = "lines"
env     = ["DEBIAN_FRONTEND=noninteractive"]

  [plan.lines]
  pattern = '^(?P<id>\S+)/\S+ (?P<available>\S+)'

[apply]
command = ["apt-get", "upgrade", "-y"]
env     = ["DEBIAN_FRONTEND=noninteractive", "NEEDRESTART_MODE=a"]
`

	m, err := Load("apt.toml", []byte(manifest))
	if err != nil {
		t.Fatalf("loading: %v", err)
	}

	if !m.Elevate {
		t.Error("elevate is false, want true")
	}
	if got := m.Plan.Env; !slices.Equal(got, []string{"DEBIAN_FRONTEND=noninteractive"}) {
		t.Errorf("plan.env is %v", got)
	}
	if got := m.Apply.Env; len(got) != 2 {
		t.Errorf("apply.env is %v, want two entries", got)
	}
}

// TestLoadRejects is the M4 criterion: unknown fields, missing required fields,
// and unknown parser names, each with an error naming the file and the field.
func TestLoadRejects(t *testing.T) {
	tests := []struct {
		name string

		// replace is applied to validManifest to break it. An empty from means
		// the whole manifest is replaced by to.
		from, to string

		// field is the TOML path the error must name.
		field string
	}{
		{
			name:  "an unknown top-level field",
			from:  `elevate   = false`,
			to:    `elevated  = false`,
			field: "elevated",
		},
		{
			name:  "an unknown field inside a step",
			from:  `command = ["winget", "upgrade"]`,
			to:    "command = [\"winget\", \"upgrade\"]\nshell = true",
			field: "plan.shell",
		},
		{
			name: "a parser configured on a step that has none",
			from: `[detect]
command = ["winget", "--version"]`,
			to: `[detect]
command = ["winget", "--version"]
parser  = "table"`,
			field: "detect.parser",
		},
		{
			name:  "a missing id",
			from:  `id        = "winget"`,
			to:    ``,
			field: "id",
		},
		{
			name:  "missing platforms",
			from:  `platforms = ["windows"]`,
			to:    ``,
			field: "platforms",
		},
		{
			name:  "an empty platform list",
			from:  `platforms = ["windows"]`,
			to:    `platforms = []`,
			field: "platforms",
		},
		{
			name:  "an unknown platform",
			from:  `platforms = ["windows"]`,
			to:    `platforms = ["win32"]`,
			field: "platforms",
		},
		{
			name: "a missing detect command",
			from: `[detect]
command = ["winget", "--version"]`,
			to:    `[detect]`,
			field: "detect.command",
		},
		{
			name:  "a missing plan command",
			from:  `command = ["winget", "upgrade"]`,
			to:    ``,
			field: "plan.command",
		},
		{
			name: "a missing apply command",
			from: `[apply]
command = ["winget", "upgrade", "--all", "--silent"]`,
			to:    `[apply]`,
			field: "apply.command",
		},
		{
			name:  "an empty argument in a command",
			from:  `command = ["winget", "upgrade"]`,
			to:    `command = ["winget", ""]`,
			field: "plan.command",
		},
		{
			name:  "a missing parser name",
			from:  `parser  = "table"`,
			to:    ``,
			field: "plan.parser",
		},
		{
			name:  "an unknown parser name",
			from:  `parser  = "table"`,
			to:    `parser  = "yaml"`,
			field: "plan.parser",
		},
		{
			name:  "a parser whose configuration block is missing",
			from:  `parser  = "table"`,
			to:    `parser  = "lines"`,
			field: "plan.parser",
		},
		{
			name: "a parser configuration that maps nothing",
			from: `  [plan.table]
  name      = "Name"
  id        = "Id"
  installed = "Version"
  available = "Available"`,
			to:    `  [plan.table]`,
			field: "plan.parser",
		},
		{
			name:  "TOML that does not parse",
			to:    `id = "winget`,
			field: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := tt.to
			if tt.from != "" {
				if !strings.Contains(validManifest, tt.from) {
					t.Fatalf("the test's own replacement target is not in the manifest: %q", tt.from)
				}
				manifest = strings.Replace(validManifest, tt.from, tt.to, 1)
			}

			_, err := Load("winget.toml", []byte(manifest))
			if err == nil {
				t.Fatal("loaded a manifest that should have been rejected")
			}

			var me *ManifestError
			if !errors.As(err, &me) {
				t.Fatalf("error is %T, want *ManifestError: %v", err, err)
			}
			if me.File != "winget.toml" {
				t.Errorf("the error names file %q, want winget.toml", me.File)
			}
			if me.Field != tt.field {
				t.Errorf("the error names field %q, want %q (full error: %v)", me.Field, tt.field, err)
			}
			if !strings.Contains(err.Error(), "winget.toml") {
				t.Errorf("the rendered error does not name the file: %v", err)
			}
		})
	}
}

// TestMissingFieldsUnwrapToASentinel lets a caller tell "you left this out" from
// "this is wrong", which are different things to tell a manifest author.
func TestMissingFieldsUnwrapToASentinel(t *testing.T) {
	manifest := strings.Replace(validManifest, `id        = "winget"`, "", 1)

	_, err := Load("winget.toml", []byte(manifest))
	if !errors.Is(err, errMissing) {
		t.Errorf("a missing required field returned %v, which does not unwrap to errMissing", err)
	}
}

// TestUnknownParserErrorListsTheKnownOnes checks that the catalogue's own
// helpfulness survives being wrapped in a ManifestError.
func TestUnknownParserErrorListsTheKnownOnes(t *testing.T) {
	manifest := strings.Replace(validManifest, `parser  = "table"`, `parser  = "yaml"`, 1)

	_, err := Load("winget.toml", []byte(manifest))
	if err == nil {
		t.Fatal("an unknown parser name was accepted")
	}
	for _, name := range ParserNames() {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("the error does not mention the known parser %q: %v", name, err)
		}
	}
}

// TestUnknownFieldErrorListsThemAll keeps a manifest with several typos from
// taking several runs to fix.
func TestUnknownFieldErrorListsThemAll(t *testing.T) {
	manifest := strings.Replace(validManifest, `elevate   = false`, "elevated  = false\nplatform  = \"windows\"", 1)

	_, err := Load("winget.toml", []byte(manifest))
	if err == nil {
		t.Fatal("unknown fields were accepted")
	}
	for _, field := range []string{"elevated", "platform"} {
		if !strings.Contains(err.Error(), field) {
			t.Errorf("the error does not mention the unknown field %q: %v", field, err)
		}
	}
}
