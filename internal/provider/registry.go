package provider

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/daniel-kindl/upall/internal/core"
)

// Registry is the set of providers a run can draw on.
//
// It is the one place that knows every provider exists, and it answers the two
// questions asked before a run does anything: which provider is called this, and
// which of them could run here. Everything after that — detect, plan, apply — is
// [core.Provider] and does not care where the value came from.
//
// A manifest adapter and a native provider are both registered through
// [Registry.Add] as a [core.Provider], so nothing here can tell them apart.
// That indistinguishability is the point of ADR-0002 and it is why this type
// has no notion of a manifest.
//
// Build one and then share it. [Registry.Add] is not safe to call concurrently,
// and nothing needs it to be: providers are registered once at startup, and the
// reads that follow happen from the goroutines detect and plan fan out into.
type Registry struct {
	// byID is the resolution index. Registration order is not kept, because
	// nothing is entitled to it — see [Registry.All].
	byID map[string]core.Provider
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{byID: make(map[string]core.Provider)}
}

// validID is what a provider ID may look like.
//
// The character set is deliberately narrower than it needs to be. An ID is a
// TOML key in the config file, a value for --only and --except, and a field in
// the JSON output schema, all of which are public under semver — so a provider
// that shipped with a space or a capital in its ID would have to keep it
// forever. Lowercase, digits, and internal hyphens cover every provider in the
// ROADMAP through M9, and leave the awkward characters unclaimed.
var validID = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// Add registers a provider.
//
// It fails on an empty or malformed ID, and on an ID already registered. A
// duplicate is a bug rather than a configuration to resolve: two providers
// answering to one name means --only picks whichever won a race, so it is
// refused here, where the error can name what collided, rather than silently
// preferring one.
func (r *Registry) Add(p core.Provider) error {
	id := p.ID()

	if id == "" {
		return fmt.Errorf("a provider has no ID; an ID is how config and --only name it")
	}
	if !validID.MatchString(id) {
		return fmt.Errorf("provider ID %q is not usable: IDs are lowercase letters, digits, and internal hyphens", id)
	}
	if _, taken := r.byID[id]; taken {
		return fmt.Errorf("provider ID %q is registered twice", id)
	}

	r.byID[id] = p
	return nil
}

// Lookup returns the provider with this ID.
//
// The second return distinguishes "no such provider" from a nil one, which is
// what lets --only reject an ID that does not exist instead of silently
// filtering the run down to nothing.
func (r *Registry) Lookup(id string) (core.Provider, bool) {
	p, found := r.byID[id]
	return p, found
}

// All returns every registered provider, ordered by ID.
//
// Sorted rather than in registration order, and that is a contract rather than
// an implementation detail. Registration order is whatever the embedded
// manifests happened to be walked in, so depending on it would make a renamed
// file change the order of `upall providers` and of every plan rendered below
// it. Sorting by ID means the output is the same on two machines that loaded the
// same providers, which is what makes it diffable.
//
// It is not an apply order. Apply is the pipeline's to schedule, under the
// concurrency bound described in internal/pipeline.
func (r *Registry) All() []core.Provider {
	providers := make([]core.Provider, 0, len(r.byID))
	for _, p := range r.byID {
		providers = append(providers, p)
	}
	slices.SortFunc(providers, func(a, b core.Provider) int {
		return strings.Compare(a.ID(), b.ID())
	})
	return providers
}

// For returns the providers that can run on this platform, ordered by ID.
//
// This is the first filter of a run and the cheapest: it asks what a provider
// declared, not what is installed, so it needs no subprocess and cannot fail.
// Whether the tool is actually present is [core.Provider].Detect's question, and
// a provider that survives here and answers false there is the ordinary case
// rather than a problem.
//
// A provider declaring no platforms is filtered out everywhere. [core.PlatformSet]
// says why: a set lost to a typo in a manifest should mean the provider is
// skipped, not that apt is attempted on Windows.
func (r *Registry) For(platform core.Platform) []core.Provider {
	var supported []core.Provider
	for _, p := range r.All() {
		if p.Platforms().Supports(platform) {
			supported = append(supported, p)
		}
	}
	return supported
}

// IDs returns every registered ID, sorted.
//
// It exists so that a message about an unknown provider can list the real ones.
// A user who typed --only wingett should be shown what is available rather than
// told to consult the documentation.
func (r *Registry) IDs() []string {
	ids := make([]string, 0, len(r.byID))
	for id := range r.byID {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

// Len returns how many providers are registered.
func (r *Registry) Len() int {
	return len(r.byID)
}
