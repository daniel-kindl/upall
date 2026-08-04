package core

// Update is one thing a provider would install: a package with a newer version
// available, a pending OS update, a container image with a newer digest.
//
// It is a description rather than a state machine. The same value is produced by
// [Provider.Plan], rendered to the user, handed back to [Provider.Apply], and
// recorded in the result, and nothing along the way modifies it. What happened
// to it is carried by the [ProviderResult] that reports on the batch, not by a
// field here, because the tools upall drives update everything in one command
// and do not report per-package outcomes. Claiming otherwise would mean
// inventing detail.
//
// Every field but Name may be empty. Package managers disagree about how much
// they will tell you, and an empty string is the honest rendering of a thing
// this one did not say.
type Update struct {
	// Name is what a human calls it, such as "Mozilla Firefox".
	Name string

	// ID is what the underlying tool calls it, such as "Mozilla.Firefox" to
	// winget or "firefox" to apt. It is what you would type to act on this
	// package yourself, which is why it is kept alongside the display name
	// rather than instead of it.
	ID string

	// Installed is the version currently on the machine, empty if the provider
	// does not report one.
	Installed string

	// Available is the version that would replace it, empty if the provider
	// reports only that something is out of date.
	Available string
}
