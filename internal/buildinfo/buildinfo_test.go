package buildinfo

import (
	"runtime"
	"runtime/debug"
	"testing"
)

// stamps builds the build-setting slice the toolchain would have embedded for a
// binary built at the given revision.
func stamps(revision, buildTime, modified string) []debug.BuildSetting {
	return []debug.BuildSetting{
		// Real build info carries settings resolve does not care about.
		// Including some here proves the lookup selects by key rather than by
		// position.
		{Key: "-compiler", Value: "gc"},
		{Key: "GOARCH", Value: "amd64"},
		{Key: "vcs", Value: "git"},
		{Key: "vcs.revision", Value: revision},
		{Key: "vcs.time", Value: buildTime},
		{Key: "vcs.modified", Value: modified},
	}
}

func TestResolve(t *testing.T) {
	const (
		fullSHA = "a1b2c3d4e5f60718293a4b5c6d7e8f9012345678"
		when    = "2026-08-03T14:22:11Z"
	)

	tests := []struct {
		name        string
		ldVersion   string
		ldCommit    string
		ldDate      string
		mainVersion string
		settings    []debug.BuildSetting

		wantVersion string
		wantCommit  string
		wantDate    string
		wantDirty   bool
	}{
		{
			name:        "linker flags win over vcs stamps",
			ldVersion:   "1.2.0",
			ldCommit:    "deadbee",
			ldDate:      "2026-01-01T00:00:00Z",
			mainVersion: "(devel)",
			settings:    stamps(fullSHA, when, "false"),

			wantVersion: "1.2.0",
			wantCommit:  "deadbee",
			wantDate:    "2026-01-01T00:00:00Z",
		},
		{
			name:        "plain go build falls back to vcs stamps",
			mainVersion: "(devel)",
			settings:    stamps(fullSHA, when, "false"),

			wantVersion: devVersion,
			wantCommit:  "a1b2c3d",
			wantDate:    when,
		},
		{
			name:        "go install records the module version",
			mainVersion: "v1.3.0-dev.4",
			settings:    stamps(fullSHA, when, "false"),

			wantVersion: "v1.3.0-dev.4",
			wantCommit:  "a1b2c3d",
			wantDate:    when,
		},
		{
			name: "a pseudo-version built from the commit in hand is just dev",
			// What the toolchain synthesizes for a working-tree build. It
			// restates the commit and date already on the line.
			mainVersion: "v0.0.0-20260803143229-a1b2c3d4e5f6+dirty",
			settings:    stamps(fullSHA, when, "true"),

			wantVersion: devVersion,
			wantCommit:  "a1b2c3d",
			wantDate:    when,
			wantDirty:   true,
		},
		{
			name: "a pseudo-version with no commit to compare against survives",
			// `go install …@main` builds from a downloaded module, so there
			// are no VCS stamps and this is the only provenance there is.
			mainVersion: "v0.0.0-20260803143229-a1b2c3d4e5f6",
			settings:    nil,

			wantVersion: "v0.0.0-20260803143229-a1b2c3d4e5f6",
			wantCommit:  "",
			wantDate:    "",
		},
		{
			name:        "built outside a repository knows only that it is dev",
			mainVersion: "(devel)",
			settings:    nil,

			wantVersion: devVersion,
			wantCommit:  "",
			wantDate:    "",
		},
		{
			name:        "a dirty tree is recorded",
			mainVersion: "(devel)",
			settings:    stamps(fullSHA, when, "true"),

			wantVersion: devVersion,
			wantCommit:  "a1b2c3d",
			wantDate:    when,
			wantDirty:   true,
		},
		{
			name:        "a commit shorter than the cut is left alone",
			mainVersion: "(devel)",
			settings:    stamps("abc123", when, "false"),

			wantVersion: devVersion,
			wantCommit:  "abc123",
			wantDate:    when,
		},
		{
			name:        "an empty version flag does not blank the version",
			ldVersion:   "",
			ldCommit:    "deadbee",
			mainVersion: "",
			settings:    nil,

			wantVersion: devVersion,
			wantCommit:  "deadbee",
			wantDate:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolve(tt.ldVersion, tt.ldCommit, tt.ldDate, tt.mainVersion, tt.settings)

			if got.Version != tt.wantVersion {
				t.Errorf("Version = %q, want %q", got.Version, tt.wantVersion)
			}
			if got.Commit != tt.wantCommit {
				t.Errorf("Commit = %q, want %q", got.Commit, tt.wantCommit)
			}
			if got.Date != tt.wantDate {
				t.Errorf("Date = %q, want %q", got.Date, tt.wantDate)
			}
			if got.Dirty != tt.wantDirty {
				t.Errorf("Dirty = %v, want %v", got.Dirty, tt.wantDirty)
			}

			// The toolchain fields are not merged from anywhere, so they are
			// asserted once rather than per case.
			if got.GoVersion != runtime.Version() {
				t.Errorf("GoVersion = %q, want %q", got.GoVersion, runtime.Version())
			}
			if got.OS != runtime.GOOS || got.Arch != runtime.GOARCH {
				t.Errorf("target = %s/%s, want %s/%s", got.OS, got.Arch, runtime.GOOS, runtime.GOARCH)
			}
		})
	}
}

func TestInfoString(t *testing.T) {
	tests := []struct {
		name string
		info Info
		want string
	}{
		{
			name: "everything known",
			info: Info{
				Version:   "1.2.0",
				Commit:    "a1b2c3d",
				Date:      "2026-08-03T14:22:11Z",
				GoVersion: "go1.26.5",
				OS:        "windows",
				Arch:      "amd64",
			},
			want: "upall 1.2.0 (a1b2c3d, 2026-08-03T14:22:11Z)\ngo1.26.5 windows/amd64",
		},
		{
			name: "a dirty tree is marked on the commit",
			info: Info{
				Version:   "dev",
				Commit:    "a1b2c3d",
				Date:      "2026-08-03T14:22:11Z",
				Dirty:     true,
				GoVersion: "go1.26.5",
				OS:        "linux",
				Arch:      "arm64",
			},
			want: "upall dev (a1b2c3d-dirty, 2026-08-03T14:22:11Z)\ngo1.26.5 linux/arm64",
		},
		{
			name: "nothing known but the version says only that",
			info: Info{
				Version:   "dev",
				GoVersion: "go1.26.5",
				OS:        "linux",
				Arch:      "amd64",
			},
			want: "upall dev\ngo1.26.5 linux/amd64",
		},
		{
			name: "a commit with no date does not print an empty field",
			info: Info{
				Version:   "dev",
				Commit:    "a1b2c3d",
				GoVersion: "go1.26.5",
				OS:        "linux",
				Arch:      "amd64",
			},
			want: "upall dev (a1b2c3d)\ngo1.26.5 linux/amd64",
		},
		{
			name: "a date with no commit does not print an empty field",
			info: Info{
				Version:   "dev",
				Date:      "2026-08-03T14:22:11Z",
				GoVersion: "go1.26.5",
				OS:        "linux",
				Arch:      "amd64",
			},
			want: "upall dev (2026-08-03T14:22:11Z)\ngo1.26.5 linux/amd64",
		},
		{
			name: "a dirty tree with no commit has nothing to mark",
			info: Info{
				Version:   "dev",
				Dirty:     true,
				GoVersion: "go1.26.5",
				OS:        "linux",
				Arch:      "amd64",
			},
			want: "upall dev\ngo1.26.5 linux/amd64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.info.String(); got != tt.want {
				t.Errorf("String() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

// TestGet checks the wiring rather than the merging: that Get reads the real
// build info and produces something usable. The test binary is built by `go
// test`, so its stamps are whatever the toolchain gave it, and asserting on
// their values would assert on how the test was invoked.
func TestGet(t *testing.T) {
	got := Get()

	if got.Version == "" {
		t.Error("Version is empty; it should always fall back to a placeholder")
	}
	if got.GoVersion != runtime.Version() {
		t.Errorf("GoVersion = %q, want %q", got.GoVersion, runtime.Version())
	}
	if len(got.Commit) > shortCommitLen {
		t.Errorf("Commit = %q, longer than the %d it should be cut to", got.Commit, shortCommitLen)
	}
}
