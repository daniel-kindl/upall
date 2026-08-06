package exec

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/daniel-kindl/upall/internal/core"
)

func TestRunCapturesTheStreamsSeparately(t *testing.T) {
	tests := []struct {
		name       string
		mode       string
		args       []string
		wantStdout string
		wantStderr string
	}{
		{
			name:       "standard output",
			mode:       "echo",
			args:       []string{"hello"},
			wantStdout: "hello",
		},
		{
			name:       "standard error",
			mode:       "warn",
			args:       []string{"trouble"},
			wantStderr: "trouble",
		},
		{
			// The two must not be interleaved into one stream. A parser reads
			// stdout and a failure report shows stderr, and neither survives
			// the other's text being mixed into it.
			name:       "both at once, kept apart",
			mode:       "both",
			args:       []string{"to stdout", "to stderr"},
			wantStdout: "to stdout",
			wantStderr: "to stderr",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := New(nil).Run(t.Context(), helperCommand(t, tt.mode, tt.args...))
			if err != nil {
				t.Fatalf("Run() returned %v, want no error", err)
			}
			if got := string(out.Stdout); got != tt.wantStdout {
				t.Errorf("Stdout = %q, want %q", got, tt.wantStdout)
			}
			if got := string(out.Stderr); got != tt.wantStderr {
				t.Errorf("Stderr = %q, want %q", got, tt.wantStderr)
			}
			if out.ExitCode != 0 {
				t.Errorf("ExitCode = %d, want 0", out.ExitCode)
			}
			if out.Truncated {
				t.Error("Truncated = true, want false; nothing here approaches MaxCapture")
			}
		})
	}
}

func TestRunReportsANonZeroExitAsAnError(t *testing.T) {
	out, err := New(nil).Run(t.Context(), helperCommand(t, "exit", "3", "it went wrong"))

	var exit *ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("Run() returned %[1]T (%[1]v), want an *ExitError", err)
	}
	if exit.Code != 3 {
		t.Errorf("ExitError.Code = %d, want 3", exit.Code)
	}
	if got := string(exit.Stderr); got != "it went wrong" {
		// The error carries stderr because core.Provider.Apply returns an error
		// and nothing else, so this field is the only route by which what the
		// command said reaches the user.
		t.Errorf("ExitError.Stderr = %q, want %q", got, "it went wrong")
	}

	// The output is returned alongside the error, not instead of it.
	if out.ExitCode != 3 {
		t.Errorf("Output.ExitCode = %d, want 3", out.ExitCode)
	}
	if got := string(out.Stderr); got != "it went wrong" {
		t.Errorf("Output.Stderr = %q, want %q", got, "it went wrong")
	}
	if out.Duration <= 0 {
		t.Error("Output.Duration = 0, want the time a failed command still took")
	}
}

func TestRunRejectsACommandWithNoArgv(t *testing.T) {
	out, err := New(nil).Run(t.Context(), Command{})

	if !errors.Is(err, ErrNoCommand) {
		t.Errorf("Run() returned %v, want ErrNoCommand", err)
	}
	if out.ExitCode != -1 {
		t.Errorf("ExitCode = %d, want -1; nothing ran, so there is no status", out.ExitCode)
	}
}

// TestRunReportsAMissingProgram covers the case every provider's Detect depends
// on. It has to be distinguishable from a command that ran and failed, because
// one means "not installed" and the other means "broken", and only the second
// is a failure.
func TestRunReportsAMissingProgram(t *testing.T) {
	cmd := Command{Argv: []string{"upall-no-such-program-exists"}}

	out, err := New(nil).Run(t.Context(), cmd)

	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Run() returned %v, want an error wrapping ErrNotFound", err)
	}
	var exit *ExitError
	if errors.As(err, &exit) {
		t.Error("a program that never started reported an *ExitError; it has no exit status to report")
	}
	if out.ExitCode != -1 {
		t.Errorf("ExitCode = %d, want -1", out.ExitCode)
	}
}

func TestRunHonoursDir(t *testing.T) {
	// Both sides go through EvalSymlinks because a temporary directory is
	// reached through one on more than one platform, and the child reports
	// where it actually is.
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolving the temporary directory: %v", err)
	}

	cmd := helperCommand(t, "pwd")
	cmd.Dir = dir

	out, err := New(nil).Run(t.Context(), cmd)
	if err != nil {
		t.Fatalf("Run() returned %v, want no error", err)
	}

	got, err := filepath.EvalSymlinks(string(out.Stdout))
	if err != nil {
		t.Fatalf("resolving what the command reported: %v", err)
	}
	if got != dir {
		t.Errorf("the command ran in %q, want %q", got, dir)
	}
}

func TestRunOverlaysEnvRatherThanReplacingIt(t *testing.T) {
	t.Run("an addition reaches the command", func(t *testing.T) {
		cmd := helperCommand(t, "env", "UPALL_TEST_ADDED")
		cmd.Env = helperEnviron("UPALL_TEST_ADDED=here")

		out, err := New(nil).Run(t.Context(), cmd)
		if err != nil {
			t.Fatalf("Run() returned %v, want no error", err)
		}
		if got := string(out.Stdout); got != "here" {
			t.Errorf("the command saw UPALL_TEST_ADDED=%q, want %q", got, "here")
		}
	})

	t.Run("the inherited environment survives", func(t *testing.T) {
		// PATH is the one that matters: a provider whose PATH was discarded
		// cannot find the tools it shells out to, so replacing rather than
		// overlaying would break every provider that set a variable.
		t.Setenv("UPALL_TEST_INHERITED", "from the parent")

		cmd := helperCommand(t, "env", "UPALL_TEST_INHERITED")
		cmd.Env = helperEnviron("UPALL_TEST_UNRELATED=1")

		out, err := New(nil).Run(t.Context(), cmd)
		if err != nil {
			t.Fatalf("Run() returned %v, want no error", err)
		}
		if got := string(out.Stdout); got != "from the parent" {
			t.Errorf("the command saw UPALL_TEST_INHERITED=%q, want %q", got, "from the parent")
		}
	})

	t.Run("an override wins over the inherited value", func(t *testing.T) {
		t.Setenv("UPALL_TEST_OVERRIDDEN", "from the parent")

		cmd := helperCommand(t, "env", "UPALL_TEST_OVERRIDDEN")
		cmd.Env = helperEnviron("UPALL_TEST_OVERRIDDEN=from the command")

		out, err := New(nil).Run(t.Context(), cmd)
		if err != nil {
			t.Fatalf("Run() returned %v, want no error", err)
		}
		if got := string(out.Stdout); got != "from the command" {
			t.Errorf("the command saw UPALL_TEST_OVERRIDDEN=%q, want %q", got, "from the command")
		}
	})
}

// TestRunGivesTheCommandNoStdin is the guarantee that an unattended run cannot
// hang on a question. A package manager that decides to prompt must read EOF
// and fail, because there is nobody to answer and no way to see it asking.
func TestRunGivesTheCommandNoStdin(t *testing.T) {
	out, err := New(nil).Run(t.Context(), helperCommand(t, "stdin"))
	if err != nil {
		t.Fatalf("Run() returned %v, want no error", err)
	}

	if got := string(out.Stdout); !strings.Contains(got, "read 0 bytes") {
		t.Errorf("reading stdin gave %q, want an immediate end of file", got)
	}
}

func TestRunEnforcesTheTimeout(t *testing.T) {
	// Comfortably longer than the second or so it takes to re-execute a
	// race-instrumented test binary, so that the deadline lands on a command
	// that is genuinely running rather than on one still starting up.
	const deadline = 3 * time.Second

	cmd := helperCommand(t, "sleep", "30s")
	cmd.Timeout = deadline

	started := time.Now()
	_, err := New(nil).Run(t.Context(), cmd)
	elapsed := time.Since(started)

	var timeout *TimeoutError
	if !errors.As(err, &timeout) {
		t.Fatalf("Run() returned %[1]T (%[1]v), want a *TimeoutError", err)
	}
	if timeout.Deadline != deadline {
		t.Errorf("TimeoutError.Deadline = %s, want %s", timeout.Deadline, deadline)
	}
	if elapsed < deadline {
		t.Errorf("Run() gave up after %s, before the %s deadline it was given", elapsed, deadline)
	}
	if elapsed > 30*time.Second {
		t.Errorf("Run() took %s; the command was not killed at its deadline", elapsed)
	}

	// The unwrapping is the contract with core, not a convenience.
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Error("a TimeoutError does not unwrap to context.DeadlineExceeded, so core.Classify cannot see it as a timeout")
	}
}

func TestRunReportsCancellation(t *testing.T) {
	ready := filepath.Join(t.TempDir(), "running")
	ctx, cancel := context.WithCancel(t.Context())

	// Cancel only once the command is genuinely running. Cancelling on a timer
	// would usually kill a child still starting up, which proves far less.
	go awaitStart(t, ready, cancel)

	started := time.Now()
	_, err := New(nil).Run(ctx, helperCommand(t, "sleep", "30s", ready))
	elapsed := time.Since(started)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Run() returned %v, want an error wrapping context.Canceled", err)
	}
	if elapsed > 30*time.Second {
		t.Errorf("Run() took %s; cancelling the context did not stop the command", elapsed)
	}
}

// TestCancellationBeatsAnExpiredDeadline pins the ordering in classify. When
// Ctrl-C lands on a command that also had a per-command timeout, both contexts
// are done at once, and only checking the parent first tells them apart. The
// difference is visible to the user as exit 130 rather than exit 1.
func TestCancellationBeatsAnExpiredDeadline(t *testing.T) {
	ready := filepath.Join(t.TempDir(), "running")
	ctx, cancel := context.WithCancel(t.Context())
	go awaitStart(t, ready, cancel)

	cmd := helperCommand(t, "sleep", "30s", ready)
	// Long enough that the cancellation above is what stops the command, and
	// short enough that a run which somehow waited would hit it rather than
	// hanging the suite.
	cmd.Timeout = 10 * time.Second

	_, err := New(nil).Run(ctx, cmd)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Run() returned %v, want an error wrapping context.Canceled", err)
	}
	var timeout *TimeoutError
	if errors.As(err, &timeout) {
		t.Error("an interrupted command reported a timeout; the run would exit 1 where the contract says 130")
	}
}

// TestErrorsClassifyTheWayCoreExpects checks the seam between this package and
// core from the outside.
//
// The package itself imports nothing from this module and a test enforces that.
// This file does import core, deliberately: the agreement being checked is the
// one between the two packages, and neither one alone can demonstrate it. The
// guard reads only non-test files for exactly this reason.
func TestErrorsClassifyTheWayCoreExpects(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want core.Outcome
	}{
		{
			name: "a timeout",
			err:  &TimeoutError{Argv: []string{"apt"}, Deadline: time.Second},
			want: core.TimedOut,
		},
		{
			name: "a non-zero exit",
			err:  &ExitError{Argv: []string{"apt"}, Code: 100},
			want: core.Failed,
		},
		{
			name: "a cancelled run",
			err:  context.Canceled,
			want: core.Cancelled,
		},
		{
			// Absence is Detect returning false, never an outcome derived from
			// an error. It classifies as a failure here because on its own,
			// stripped of the question that was being asked, that is all it is.
			name: "a missing program",
			err:  ErrNotFound,
			want: core.Failed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := core.Classify(tt.err); got != tt.want {
				t.Errorf("core.Classify(%v) = %s, want %s", tt.err, got, tt.want)
			}
		})
	}
}
