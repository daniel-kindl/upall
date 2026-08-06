package exec

import (
	"context"
	"time"
)

// Runner runs one subprocess and returns what it wrote.
//
// It is an interface so that tests can replace it. Every subprocess in upall
// goes through a Runner, which means every test can substitute canned output for
// a real package manager, and no test suite mutates the machine running it. The
// fake lives in internal/exec/exectest.
//
// Implementations must be safe for concurrent use. Detect and plan run
// concurrently across providers, so one Runner is shared by all of them.
type Runner interface {
	// Run starts the command, waits for it to finish, and returns what it
	// wrote.
	//
	// The returned [Output] is populated whether or not the error is nil, so a
	// failed command's stderr survives for the report. See [ExitError] for what
	// a non-zero exit looks like, and the package comment for how cancellation
	// and timeouts are distinguished.
	Run(ctx context.Context, cmd Command) (Output, error)
}

// Command is one subprocess to run.
//
// It is a struct rather than a list of parameters so that a field can be added
// without breaking every implementation of [Runner].
type Command struct {
	// Argv is the program and its arguments, with the program at Argv[0].
	//
	// It is one slice, and there is deliberately no field anywhere in this
	// package that takes a command line as a string. There is no quoting rule
	// that is correct on both cmd.exe and sh, and interpolating into a shell is
	// an injection surface in a tool that runs elevated. Passing argv sidesteps
	// both, and having no string form means no caller can reintroduce them.
	//
	// An empty Argv is [ErrNoCommand].
	Argv []string

	// Dir is the working directory. Empty means the directory upall was
	// started in.
	Dir string

	// Env is added to the environment the command inherits, in "KEY=value"
	// form. A key already present is overridden rather than duplicated.
	//
	// It overlays rather than replaces because a package manager with no PATH
	// cannot find the tools it shells out to, so a replacing Env would break
	// every provider that used it. The additions are things like
	// DEBIAN_FRONTEND=noninteractive, which are additions in spirit too.
	//
	// The environment is never logged. See the package comment.
	Env []string

	// Timeout is how long the command may run before it is killed. Zero means
	// no limit, and the context's own deadline, if it has one, still applies.
	//
	// Exceeding it is a [TimeoutError], which unwraps to
	// context.DeadlineExceeded and so classifies as a timeout in
	// [github.com/daniel-kindl/upall/internal/core.Classify].
	Timeout time.Duration
}

// Output is what a finished subprocess produced.
//
// It is returned even when Run reports an error, because the reason a command
// failed is usually in the output rather than in the error.
type Output struct {
	// Stdout is what the command wrote to standard output, capped at
	// [MaxCapture]. The beginning is kept, because that is where the output a
	// parser reads starts.
	Stdout []byte

	// Stderr is what the command wrote to standard error, capped at
	// [MaxCapture]. The end is kept, because that is where the error that
	// stopped it is.
	Stderr []byte

	// ExitCode is the code the process exited with.
	//
	// It is -1 when the process never ran or was killed by a signal rather than
	// exiting on its own. In that case the error says what happened; the code
	// alone cannot.
	ExitCode int

	// Truncated reports that either stream produced more than [MaxCapture]
	// bytes and some were dropped.
	//
	// It exists so that the loss is never silent. A parser handed a truncated
	// stdout would otherwise report on part of the output as though it were all
	// of it, which is how a tool starts describing a machine state that is not
	// the one it looked at.
	Truncated bool

	// Duration is how long the command took, measured around the whole call.
	Duration time.Duration
}
