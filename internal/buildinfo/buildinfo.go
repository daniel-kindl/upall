package buildinfo

import (
	"runtime"
	"runtime/debug"
	"strings"
)

// Overwritten at link time with -ldflags "-X <import path>.<name>=<value>".
// Left alone, they are what a plain `go build` produces, and [Get] fills the
// gaps from the VCS stamps the toolchain embeds by itself.
var (
	version = ""
	commit  = ""
	date    = ""
)

// devVersion is what a binary calls itself when nothing told it otherwise:
// neither a linker flag nor a module version from `go install`. It is not a
// valid semver string, which is deliberate, so that a development build can
// never be mistaken for a release by something comparing versions.
const devVersion = "dev"

// shortCommitLen is how much of a commit SHA gets printed. Seven hex digits is
// what git abbreviates to by default, and it is enough to find a commit in a
// repository this size.
const shortCommitLen = 7

// Info is the provenance of a upall binary: which version it claims to be,
// which commit it was built from, and when.
//
// Any field can be empty. A binary built outside a repository knows neither its
// commit nor its date, and saying so is better than inventing either.
type Info struct {
	// Version is the release it claims to be, such as "1.2.0" or
	// "1.3.0-dev.4", or "dev" for a build that was never given one.
	Version string

	// Commit is the abbreviated SHA of the commit it was built from, empty if
	// it was not built inside a repository.
	Commit string

	// Date is when it was built, in RFC 3339, empty if unknown.
	Date string

	// Dirty reports whether the working tree had uncommitted changes at build
	// time, which means Commit does not fully describe what is in this binary.
	Dirty bool

	// GoVersion is the toolchain that compiled it, such as "go1.26.5".
	GoVersion string

	// OS and Arch are the target it was compiled for, in GOOS and GOARCH
	// spelling.
	OS, Arch string
}

// Get returns the provenance of the running binary.
func Get() Info {
	var mainVersion string
	var settings []debug.BuildSetting
	if bi, ok := debug.ReadBuildInfo(); ok {
		mainVersion = bi.Main.Version
		settings = bi.Settings
	}
	return resolve(version, commit, date, mainVersion, settings)
}

// resolve merges the three linker-flag values with what the toolchain stamped
// into the binary, and is where every fallback decision is made.
//
// It is separated from [Get] so the merging can be tested against made-up
// stamps. Testing it through Get would mean building binaries from a test,
// which would prove the linker works rather than that this logic does.
func resolve(ldVersion, ldCommit, ldDate, mainVersion string, settings []debug.BuildSetting) Info {
	lookup := func(key string) string {
		for _, s := range settings {
			if s.Key == key {
				return s.Value
			}
		}
		return ""
	}

	revision := lookup("vcs.revision")

	info := Info{
		Version:   ldVersion,
		Commit:    ldCommit,
		Date:      ldDate,
		Dirty:     lookup("vcs.modified") == "true",
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}

	if info.Version == "" {
		info.Version = resolveVersion(mainVersion, revision)
	}

	if info.Commit == "" {
		info.Commit = revision
	}
	if len(info.Commit) > shortCommitLen {
		info.Commit = info.Commit[:shortCommitLen]
	}

	if info.Date == "" {
		info.Date = lookup("vcs.time")
	}

	return info
}

// resolveVersion picks the version to report when no linker flag supplied one.
//
// The recorded module version is authoritative for a binary installed with
// `go install …@v1.2.3`. It is not for one built inside a working tree: there
// the toolchain synthesizes a pseudo-version such as
//
//	v0.0.0-20260803143229-2324cc3ee29c+dirty
//
// which says nothing the commit and date on the same line do not already say,
// at four times the width, while looking enough like a release to be mistaken
// for one. Recognizing it is a matter of asking whether it was derived from the
// commit already in hand.
//
// A pseudo-version from `go install …@main` survives this, because a binary
// built from a downloaded module carries no VCS stamps to compare against, and
// there it is the only provenance available.
func resolveVersion(mainVersion, revision string) string {
	if mainVersion == "" || mainVersion == "(devel)" {
		return devVersion
	}

	// A pseudo-version embeds the first 12 hex digits of the commit.
	const pseudoVersionSHALen = 12
	if len(revision) >= pseudoVersionSHALen && strings.Contains(mainVersion, revision[:pseudoVersionSHALen]) {
		return devVersion
	}

	return mainVersion
}

// String renders the provenance as two lines, the first about upall and the
// second about what compiled it:
//
//	upall 1.2.0 (a1b2c3d, 2026-08-03T14:22:11Z)
//	go1.26.5 windows/amd64
//
// Unknown parts are left out rather than printed as "unknown", so a binary
// built outside a repository says only what it can stand behind. A commit built
// from a dirty tree is marked, because on its own it would be a lie.
func (i Info) String() string {
	var b strings.Builder
	b.WriteString("upall ")
	b.WriteString(i.Version)

	commitText := i.Commit
	if commitText != "" && i.Dirty {
		commitText += "-dirty"
	}

	var parts []string
	if commitText != "" {
		parts = append(parts, commitText)
	}
	if i.Date != "" {
		parts = append(parts, i.Date)
	}
	if len(parts) > 0 {
		b.WriteString(" (")
		b.WriteString(strings.Join(parts, ", "))
		b.WriteString(")")
	}

	b.WriteString("\n")
	b.WriteString(i.GoVersion)
	b.WriteString(" ")
	b.WriteString(i.OS)
	b.WriteString("/")
	b.WriteString(i.Arch)

	return b.String()
}
