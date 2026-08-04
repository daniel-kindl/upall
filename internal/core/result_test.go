package core

import (
	"errors"
	"slices"
	"testing"
)

func result(id string, outcome Outcome) ProviderResult {
	return ProviderResult{Provider: id, Outcome: outcome}
}

func resultIDs(results []ProviderResult) []string {
	ids := make([]string, len(results))
	for i, r := range results {
		ids[i] = r.Provider
	}
	return ids
}

func TestOutcomeString(t *testing.T) {
	tests := []struct {
		outcome Outcome
		want    string
	}{
		{Succeeded, "succeeded"},
		{Absent, "absent"},
		{Failed, "failed"},
		{Blocked, "blocked"},
		{TimedOut, "timed out"},
		{Cancelled, "cancelled"},
		{Unknown, "unknown"},
		{Outcome(42), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.outcome.String(); got != tt.want {
				t.Errorf("Outcome(%d).String() = %q, want %q", int(tt.outcome), got, tt.want)
			}
		})
	}
}

func TestOutcomeOK(t *testing.T) {
	tests := []struct {
		name    string
		outcome Outcome
		want    bool
	}{
		{name: "succeeded", outcome: Succeeded, want: true},
		{
			name: "absent",
			// Most machines will not have most providers. A provider that
			// isn't there did not fail at anything.
			outcome: Absent,
			want:    true,
		},
		{name: "failed", outcome: Failed, want: false},
		{name: "blocked", outcome: Blocked, want: false},
		{name: "timed out", outcome: TimedOut, want: false},
		{name: "cancelled", outcome: Cancelled, want: false},
		{
			name: "the zero value is not a success",
			// Nobody recorded an outcome. That is a bug in upall, and it must
			// not read as "applied cleanly".
			outcome: Unknown,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.outcome.OK(); got != tt.want {
				t.Errorf("%s.OK() = %v, want %v", tt.outcome, got, tt.want)
			}
		})
	}
}

func TestMerge(t *testing.T) {
	tests := []struct {
		name    string
		results []ProviderResult
		want    []string
	}{
		{
			name: "results are ordered by provider ID, not by completion",
			// Apply runs providers concurrently up to a bound, so this is
			// the order they finished in. The summary may not reshuffle
			// itself between identical runs.
			results: []ProviderResult{
				result("winget", Succeeded),
				result("apt", Failed),
				result("scoop", Absent),
			},
			want: []string{"apt", "scoop", "winget"},
		},
		{
			name: "the same results in a different order merge identically",
			results: []ProviderResult{
				result("scoop", Absent),
				result("winget", Succeeded),
				result("apt", Failed),
			},
			want: []string{"apt", "scoop", "winget"},
		},
		{
			name:    "one provider",
			results: []ProviderResult{result("apt", Succeeded)},
			want:    []string{"apt"},
		},
		{
			name:    "no providers",
			results: nil,
			want:    []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Merge(tt.results)
			if ids := resultIDs(got.Providers); !slices.Equal(ids, tt.want) {
				t.Errorf("Providers = %v, want %v", ids, tt.want)
			}
		})
	}
}

// TestMergeDoesNotModifyItsArgument guards the same property TestAggregate does
// for plans: the pipeline may still be holding the slice it passed.
func TestMergeDoesNotModifyItsArgument(t *testing.T) {
	results := []ProviderResult{result("winget", Succeeded), result("apt", Failed)}

	Merge(results)

	if ids := resultIDs(results); !slices.Equal(ids, []string{"winget", "apt"}) {
		t.Errorf("the results argument was reordered to %v", ids)
	}
}

func TestResultFailed(t *testing.T) {
	tests := []struct {
		name   string
		result Result
		want   []string
	}{
		{
			name: "the partial-success run names only what did not succeed",
			// Three succeeded, one failed, one was blocked. This is the
			// ordinary shape of a run, not an exceptional one.
			result: Merge([]ProviderResult{
				result("apt", Succeeded),
				result("docker", Succeeded),
				result("flatpak", Blocked),
				result("snap", Failed),
				result("podman", Succeeded),
			}),
			want: []string{"flatpak", "snap"},
		},
		{
			name: "absent providers are not failures",
			result: Merge([]ProviderResult{
				result("apt", Absent),
				result("dnf", Absent),
				result("winget", Succeeded),
			}),
			want: nil,
		},
		{
			name: "timed out and cancelled are named too",
			// All four non-OK outcomes need naming in the summary. What
			// differs is what is said about them.
			result: Merge([]ProviderResult{
				result("apt", TimedOut),
				result("docker", Cancelled),
				result("winget", Succeeded),
			}),
			want: []string{"apt", "docker"},
		},
		{
			name:   "an all-succeeded run names nothing",
			result: Merge([]ProviderResult{result("apt", Succeeded), result("winget", Succeeded)}),
			want:   nil,
		},
		{
			name:   "an empty run names nothing",
			result: Result{},
			want:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if ids := resultIDs(tt.result.Failed()); !slices.Equal(ids, tt.want) {
				t.Errorf("Failed() = %v, want %v", ids, tt.want)
			}
		})
	}
}

// TestProviderResultCarriesItsDetail checks that the fields the error taxonomy
// requires survive a merge, since a summary that lost the stderr tail or the
// manual command would satisfy every other test here.
func TestProviderResultCarriesItsDetail(t *testing.T) {
	cause := errors.New("E: Could not get lock /var/lib/dpkg/lock-frontend")
	failed := ProviderResult{
		Provider: "apt",
		Outcome:  Failed,
		Updates:  []Update{{Name: "curl", ID: "curl", Installed: "8.5.0", Available: "8.6.0"}},
		Err:      cause,
		Output:   "E: Could not get lock /var/lib/dpkg/lock-frontend",
	}
	blocked := ProviderResult{
		Provider: "snap",
		Outcome:  Blocked,
		Err:      ErrNeedsElevation,
		Command:  []string{"sudo", "snap", "refresh"},
	}

	got := Merge([]ProviderResult{blocked, failed}).Failed()
	if len(got) != 2 {
		t.Fatalf("Failed() returned %d results, want 2", len(got))
	}

	if !errors.Is(got[0].Err, cause) {
		t.Errorf("apt lost its cause: Err = %v", got[0].Err)
	}
	if got[0].Output == "" {
		t.Error("apt lost its captured output, which is what the summary prints")
	}
	if len(got[0].Updates) != 1 {
		t.Errorf("apt lost what it was attempting: Updates = %v", got[0].Updates)
	}

	if !errors.Is(got[1].Err, ErrNeedsElevation) {
		t.Errorf("snap lost its cause: Err = %v", got[1].Err)
	}
	if !slices.Equal(got[1].Command, []string{"sudo", "snap", "refresh"}) {
		t.Errorf("snap lost the command the user needs to run: %v", got[1].Command)
	}
}
