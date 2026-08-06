package exec

import (
	"context"
	"errors"
	"fmt"
	osexec "os/exec"
	"time"
)

// ErrNoCommand reports that a [Command] carried no Argv at all.
//
// It is a bug in the caller rather than a failure of the machine, and it is
// caught before anything is started so that Argv[0] can be indexed safely
// everywhere below.
var ErrNoCommand = errors.New("no command to run")

// ErrNotFound reports that the program named in [Command].Argv[0] is not on
// PATH.
//
// It is the standard library's sentinel, re-exported so that a caller asking
// "is this tool installed?" never has to import os/exec to find out. That
// question is [github.com/daniel-kindl/upall/internal/core.Provider].Detect,
// and the answer is (false, nil): a provider that is not installed is not an
// error.
var ErrNotFound = osexec.ErrNotFound

// ExitError reports that a command ran to completion and exited non-zero.
//
// A non-zero exit is an error rather than a field a caller might forget to
// read. The default is loud on purpose: errcheck fails the build on an ignored
// error, and an unrecognised error classifies as
// [github.com/daniel-kindl/upall/internal/core.Failed], so a provider that says
// nothing about a failed command reports a failed run rather than a silent
// success.
//
// Tolerating a particular code is therefore explicit, which is what makes it
// reviewable:
//
//	out, err := runner.Run(ctx, cmd)
//	var exit *exec.ExitError
//	if errors.As(err, &exit) && exit.Code == 1 {
//		// winget says 1 when there is nothing applicable to upgrade.
//	}
type ExitError struct {
	// Argv is the command that failed, as it was passed to Run.
	Argv []string

	// Code is the exit status.
	Code int

	// Stderr is the captured tail of standard error, the same bytes as
	// [Output].Stderr.
	//
	// It is carried on the error, and not only on the Output, because
	// [github.com/daniel-kindl/upall/internal/core.Provider].Apply returns an
	// error and nothing else. This field is the only route by which what the
	// command said reaches
	// [github.com/daniel-kindl/upall/internal/core.ProviderResult].Output and
	// gets shown to the user.
	Stderr []byte
}

// Error names the program and the status it exited with.
//
// It names Argv[0] rather than the whole command line, because joining argv is
// a decision that belongs where the target shell is known. Anything that wants
// to show the full command has [ExitError.Argv] to render properly.
func (e *ExitError) Error() string {
	return fmt.Sprintf("%s: exit status %d", e.Argv[0], e.Code)
}

// TimeoutError reports that a command exceeded [Command].Timeout and was
// killed.
type TimeoutError struct {
	// Argv is the command that ran out of time.
	Argv []string

	// Deadline is the limit that was exceeded. It fills
	// [github.com/daniel-kindl/upall/internal/core.ProviderResult].Deadline.
	Deadline time.Duration
}

// Error names the program and the deadline it exceeded.
func (e *TimeoutError) Error() string {
	return fmt.Sprintf("%s: timed out after %s", e.Argv[0], e.Deadline)
}

// Unwrap returns context.DeadlineExceeded.
//
// This is load-bearing rather than decorative.
// [github.com/daniel-kindl/upall/internal/core.Classify] maps
// context.DeadlineExceeded to
// [github.com/daniel-kindl/upall/internal/core.TimedOut], and it says in as
// many words that a per-command deadline here must wrap it rather than
// introduce a second error meaning the same thing. Unwrapping this way is what
// lets this package carry the deadline for the report without core having to
// learn that this type exists.
func (e *TimeoutError) Unwrap() error {
	return context.DeadlineExceeded
}
