package exec

import (
	"context"
	"errors"
	"fmt"
	"os"
	osexec "os/exec"
	"slices"
	"time"
)

// waitDelay bounds how long Wait keeps reading the output pipes after the
// process itself is gone.
//
// A process that exits while a descendant still holds the inherited pipes would
// otherwise leave Wait blocked forever on output nobody is going to send. The
// delay turns that into a bounded, reported failure.
const waitDelay = 5 * time.Second

// osRunner is the [Runner] that starts real processes. It holds no state, so
// one is shared by every provider in a run.
type osRunner struct{}

// New returns a [Runner] that starts real processes.
//
// Tests want the fake in internal/exec/exectest instead. Nothing in upall should
// construct this outside the wiring in cmd/.
func New() Runner {
	return &osRunner{}
}

// Run implements [Runner].
func (r *osRunner) Run(ctx context.Context, c Command) (Output, error) {
	if len(c.Argv) == 0 {
		return Output{ExitCode: -1}, ErrNoCommand
	}

	// The per-command deadline is a context deadline and nothing else, so that
	// it cancels the process by the same path Ctrl-C does and needs no second
	// mechanism to unwind.
	runCtx := ctx
	if c.Timeout > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, c.Timeout)
		defer cancel()
	}

	stdout := &capture{keep: keepHead, limit: MaxCapture}
	stderr := &capture{keep: keepTail, limit: MaxCapture}

	cmd := osexec.CommandContext(runCtx, c.Argv[0], c.Argv[1:]...)
	cmd.Dir = c.Dir
	if len(c.Env) > 0 {
		// Appended rather than assigned: os/exec keeps the last occurrence of a
		// key, so this overrides what it inherits instead of discarding it.
		cmd.Env = append(os.Environ(), c.Env...)
	}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.WaitDelay = waitDelay

	// cmd.Stdin is left nil, which os/exec connects to the null device. That is
	// deliberate and there is no field to change it: a package manager that
	// decides to prompt reads EOF and fails, rather than waiting forever on an
	// answer nobody can see it asking for.

	started := time.Now()
	err := cmd.Run()

	out := Output{
		Stdout:    stdout.bytes(),
		Stderr:    stderr.bytes(),
		ExitCode:  exitCode(cmd),
		Duration:  time.Since(started),
		Truncated: stdout.truncated || stderr.truncated,
	}

	if err == nil {
		return out, nil
	}
	return out, classify(ctx, runCtx, c, out, err)
}

// classify turns the error Run got from os/exec into the one this package
// promises, which is what lets
// [github.com/daniel-kindl/upall/internal/core.Classify] decide an outcome
// without reading any message.
//
// It is only reached when the command failed. A command that succeeded reports
// success even if the context was cancelled a moment later, because it did in
// fact do the work.
func classify(ctx, runCtx context.Context, c Command, out Output, err error) error {
	// The parent context is checked first, and the order is the whole point. A
	// Ctrl-C that lands on a command which also had a deadline is a
	// cancellation: both contexts are done, and only this ordering keeps the
	// run from exiting 1 for a timeout where the contract says 130 for an
	// interrupt.
	if parent := ctx.Err(); parent != nil {
		return fmt.Errorf("%s: %w", c.Argv[0], parent)
	}

	// Reaching here with runCtx done means the per-command deadline fired,
	// since without one runCtx is ctx and was just checked.
	if runCtx.Err() != nil {
		return &TimeoutError{Argv: slices.Clone(c.Argv), Deadline: c.Timeout}
	}

	var exitErr *osexec.ExitError
	if errors.As(err, &exitErr) {
		return &ExitError{
			Argv:   slices.Clone(c.Argv),
			Code:   out.ExitCode,
			Stderr: out.Stderr,
		}
	}

	// Everything else is a failure to run it at all: not on PATH, not
	// executable, a working directory that does not exist. ErrNotFound travels
	// out through this wrapping and errors.Is still finds it.
	return fmt.Errorf("%s: %w", c.Argv[0], err)
}

// exitCode reads the status the process exited with, or -1 if it never got one
// because it did not start.
func exitCode(cmd *osexec.Cmd) int {
	if cmd.ProcessState == nil {
		return -1
	}
	return cmd.ProcessState.ExitCode()
}
