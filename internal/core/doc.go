// Package core is the vocabulary the whole of upall speaks: what an update is,
// what a provider is, what a run produced, and what the process should exit
// with.
//
// Everything else in upall depends on this package and it depends on nothing.
// It imports only the standard library, and only the parts of it that compute:
// no os, no os/exec, no printing, no files, no network. That is enforced by a
// test rather than by review, because it is the property the rest of the design
// rests on. Types that cannot perform I/O are types the CLI and the GUI can
// share without either one bending to the other, which is what makes
// internal/gui a frontend rather than a second implementation.
//
// # The types
//
// An [Update] is one thing that could be installed. It is grouped by the
// provider that found it into a [ProviderPlan], and those are aggregated into
// the one [Plan] a run produces. A [Provider] is the thing that finds and
// installs them; it is an interface, and a TOML manifest wrapped in an adapter
// satisfies it exactly as a hand-written Go provider does. [Platform] and
// [PlatformSet] answer whether a given provider can run on this machine at all.
//
// After the work is done, each provider's fate is a [ProviderResult] carrying an
// [Outcome], and those merge into the one [Result] a run produces. [ExitCode] is
// derived from that and from nothing else.
//
// So the plan side and the result side mirror each other:
//
//	Update  →  ProviderPlan    →  Plan    (via Aggregate)
//	           ProviderResult  →  Result  (via Merge)
//	                              Result  →  ExitCode
//
// Both [Aggregate] and [Merge] sort by provider ID. Detect, plan, and apply all
// run concurrently across providers, so these arrive in whatever order the
// package managers finished in, and a summary that reshuffles itself between
// identical runs is one nobody can diff or test.
//
// # The lifecycle of an Update
//
// An Update is described once and never changed. Following one through a run:
//
//  1. discover and detect. The provider that will find it is selected by
//     [PlatformSet.Supports] against [Current], then asked
//     [Provider.Detect]. A provider that says no is recorded in [Plan.Absent]
//     and the Update is never created.
//  2. plan. [Provider.Plan] returns it, along with everything else that
//     provider would update. The pipeline attaches the provider's ID and
//     elevation requirement, making a [ProviderPlan].
//  3. aggregate. [Aggregate] collects those into a [Plan], in a fixed order.
//  4. render and confirm. A frontend shows it. [Plan.Elevated] is what marks the
//     providers that will need admin or root, and it is shown before the prompt,
//     never after.
//  5. apply. The same value, unmodified, is handed back to [Provider.Apply] with
//     the rest of its provider's batch.
//  6. report. It appears in [ProviderResult.Updates] as part of what was
//     attempted. The [Outcome] belongs to the batch, not to it.
//
// It carries no status field, and that is the substantive point. The tools upall
// drives — winget upgrade --all, apt upgrade -y — update everything in one
// command and do not report which package succeeded. A per-update outcome would
// therefore be inferred rather than observed, and inferring it is how a tool
// starts reporting things that did not happen.
//
// # What is not here
//
// No progress events: those are the pipeline's, and arrive at M5. No
// confirmation prompt: the confirmer is an interface the pipeline takes and a
// frontend satisfies, so the core never learns whether it got a terminal or a
// dialog. No rendering of any kind, for the reason at the top.
package core
