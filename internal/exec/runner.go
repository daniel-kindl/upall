package exec

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	osexec "os/exec"
	"slices"
	"time"
)

const (
	// defaultKillGrace is how long a cancelled command is given to unwind after
	// being asked to stop, before it is killed outright. It is only used where
	// a polite stop exists, which is Linux; see process_windows.go.
	defaultKillGrace = 5 * time.Second

	// defaultWaitDelay bounds how long Wait keeps reading the output pipes
	// after the process itself is gone.
	//
	// A descendant that survived holds the inherited pipes open, and Wait
	// cannot return until every writer has closed them, so without this a
	// leaked process turns into a hung run. It must stay comfortably above
	// defaultKillGrace, or it would fire while the escalation to SIGKILL is
	// still pending and kill the direct child out from under it.
	defaultWaitDelay = 10 * time.Second
)

// osRunner is the [Runner] that starts real processes. It holds no mutable
// state, so one is shared by every provider in a run.
type osRunner struct {
	// logger records each command once it has finished. It is never nil: [New]
	// substitutes a discarding handler, and the field is unexported because a
	// settable one would be reassigned mid-run by somebody, and detect and plan
	// share one runner across goroutines.
	logger *slog.Logger

	// killGrace and waitDelay are fields rather than constants so this
	// package's own tests can stretch or shrink them. Nothing outside sets
	// them.
	killGrace time.Duration
	waitDelay time.Duration

	// confine builds the process tree, and is a field for one reason: a
	// platform that cannot confine a command returns an empty tree and the run
	// degrades to killing the direct child. That path has to be tested, and
	// contriving a machine where job objects are unavailable is not something
	// a test can do. Nothing outside this package sets it.
	confine func(*osexec.Cmd) *processTree
}

// New returns a [Runner] that starts real processes, recording each one to
// logger at debug level.
//
// A nil logger discards. That is the default rather than slog.Default because
// the default logger writes to standard error, and nothing below internal/cli
// may write to a terminal. A package that acquired one by omission is exactly
// the failure the frontend boundary exists to prevent, so the omission has to
// be silence. What is and is not recorded is in the package comment.
//
// Tests want the fake in internal/exec/exectest instead. Nothing in upall should
// construct this outside the wiring in cmd/.
func New(logger *slog.Logger) Runner {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &osRunner{
		logger:    logger,
		killGrace: defaultKillGrace,
		waitDelay: defaultWaitDelay,
		confine:   newProcessTree,
	}
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
	cmd.WaitDelay = r.waitDelay

	// cmd.Stdin is left nil, which os/exec connects to the null device. That is
	// deliberate and there is no field to change it: a package manager that
	// decides to prompt reads EOF and fails, rather than waiting forever on an
	// answer nobody can see it asking for.

	tree := r.confine(cmd)
	defer tree.release()

	// Closed once Wait has returned, which bounds the escalation a platform's
	// kill may have started. The defer runs before tree.release above it, so
	// nothing is still watching when the confinement is dropped.
	unwound := make(chan struct{})
	defer close(unwound)

	// Replaces the plain process kill CommandContext installs, which reaches
	// the command and nothing it spawned.
	cmd.Cancel = func() error { return tree.kill(cmd, r.killGrace, unwound) }

	started := time.Now()
	if err := cmd.Start(); err != nil {
		out := Output{ExitCode: -1, Duration: time.Since(started)}
		err = classify(ctx, runCtx, c, out, err)
		r.log(ctx, c, out, err)
		return out, err
	}

	// Confinement can only be completed once the process exists. Failing here
	// is a degradation rather than a failure: the command is already running,
	// kill falls back to the direct child, and a machine where process
	// confinement is unavailable should still be able to update itself. The
	// debug logger records it when logging arrives.
	_ = tree.attach(cmd)

	err := cmd.Wait()

	out := Output{
		Stdout:    stdout.bytes(),
		Stderr:    stderr.bytes(),
		ExitCode:  exitCode(cmd),
		Duration:  time.Since(started),
		Truncated: stdout.truncated || stderr.truncated,
	}

	if err != nil {
		err = classify(ctx, runCtx, c, out, err)
	}
	r.log(ctx, c, out, err)
	return out, err
}

// log records one finished command at debug level.
//
// What is here is argv, where it ran, how long it took, and how it ended. What
// is deliberately absent matters more:
//
// The environment is never recorded, not even its keys. It routinely holds
// credentials — GITHUB_TOKEN, DOCKER_AUTH_CONFIG, an HTTPS_PROXY with a
// password in the URL — and even the key names leak which services a machine
// talks to. Only the count is logged, which answers the one question a debug
// session has: whether the overlay was applied.
//
// Captured output is never recorded either, only its size. Package manager
// output carries repository URLs with credentials embedded, and at debug level
// that ends up pasted into a bug report. The stderr tail reaches the user
// through core.ProviderResult.Output, which is a deliberate and bounded path.
//
// Argv is recorded in full, and that is safe by construction rather than by
// luck: a command line is already readable by any user on the machine through
// /proc/<pid>/cmdline or Task Manager, and core.ProviderResult.Command shows it
// to the user anyway. The corollary is the useful part — a credential on a
// command line is a bug whatever this function does, and the fix is to put it
// in Command.Env, which is never logged. Redacting argv here would hide the bug
// rather than remove it.
func (r *osRunner) log(ctx context.Context, c Command, out Output, err error) {
	attrs := []slog.Attr{
		slog.Any("argv", c.Argv),
		slog.Duration("duration", out.Duration),
		slog.Int("exit_code", out.ExitCode),
		slog.Int("stdout_bytes", len(out.Stdout)),
		slog.Int("stderr_bytes", len(out.Stderr)),
		slog.Int("env_overlay", len(c.Env)),
	}
	if c.Dir != "" {
		attrs = append(attrs, slog.String("dir", c.Dir))
	}
	if err != nil {
		attrs = append(attrs, slog.String("error", err.Error()))
	}

	// LogAttrs rather than Debug, because this runs once per command and the
	// variadic form allocates whether or not anything is listening.
	r.logger.LogAttrs(ctx, slog.LevelDebug, "ran a command", attrs...)
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
