package provider

import (
	"context"
	"slices"
	"testing"

	"github.com/daniel-kindl/upall/internal/core"
)

// stub is a provider that does nothing, for tests about the registry rather than
// about providers. The registry never calls the three methods that do work, and
// this failing if it did is the point: resolving and filtering must not run
// anything on the machine.
type stub struct {
	id        string
	platforms core.PlatformSet
	t         *testing.T
}

func newStub(t *testing.T, id string, platforms ...core.Platform) *stub {
	t.Helper()
	return &stub{id: id, platforms: platforms, t: t}
}

func (s *stub) ID() string                  { return s.id }
func (s *stub) Platforms() core.PlatformSet { return s.platforms }
func (s *stub) NeedsElevation() bool        { return false }

func (s *stub) Detect(context.Context) (bool, error) {
	s.t.Errorf("the registry called Detect on %q; resolving and filtering run nothing", s.id)
	return false, nil
}

func (s *stub) Plan(context.Context) ([]core.Update, error) {
	s.t.Errorf("the registry called Plan on %q; resolving and filtering run nothing", s.id)
	return nil, nil
}

func (s *stub) Apply(context.Context, []core.Update) error {
	s.t.Errorf("the registry called Apply on %q; resolving and filtering run nothing", s.id)
	return nil
}

// mustRegistry builds a registry from stubs, failing the test if any is refused.
func mustRegistry(t *testing.T, providers ...core.Provider) *Registry {
	t.Helper()

	r := NewRegistry()
	for _, p := range providers {
		if err := r.Add(p); err != nil {
			t.Fatalf("registering %q: %v", p.ID(), err)
		}
	}
	return r
}

// ids is the IDs of these providers, in the order they were returned.
func ids(providers []core.Provider) []string {
	out := make([]string, len(providers))
	for i, p := range providers {
		out[i] = p.ID()
	}
	return out
}

func TestLookupResolvesByID(t *testing.T) {
	apt := newStub(t, "apt", core.Linux)
	r := mustRegistry(t, apt, newStub(t, "winget", core.Windows))

	got, found := r.Lookup("apt")
	if !found {
		t.Fatal("apt was registered but did not resolve")
	}
	if got != core.Provider(apt) {
		t.Errorf("resolved %q, want the apt stub", got.ID())
	}
}

// TestLookupSaysWhenThereIsNoSuchProvider is what lets --only reject an unknown
// ID rather than filtering the run down to nothing and reporting success.
func TestLookupSaysWhenThereIsNoSuchProvider(t *testing.T) {
	r := mustRegistry(t, newStub(t, "apt", core.Linux))

	if _, found := r.Lookup("wingett"); found {
		t.Error("an ID that was never registered resolved")
	}
}

// TestAllIsOrderedByIDNotByRegistration is the contract stated on Registry.All.
// The stubs go in reversed so that registration order and ID order disagree.
func TestAllIsOrderedByIDNotByRegistration(t *testing.T) {
	r := mustRegistry(t,
		newStub(t, "winget", core.Windows),
		newStub(t, "snap", core.Linux),
		newStub(t, "apt", core.Linux),
	)

	want := []string{"apt", "snap", "winget"}
	if got := ids(r.All()); !slices.Equal(got, want) {
		t.Errorf("All returned %v, want %v", got, want)
	}
	if got := r.IDs(); !slices.Equal(got, want) {
		t.Errorf("IDs returned %v, want %v", got, want)
	}
}

// TestForFiltersByPlatform covers the first filter of a run, on both supported
// platforms and on the one that is representable but has no providers.
func TestForFiltersByPlatform(t *testing.T) {
	registry := func(t *testing.T) *Registry {
		return mustRegistry(t,
			newStub(t, "apt", core.Linux),
			newStub(t, "snap", core.Linux, core.Windows),
			newStub(t, "winget", core.Windows),
		)
	}

	tests := []struct {
		platform core.Platform
		want     []string
	}{
		{platform: core.Linux, want: []string{"apt", "snap"}},
		{platform: core.Windows, want: []string{"snap", "winget"}},

		// A mac gets a run where every provider is filtered out, which is an
		// empty result rather than an error. core.Platform says so, and this is
		// where that becomes true.
		{platform: core.Darwin, want: nil},
	}

	for _, tt := range tests {
		t.Run(string(tt.platform), func(t *testing.T) {
			if got := ids(registry(t).For(tt.platform)); !slices.Equal(got, tt.want) {
				t.Errorf("For(%s) returned %v, want %v", tt.platform, got, tt.want)
			}
		})
	}
}

// TestForSkipsAProviderDeclaringNoPlatforms covers a manifest whose platform
// list was left off or lost to a typo. Being absent from every run is the safe
// reading; being attempted on every run means apt on Windows.
func TestForSkipsAProviderDeclaringNoPlatforms(t *testing.T) {
	r := mustRegistry(t, newStub(t, "nowhere"))

	for _, platform := range []core.Platform{core.Linux, core.Windows, core.Darwin} {
		if got := r.For(platform); len(got) != 0 {
			t.Errorf("For(%s) returned %v for a provider declaring no platforms", platform, ids(got))
		}
	}

	// It is still registered, so `upall providers` can list it and say why it
	// never runs.
	if _, found := r.Lookup("nowhere"); !found {
		t.Error("a provider with no platforms was dropped from the registry entirely")
	}
}

// TestAddRefusesADuplicateID covers two providers claiming one name, which would
// make --only pick whichever won a race.
func TestAddRefusesADuplicateID(t *testing.T) {
	r := mustRegistry(t, newStub(t, "apt", core.Linux))

	err := r.Add(newStub(t, "apt", core.Linux))
	if err == nil {
		t.Fatal("registered two providers under one ID")
	}
	if r.Len() != 1 {
		t.Errorf("the registry holds %d providers after a refused Add, want 1", r.Len())
	}
}

// TestAddRefusesAnUnusableID guards the character set. An ID is a TOML key, an
// --only value, and a JSON field, all public under semver, so one that shipped
// wrong would have to stay wrong.
func TestAddRefusesAnUnusableID(t *testing.T) {
	bad := []string{
		"",
		"Winget",
		"windows update",
		"apt.deb",
		"-apt",
		"apt-",
		"apt--get",
		"apt_get",
		"apt/deb",
		"--only",
	}

	for _, id := range bad {
		t.Run(id, func(t *testing.T) {
			r := NewRegistry()
			if err := r.Add(newStub(t, id, core.Linux)); err == nil {
				t.Errorf("registered a provider with ID %q", id)
			}
		})
	}
}

// TestAddAcceptsTheIDsUpallWillShip checks the character set from the other
// side, against the providers the ROADMAP names through M9.
func TestAddAcceptsTheIDsUpallWillShip(t *testing.T) {
	good := []string{
		"winget", "scoop", "chocolatey", "windows-update",
		"apt", "dnf", "pacman", "snap", "flatpak",
		"docker", "podman",
	}

	r := NewRegistry()
	for _, id := range good {
		if err := r.Add(newStub(t, id, core.Linux)); err != nil {
			t.Errorf("refused %q, which is a provider upall is going to ship: %v", id, err)
		}
	}
	if r.Len() != len(good) {
		t.Errorf("registered %d of %d providers", r.Len(), len(good))
	}
}
