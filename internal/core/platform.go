package core

import (
	"runtime"
	"strings"
)

// Platform is an operating system a provider can run on, spelled the way Go
// spells GOOS.
//
// The spelling matters because it is the one a manifest author will type, a
// user will read in `upall providers`, and the toolchain already uses. Inventing
// a second vocabulary for the same three values would mean translating between
// them forever.
type Platform string

// The platforms upall knows about.
//
// [Windows] and [Linux] are supported. [Darwin] is representable so that
// [Current] can name a machine upall was not built for, and so the type does not
// have to change when macOS support arrives. No provider declares it today, so a
// mac gets a run where every provider is filtered out, which is an empty result
// rather than a crash or a lie.
const (
	Windows Platform = "windows"
	Linux   Platform = "linux"
	Darwin  Platform = "darwin"
)

// Current is the platform this binary is running on.
//
// This is the only place in upall that reads runtime.GOOS. The rule in
// docs/ARCHITECTURE.md forbids *branching* on it, because such a branch compiles
// on every platform while only ever being exercised on one, hiding the mistakes
// in the arm you are not currently running. Reading it into a value branches on
// nothing, and having exactly one place that does it is what keeps the branches
// from growing back elsewhere: everything downstream compares Platform values,
// and platform-specific behavior goes in a file with a build tag.
func Current() Platform {
	return Platform(runtime.GOOS)
}

// PlatformSet is the set of platforms a provider can run on.
//
// Order is not significant, and duplicates change nothing.
type PlatformSet []Platform

// Supports reports whether a provider declaring this set can run on p.
//
// The empty set supports nothing. That is deliberate: a provider whose platforms
// were left off, or lost to a typo in a manifest, is skipped everywhere rather
// than attempted everywhere. Being absent from a run is upall's normal case and
// costs nothing, while running apt on Windows is a confusing failure the user
// did not ask for.
func (s PlatformSet) Supports(p Platform) bool {
	for _, candidate := range s {
		if candidate == p {
			return true
		}
	}
	return false
}

// String renders the set as its platforms in declaration order, comma-separated,
// for a provider listing. The empty set renders as "none".
func (s PlatformSet) String() string {
	if len(s) == 0 {
		return "none"
	}

	names := make([]string, len(s))
	for i, p := range s {
		names[i] = string(p)
	}
	return strings.Join(names, ", ")
}
