package core

import (
	"slices"
	"strings"
)

// ProviderPlan is what one provider would update, and is the unit the plan is
// rendered and confirmed in.
//
// A ProviderPlan with no updates means the provider is installed and already
// current, which is a state worth distinguishing from not having it at all. The
// second case is not a ProviderPlan; it is an entry in [Plan.Absent].
type ProviderPlan struct {
	// Provider is the provider's ID, such as "winget" or "apt".
	Provider string

	// Updates is what it would install, empty if it is already current.
	Updates []Update

	// NeedsElevation reports whether applying these updates will require admin
	// or root. Planning never does. It is copied from the provider at plan time
	// rather than asked for again later, so that what the user was shown and
	// what gets elevated cannot diverge.
	NeedsElevation bool
}

// Plan is everything a run would do, aggregated across providers.
//
// It is produced by [Aggregate], rendered by a frontend, and answered by the
// user before anything changes. Nothing in it can act: a Plan describes work and
// holds no reference to the providers that would perform it.
type Plan struct {
	// Providers is one entry per detected provider, ordered by ID.
	//
	// Providers with nothing to update are kept rather than dropped, because
	// "checked apt, already current" is information the user asked for by
	// running upall, and a frontend that would rather not show it can filter
	// on [ProviderPlan.Updates] being empty.
	Providers []ProviderPlan

	// Absent is the IDs of providers that are not installed on this machine,
	// ordered alphabetically.
	//
	// They are held apart from Providers rather than flagged within it because
	// most machines have most providers absent, and that has to stay silent
	// unless the user asks. Separating them means a renderer omits them by not
	// reading this field, rather than by filtering the field it does read.
	Absent []string
}

// Aggregate builds a [Plan] from the per-provider plans and the IDs of the
// providers found absent.
//
// Its job is order. Detect and plan run concurrently across providers, so these
// arrive in whatever order they finished, which is a function of how fast each
// package manager felt like being. Sorting by ID here means the same machine in
// the same state renders the same plan every time, which is what makes the
// output diffable and the tests deterministic.
//
// The arguments are not modified; both slices are copied before sorting.
func Aggregate(plans []ProviderPlan, absent []string) Plan {
	sortedPlans := slices.Clone(plans)
	slices.SortFunc(sortedPlans, func(a, b ProviderPlan) int {
		return strings.Compare(a.Provider, b.Provider)
	})

	sortedAbsent := slices.Clone(absent)
	slices.Sort(sortedAbsent)

	return Plan{Providers: sortedPlans, Absent: sortedAbsent}
}

// Empty reports whether the plan would change nothing.
//
// A plan can be empty and still have found several providers: they were all
// present, all asked, and all already current. That is a successful run with
// nothing to do, not a failed one.
func (p Plan) Empty() bool {
	return p.Count() == 0
}

// Count is the total number of updates across every provider.
func (p Plan) Count() int {
	total := 0
	for _, pp := range p.Providers {
		total += len(pp.Updates)
	}
	return total
}

// NeedsElevation reports whether applying this plan will require admin or root
// anywhere.
func (p Plan) NeedsElevation() bool {
	return len(p.Elevated()) > 0
}

// Elevated is the IDs of the providers that will be elevated if this plan is
// applied, ordered as [Plan.Providers] is.
//
// A provider is listed only if it declared the need *and* has something to
// update. One that is already current will not be run, so it will not be
// elevated, and saying it would be would ask the user to approve a privilege
// escalation that is not going to happen. Both frontends must show this before
// the confirmation prompt; see ADR-0003.
func (p Plan) Elevated() []string {
	var ids []string
	for _, pp := range p.Providers {
		if pp.NeedsElevation && len(pp.Updates) > 0 {
			ids = append(ids, pp.Provider)
		}
	}
	return ids
}
