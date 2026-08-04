package core

import (
	"slices"
	"strings"
	"time"
)

// Outcome is how one provider's part of a run ended.
//
// The set is closed and matches the error taxonomy in docs/ARCHITECTURE.md and
// the state vocabulary in docs/PRODUCT.md, because both frontends and the
// journal render from this one list. Adding a value means deciding what it looks
// like in the terminal, what it looks like in the GUI, and which exit code it
// contributes to, so it is not a decision to make while writing a provider.
type Outcome int

// The outcomes a provider can end a run with.
//
// [Unknown] is the zero value and is not one of them. It means nobody recorded
// an outcome, which never happens in a finished run, and exists so that a
// [ProviderResult] left half-built cannot pass itself off as a success. Failing
// loudly on a bug in upall is better than reporting that updates were applied
// when nothing ran.
const (
	Unknown Outcome = iota

	// Succeeded means it ran and did what it said it would.
	Succeeded

	// Absent means the provider is not installed here. It is not a failure and
	// contributes nothing to the exit code, because most machines will not have
	// most providers and that is the normal case.
	Absent

	// Failed means it ran and failed. Carries a captured output tail.
	Failed

	// Blocked means elevation was refused or unavailable, so it was never
	// attempted. Carries the command to run manually.
	Blocked

	// TimedOut means it exceeded its deadline and was killed. Carries the
	// deadline.
	TimedOut

	// Cancelled means Ctrl-C, or the GUI's cancel, arrived while it was
	// running.
	Cancelled
)

// String returns the outcome as the word both frontends use for it. Rendering
// may add colour or a glyph; it may not use a different word.
func (o Outcome) String() string {
	switch o {
	case Succeeded:
		return "succeeded"
	case Absent:
		return "absent"
	case Failed:
		return "failed"
	case Blocked:
		return "blocked"
	case TimedOut:
		return "timed out"
	case Cancelled:
		return "cancelled"
	case Unknown:
		return "unknown"
	default:
		return "unknown"
	}
}

// OK reports whether this outcome is one a successful run is made of, meaning
// [Succeeded] or [Absent]. Everything else, including [Unknown], is not.
func (o Outcome) OK() bool {
	return o == Succeeded || o == Absent
}

// ProviderResult is what happened to one provider during a run.
//
// The pipeline builds it, not the provider: [Provider.Apply] reports success or
// an error and nothing else, and the timing, the classification, and this record
// are assembled the same way for every provider. That is what stops a
// manifest-backed provider and a native one from disagreeing about what a
// failure looks like.
//
// Which of the optional fields carry anything depends on Outcome, and each is
// documented with the outcome it belongs to. A field that does not apply is
// left at its zero value rather than filled with a placeholder.
type ProviderResult struct {
	// Provider is the provider's ID.
	Provider string

	// Outcome is how it ended.
	Outcome Outcome

	// Updates is what was attempted. It is the same slice the plan showed the
	// user, so a failed provider reports what it was trying to do rather than
	// only that it failed.
	//
	// The whole batch shares the outcome. Package managers apply everything in
	// one command and do not report which package succeeded, so per-update
	// results would be invented rather than observed.
	Updates []Update

	// Err is the underlying cause, for [Failed], [Blocked], [TimedOut], and
	// [Cancelled]. Frontends render [ProviderResult.Outcome] and Output; Err is
	// what [Classify] read and what the journal keeps.
	Err error

	// Output is the tail of what the provider wrote to stderr, for [Failed].
	// Truncated for rendering; the journal keeps more.
	Output string

	// Command is the argv a user would run themselves to do this without
	// upall, for [Blocked]. It is argv rather than a command line because
	// there is no quoting that is correct on both cmd.exe and sh, so joining
	// it is the frontend's decision, made where the target shell is known.
	Command []string

	// Deadline is the limit that was exceeded, for [TimedOut].
	Deadline time.Duration

	// Duration is how long the provider ran, whatever the outcome.
	Duration time.Duration
}

// Result is everything that happened during a run.
//
// It is the last thing the pipeline produces and the only thing the exit code is
// derived from. Partial success is its normal shape: a run where three providers
// succeeded, one failed, and one was blocked is ordinary, and [Result.ExitCode]
// reports it as a failed run rather than an aborted one.
type Result struct {
	// Providers is one entry per provider that took part, ordered by ID.
	Providers []ProviderResult
}

// Merge collects per-provider results into a [Result].
//
// Like [Aggregate], it exists for order: apply runs providers concurrently up to
// a bound, so results arrive in completion order, and a summary that reorders
// itself between runs of the same machine is one nobody can diff. The argument
// is not modified.
func Merge(results []ProviderResult) Result {
	sorted := slices.Clone(results)
	slices.SortFunc(sorted, func(a, b ProviderResult) int {
		return strings.Compare(a.Provider, b.Provider)
	})
	return Result{Providers: sorted}
}

// Failed returns the results that did not end well, in the order they appear in
// [Result.Providers]. These are the ones a summary names.
//
// "Did not end well" is anything [Outcome.OK] rejects, so it covers blocked,
// timed out, and cancelled providers as well as failed ones. All four need
// naming; what differs is what is said about them.
func (r Result) Failed() []ProviderResult {
	var failed []ProviderResult
	for _, pr := range r.Providers {
		if !pr.Outcome.OK() {
			failed = append(failed, pr)
		}
	}
	return failed
}
