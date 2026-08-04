package core

import "testing"

func TestResultExitCode(t *testing.T) {
	tests := []struct {
		name   string
		result Result
		want   ExitCode
	}{
		{
			name: "everything succeeded",
			result: Merge([]ProviderResult{
				result("apt", Succeeded),
				result("docker", Succeeded),
			}),
			want: ExitOK,
		},
		{
			name: "a machine with none of the providers installed",
			// Nothing to update is a successful answer to the question, not a
			// failure to answer it.
			result: Merge([]ProviderResult{
				result("winget", Absent),
				result("scoop", Absent),
				result("chocolatey", Absent),
			}),
			want: ExitOK,
		},
		{
			name: "a mix of succeeded and absent",
			result: Merge([]ProviderResult{
				result("apt", Succeeded),
				result("dnf", Absent),
			}),
			want: ExitOK,
		},
		{
			name: "no providers took part at all",
			// Nothing ran, nothing failed. The empty run exits 0.
			result: Result{},
			want:   ExitOK,
		},
		{
			name: "one failure among successes fails the run",
			// The others still ran. A failed provider makes a failed run, not
			// an aborted one.
			result: Merge([]ProviderResult{
				result("apt", Succeeded),
				result("docker", Failed),
				result("snap", Succeeded),
			}),
			want: ExitFailure,
		},
		{
			name: "every provider failed",
			result: Merge([]ProviderResult{
				result("apt", Failed),
				result("docker", Failed),
			}),
			want: ExitFailure,
		},
		{
			name: "blocked counts as a failure",
			// It never ran, but it is still work the user asked for that did
			// not get done.
			result: Merge([]ProviderResult{
				result("apt", Blocked),
				result("docker", Succeeded),
			}),
			want: ExitFailure,
		},
		{
			name: "timed out counts as a failure",
			result: Merge([]ProviderResult{
				result("apt", TimedOut),
				result("docker", Succeeded),
			}),
			want: ExitFailure,
		},
		{
			name: "a result nobody filled in is a failure, not a success",
			// Unknown is the zero value. Reaching this means a bug in upall,
			// and it must not report that updates were applied.
			result: Merge([]ProviderResult{
				result("apt", Succeeded),
				{Provider: "docker"},
			}),
			want: ExitFailure,
		},
		{
			name: "cancelled interrupts the run",
			result: Merge([]ProviderResult{
				result("apt", Succeeded),
				result("docker", Cancelled),
			}),
			want: ExitInterrupted,
		},
		{
			name: "interrupted beats failed",
			// Ctrl-C arrived and the results are incomplete. Reporting a
			// complete failure would misdescribe what happened.
			result: Merge([]ProviderResult{
				result("apt", Failed),
				result("docker", Cancelled),
				result("snap", Succeeded),
			}),
			want: ExitInterrupted,
		},
		{
			name: "interrupted beats failed whichever order they merged in",
			// Guards against the answer depending on which provider the loop
			// happens to reach first.
			result: Merge([]ProviderResult{
				result("aaa-cancelled", Cancelled),
				result("zzz-failed", Failed),
			}),
			want: ExitInterrupted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.ExitCode(); got != tt.want {
				t.Errorf("ExitCode() = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestExitCodeValues pins the numbers themselves. They are a public interface
// under semver that scripts depend on, so changing one has to be a deliberate
// edit to a failing test rather than a quiet consequence of reordering
// constants.
func TestExitCodeValues(t *testing.T) {
	tests := []struct {
		name string
		code ExitCode
		want int
	}{
		{name: "ok", code: ExitOK, want: 0},
		{name: "failure", code: ExitFailure, want: 1},
		{name: "usage", code: ExitUsage, want: 2},
		{name: "interrupted", code: ExitInterrupted, want: 130},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if int(tt.code) != tt.want {
				t.Errorf("%s = %d, want %d", tt.name, int(tt.code), tt.want)
			}
		})
	}
}
