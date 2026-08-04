package core

import "context"

// Provider is one thing that can update software: winget, apt, docker, Windows
// Update.
//
// It is the unit of everything in upall. Detection, planning, elevation,
// failure, and reporting all happen per provider, and a provider's outcome is
// independent of every other's.
//
// There are two kinds and this interface is the reason nothing has to care
// which. Most providers are a TOML manifest describing commands and a parser,
// loaded into an adapter that satisfies this interface; a few need real code and
// implement it directly. The registry, the pipeline, the config layer, and both
// frontends see one kind of thing. See ADR-0002.
//
// # Implementing one
//
// The three methods that do work take a [context.Context] and must honour its
// cancellation, all the way down into whatever subprocess they start. Ctrl-C
// during a run has to unwind, not wait.
//
// None of them may write to a terminal. Nothing below internal/cli may, and a
// provider is well below it.
//
// Implementations are used concurrently across providers during detect and plan,
// so they must not share mutable state.
type Provider interface {
	// ID is the provider's stable short name, such as "winget" or "apt".
	//
	// It appears in the config file, in --only and --except, and in the JSON
	// output schema, all of which are public interfaces under semver. Renaming
	// one is a breaking change.
	ID() string

	// Platforms is where this provider can run at all. A provider is filtered
	// out of the run before anything else happens if its set does not support
	// [Current].
	Platforms() PlatformSet

	// NeedsElevation reports whether applying requires admin or root. Planning
	// never does, so this describes [Provider.Apply] alone.
	//
	// It is answered before the plan is rendered, because the user is told
	// which providers will be elevated while deciding whether to proceed, not
	// afterwards. See ADR-0003.
	NeedsElevation() bool

	// Detect reports whether this provider's tool is present and usable here.
	//
	// Returning (false, nil) means it is not installed. That is the normal
	// case, not an error: most machines will not have most providers, and one
	// that is absent is dropped from the run and mentioned only if asked.
	//
	// An error means the question could not be answered — the check itself
	// failed — which is different from answering "no".
	Detect(ctx context.Context) (bool, error)

	// Plan reports what this provider would update. It is read-only and must
	// change nothing on the machine.
	//
	// No updates available is an empty slice and a nil error. The provider's ID
	// and elevation requirement are not returned here; the pipeline attaches
	// them when it builds the [ProviderPlan].
	Plan(ctx context.Context) ([]Update, error)

	// Apply installs the updates, which are the ones [Provider.Plan] returned
	// and the user confirmed.
	//
	// It reports success or an error and nothing else. The pipeline times the
	// call, runs the error through [Classify], and builds the
	// [ProviderResult], identically for every provider — which is what stops a
	// manifest adapter and a native provider from disagreeing about what a
	// failure record looks like. Return [ErrNeedsElevation], wrapped or bare,
	// when the work could not proceed unprivileged.
	//
	// Apply is never called concurrently with itself for the same provider.
	// Most package managers hold a global lock, so two of them at once fail
	// confusingly.
	Apply(ctx context.Context, updates []Update) error
}
