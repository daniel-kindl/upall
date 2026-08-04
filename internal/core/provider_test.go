package core

import (
	"context"
	"errors"
	"testing"
)

// stubProvider is the smallest thing that satisfies [Provider]. It exists to
// prove the interface is implementable and to stand in for one in the tests
// below. The registry's shared fakes arrive with internal/provider at M4; this
// is deliberately not that.
type stubProvider struct {
	id             string
	platforms      PlatformSet
	needsElevation bool

	present  bool
	updates  []Update
	applyErr error
}

func (s stubProvider) ID() string             { return s.id }
func (s stubProvider) Platforms() PlatformSet { return s.platforms }
func (s stubProvider) NeedsElevation() bool   { return s.needsElevation }

func (s stubProvider) Detect(context.Context) (bool, error) { return s.present, nil }
func (s stubProvider) Plan(context.Context) ([]Update, error) {
	return s.updates, nil
}
func (s stubProvider) Apply(context.Context, []Update) error { return s.applyErr }

// A native provider satisfies Provider directly. At M4 a manifest loaded into an
// adapter will satisfy it the same way, and nothing downstream will be able to
// tell the two apart. See ADR-0002.
var _ Provider = stubProvider{}

func TestProviderGating(t *testing.T) {
	providers := []Provider{
		stubProvider{id: "winget", platforms: PlatformSet{Windows}},
		stubProvider{id: "apt", platforms: PlatformSet{Linux}},
		stubProvider{id: "docker", platforms: PlatformSet{Windows, Linux}},
		stubProvider{id: "misconfigured"},
	}

	tests := []struct {
		name string
		host Platform
		want []string
	}{
		{name: "on windows", host: Windows, want: []string{"winget", "docker"}},
		{name: "on linux", host: Linux, want: []string{"apt", "docker"}},
		{
			name: "on darwin, where no provider claims to run yet",
			host: Darwin,
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			for _, p := range providers {
				if p.Platforms().Supports(tt.host) {
					got = append(got, p.ID())
				}
			}

			if len(got) != len(tt.want) {
				t.Fatalf("selected %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("selected %v, want %v", got, tt.want)
					break
				}
			}
		})
	}
}

// TestDetectingAnAbsentProviderIsNotAnError pins the invariant that a machine
// missing a tool is the normal case. It is asserted here, on the interface,
// because every provider written from M4 onwards has to honour it.
func TestDetectingAnAbsentProviderIsNotAnError(t *testing.T) {
	absent := stubProvider{id: "winget", platforms: PlatformSet{Windows}, present: false}

	present, err := absent.Detect(context.Background())
	if err != nil {
		t.Errorf("Detect returned %v; not installed is an answer, not a failure", err)
	}
	if present {
		t.Error("Detect reported the provider present")
	}
}

// TestARunComposes walks a whole run through the domain types the way the
// pipeline will at M5 and M6: gate by platform, detect, plan, aggregate,
// apply, classify, merge, exit. It is here to prove the vocabulary fits
// together, which no single-type test can show.
func TestARunComposes(t *testing.T) {
	ctx := context.Background()
	const host = Linux

	providers := []Provider{
		stubProvider{
			id:        "apt",
			platforms: PlatformSet{Linux},
			present:   true,
			updates: []Update{
				{Name: "curl", ID: "curl", Installed: "8.5.0", Available: "8.6.0"},
				{Name: "vim", ID: "vim", Installed: "9.1", Available: "9.2"},
			},
			needsElevation: true,
		},
		stubProvider{
			id:        "docker",
			platforms: PlatformSet{Windows, Linux},
			present:   true,
			updates:   []Update{{Name: "nginx", ID: "nginx:latest"}},
			applyErr:  errors.New("failed to pull nginx:latest"),
		},
		// Present, checked, already current.
		stubProvider{id: "flatpak", platforms: PlatformSet{Linux}, present: true},
		// Installed, but this is not a Linux tool.
		stubProvider{id: "winget", platforms: PlatformSet{Windows}, present: true},
		// A Linux tool this machine does not have.
		stubProvider{id: "dnf", platforms: PlatformSet{Linux}, present: false},
	}

	// discover, detect, plan.
	var plans []ProviderPlan
	var absent []string
	var planned []Provider
	for _, p := range providers {
		if !p.Platforms().Supports(host) {
			continue
		}

		present, err := p.Detect(ctx)
		if err != nil {
			t.Fatalf("%s: Detect: %v", p.ID(), err)
		}
		if !present {
			absent = append(absent, p.ID())
			continue
		}

		updates, err := p.Plan(ctx)
		if err != nil {
			t.Fatalf("%s: Plan: %v", p.ID(), err)
		}
		plans = append(plans, ProviderPlan{
			Provider:       p.ID(),
			Updates:        updates,
			NeedsElevation: p.NeedsElevation(),
		})
		planned = append(planned, p)
	}

	// aggregate.
	plan := Aggregate(plans, absent)

	if plan.Count() != 3 {
		t.Errorf("plan Count() = %d, want 3", plan.Count())
	}
	if plan.Empty() {
		t.Error("plan reports empty with three updates in it")
	}
	if got := plan.Elevated(); len(got) != 1 || got[0] != "apt" {
		t.Errorf("plan Elevated() = %v, want [apt]", got)
	}
	if len(plan.Absent) != 1 || plan.Absent[0] != "dnf" {
		t.Errorf("plan Absent = %v, want [dnf]", plan.Absent)
	}
	if len(plan.Providers) != 3 {
		t.Fatalf("plan has %d providers, want 3 (apt, docker, flatpak)", len(plan.Providers))
	}

	// apply, report.
	var results []ProviderResult
	for _, p := range planned {
		var updates []Update
		for _, pp := range plan.Providers {
			if pp.Provider == p.ID() {
				updates = pp.Updates
			}
		}

		err := p.Apply(ctx, updates)
		results = append(results, ProviderResult{
			Provider: p.ID(),
			Outcome:  Classify(err),
			Updates:  updates,
			Err:      err,
		})
	}

	// merge.
	run := Merge(results)

	if failed := run.Failed(); len(failed) != 1 || failed[0].Provider != "docker" {
		t.Errorf("run Failed() = %v, want only docker", resultIDs(failed))
	}
	if got := run.ExitCode(); got != ExitFailure {
		t.Errorf("run ExitCode() = %d, want %d: one provider failed and the others did not", got, ExitFailure)
	}
}
