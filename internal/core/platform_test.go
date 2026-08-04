package core

import (
	"runtime"
	"testing"
)

func TestPlatformSetSupports(t *testing.T) {
	tests := []struct {
		name      string
		platforms PlatformSet
		host      Platform
		want      bool
	}{
		{
			name:      "a windows-only provider on windows",
			platforms: PlatformSet{Windows},
			host:      Windows,
			want:      true,
		},
		{
			name:      "a windows-only provider on linux",
			platforms: PlatformSet{Windows},
			host:      Linux,
			want:      false,
		},
		{
			name:      "a cross-platform provider on either",
			platforms: PlatformSet{Windows, Linux},
			host:      Linux,
			want:      true,
		},
		{
			name: "no provider claims darwin yet",
			// Every platform upall currently ships providers for, asked
			// about the one it does not. macOS is representable so that
			// Current can name it; the answer is a clean no.
			platforms: PlatformSet{Windows, Linux},
			host:      Darwin,
			want:      false,
		},
		{
			name:      "darwin is representable when something does claim it",
			platforms: PlatformSet{Darwin},
			host:      Darwin,
			want:      true,
		},
		{
			name: "the empty set supports nothing",
			// A provider whose platforms were left off, or lost to a typo
			// in a manifest, is skipped everywhere rather than attempted
			// everywhere.
			platforms: PlatformSet{},
			host:      Linux,
			want:      false,
		},
		{
			name:      "a nil set supports nothing either",
			platforms: nil,
			host:      Linux,
			want:      false,
		},
		{
			name:      "an unknown host matches nothing",
			platforms: PlatformSet{Windows, Linux},
			host:      Platform("plan9"),
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.platforms.Supports(tt.host); got != tt.want {
				t.Errorf("PlatformSet{%s}.Supports(%q) = %v, want %v", tt.platforms, tt.host, got, tt.want)
			}
		})
	}
}

func TestPlatformSetString(t *testing.T) {
	tests := []struct {
		name      string
		platforms PlatformSet
		want      string
	}{
		{
			name:      "one platform",
			platforms: PlatformSet{Linux},
			want:      "linux",
		},
		{
			name:      "several keep declaration order",
			platforms: PlatformSet{Windows, Linux},
			want:      "windows, linux",
		},
		{
			name:      "the empty set says so rather than rendering blank",
			platforms: PlatformSet{},
			want:      "none",
		},
		{
			name:      "a nil set is the empty set",
			platforms: nil,
			want:      "none",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.platforms.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestCurrent checks that the one permitted read of runtime.GOOS reports the
// platform the test is running on. It cannot assert a literal without asserting
// which machine ran it, so it compares against the same source, which at least
// proves the conversion is not dropping or rewriting anything.
func TestCurrent(t *testing.T) {
	if got := Current(); string(got) != runtime.GOOS {
		t.Errorf("Current() = %q, want %q", got, runtime.GOOS)
	}

	// CI runs on windows-latest and ubuntu-latest, so on CI this is one of the
	// two supported platforms. A developer on a mac gets Darwin, which is
	// representable and supported by no provider, so the assertion is only
	// that the value is one core knows about.
	switch got := Current(); got {
	case Windows, Linux, Darwin:
	default:
		t.Errorf("Current() = %q, which is not a platform core names", got)
	}
}
