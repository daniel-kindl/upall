package core

import (
	"slices"
	"testing"
)

// providerPlan is shorthand for a plan with n updates, since most cases here
// care about which provider and how many rather than about version strings.
func providerPlan(id string, n int, needsElevation bool) ProviderPlan {
	updates := make([]Update, n)
	for i := range updates {
		updates[i] = Update{Name: id + "-package", ID: id + "-package", Installed: "1.0", Available: "1.1"}
	}
	return ProviderPlan{Provider: id, Updates: updates, NeedsElevation: needsElevation}
}

func providerIDs(plans []ProviderPlan) []string {
	ids := make([]string, len(plans))
	for i, p := range plans {
		ids[i] = p.Provider
	}
	return ids
}

func TestAggregate(t *testing.T) {
	tests := []struct {
		name   string
		plans  []ProviderPlan
		absent []string

		wantProviders []string
		wantAbsent    []string
	}{
		{
			name: "providers are ordered by ID whatever order they finished in",
			// Detect and plan run concurrently, so this is the order the
			// package managers happened to return in, not one anybody chose.
			plans: []ProviderPlan{
				providerPlan("winget", 2, false),
				providerPlan("apt", 1, true),
				providerPlan("scoop", 0, false),
			},
			wantProviders: []string{"apt", "scoop", "winget"},
		},
		{
			name: "the same plans in a different order aggregate identically",
			plans: []ProviderPlan{
				providerPlan("scoop", 0, false),
				providerPlan("winget", 2, false),
				providerPlan("apt", 1, true),
			},
			wantProviders: []string{"apt", "scoop", "winget"},
		},
		{
			name:          "absent providers are sorted too",
			plans:         []ProviderPlan{providerPlan("apt", 1, true)},
			absent:        []string{"snap", "dnf", "flatpak"},
			wantProviders: []string{"apt"},
			wantAbsent:    []string{"dnf", "flatpak", "snap"},
		},
		{
			name: "a machine with nothing installed aggregates cleanly",
			// Not an error. It is the answer to the question that was asked.
			absent:        []string{"winget", "scoop"},
			wantProviders: nil,
			wantAbsent:    []string{"scoop", "winget"},
		},
		{
			name:          "nothing at all is a valid plan",
			wantProviders: nil,
			wantAbsent:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Aggregate(tt.plans, tt.absent)

			if ids := providerIDs(got.Providers); !slices.Equal(ids, tt.wantProviders) {
				t.Errorf("Providers = %v, want %v", ids, tt.wantProviders)
			}
			if !slices.Equal(got.Absent, tt.wantAbsent) {
				t.Errorf("Absent = %v, want %v", got.Absent, tt.wantAbsent)
			}
		})
	}
}

// TestAggregateDoesNotModifyItsArguments guards the property that makes
// Aggregate safe to call on a slice the pipeline is still holding.
func TestAggregateDoesNotModifyItsArguments(t *testing.T) {
	plans := []ProviderPlan{
		providerPlan("winget", 1, false),
		providerPlan("apt", 1, false),
	}
	absent := []string{"snap", "dnf"}

	Aggregate(plans, absent)

	if ids := providerIDs(plans); !slices.Equal(ids, []string{"winget", "apt"}) {
		t.Errorf("the plans argument was reordered to %v", ids)
	}
	if !slices.Equal(absent, []string{"snap", "dnf"}) {
		t.Errorf("the absent argument was reordered to %v", absent)
	}
}

func TestPlanCounts(t *testing.T) {
	tests := []struct {
		name  string
		plan  Plan
		empty bool
		count int
	}{
		{
			name:  "updates across several providers are totalled",
			plan:  Aggregate([]ProviderPlan{providerPlan("winget", 2, false), providerPlan("scoop", 3, false)}, nil),
			empty: false,
			count: 5,
		},
		{
			name: "everything present and already current is an empty plan",
			// The providers were found and asked. There is simply nothing to
			// do, which is a successful run rather than a failed one.
			plan:  Aggregate([]ProviderPlan{providerPlan("winget", 0, false), providerPlan("scoop", 0, false)}, nil),
			empty: true,
			count: 0,
		},
		{
			name:  "a plan with only absent providers is empty",
			plan:  Aggregate(nil, []string{"apt", "dnf"}),
			empty: true,
			count: 0,
		},
		{
			name:  "the zero Plan is empty",
			plan:  Plan{},
			empty: true,
			count: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.plan.Count(); got != tt.count {
				t.Errorf("Count() = %d, want %d", got, tt.count)
			}
			if got := tt.plan.Empty(); got != tt.empty {
				t.Errorf("Empty() = %v, want %v", got, tt.empty)
			}
		})
	}
}

func TestPlanElevation(t *testing.T) {
	tests := []struct {
		name         string
		plan         Plan
		wantElevated []string
		wantNeeds    bool
	}{
		{
			name: "only the providers that declared it are listed",
			plan: Aggregate([]ProviderPlan{
				providerPlan("apt", 2, true),
				providerPlan("scoop", 1, false),
				providerPlan("snap", 1, true),
			}, nil),
			wantElevated: []string{"apt", "snap"},
			wantNeeds:    true,
		},
		{
			name: "a provider that needs elevation but has nothing to do is not listed",
			// It will not be run, so it will not be elevated, and saying it
			// would asks the user to approve an escalation that is not going
			// to happen.
			plan:         Aggregate([]ProviderPlan{providerPlan("apt", 0, true), providerPlan("scoop", 2, false)}, nil),
			wantElevated: nil,
			wantNeeds:    false,
		},
		{
			name:         "nothing needs elevation",
			plan:         Aggregate([]ProviderPlan{providerPlan("scoop", 1, false)}, nil),
			wantElevated: nil,
			wantNeeds:    false,
		},
		{
			name:         "the elevated list follows the aggregated order",
			plan:         Aggregate([]ProviderPlan{providerPlan("snap", 1, true), providerPlan("apt", 1, true)}, nil),
			wantElevated: []string{"apt", "snap"},
			wantNeeds:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.plan.Elevated(); !slices.Equal(got, tt.wantElevated) {
				t.Errorf("Elevated() = %v, want %v", got, tt.wantElevated)
			}
			if got := tt.plan.NeedsElevation(); got != tt.wantNeeds {
				t.Errorf("NeedsElevation() = %v, want %v", got, tt.wantNeeds)
			}
		})
	}
}
