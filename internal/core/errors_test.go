package core

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want Outcome
	}{
		{
			name: "no error is a success",
			err:  nil,
			want: Succeeded,
		},
		{
			name: "a cancelled context",
			err:  context.Canceled,
			want: Cancelled,
		},
		{
			name: "an exceeded deadline",
			err:  context.DeadlineExceeded,
			want: TimedOut,
		},
		{
			name: "the elevation sentinel",
			err:  ErrNeedsElevation,
			want: Blocked,
		},
		{
			name: "anything else failed",
			err:  errors.New("exit status 100"),
			want: Failed,
		},
		{
			name: "a wrapped cancellation is still a cancellation",
			// Providers add context to what they return; the classification
			// has to see through it, which is why this matches with errors.Is
			// rather than by comparison.
			err:  fmt.Errorf("apt: %w", context.Canceled),
			want: Cancelled,
		},
		{
			name: "a wrapped deadline is still a timeout",
			err:  fmt.Errorf("running winget: %w", context.DeadlineExceeded),
			want: TimedOut,
		},
		{
			name: "a wrapped elevation sentinel is still blocked",
			err:  fmt.Errorf("snap refresh: %w", ErrNeedsElevation),
			want: Blocked,
		},
		{
			name: "wrapped twice is still found",
			err:  fmt.Errorf("apply: %w", fmt.Errorf("snap: %w", ErrNeedsElevation)),
			want: Blocked,
		},
		{
			name: "an error that merely mentions elevation is not the sentinel",
			// Classification is by identity, never by reading the message.
			// Matching on strings is how a provider's error text becomes an
			// interface nobody agreed to.
			err:  errors.New("needs elevation"),
			want: Failed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Classify(tt.err); got != tt.want {
				t.Errorf("Classify(%v) = %s, want %s", tt.err, got, tt.want)
			}
		})
	}
}

// TestClassifyNeverReportsAbsent records that "not installed" does not travel as
// an error. Detect returns false for it, and the pipeline records Absent
// directly, so no error value maps here.
func TestClassifyNeverReportsAbsent(t *testing.T) {
	errs := []error{
		nil,
		context.Canceled,
		context.DeadlineExceeded,
		ErrNeedsElevation,
		errors.New("command not found"),
		fmt.Errorf("winget: %w", errors.New("executable file not found in %PATH%")),
	}

	for _, err := range errs {
		if got := Classify(err); got == Absent {
			t.Errorf("Classify(%v) = %s; absence is Detect returning false, not an error", err, got)
		}
	}
}
