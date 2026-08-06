package exectest

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/daniel-kindl/upall/internal/exec"
)

// Response is what a [Fake] returns for one argv.
//
// The zero value is a silent success, which is the right default for a command
// a test does not care about beyond it having been run.
type Response struct {
	// Stdout and Stderr are what the command "wrote".
	Stdout string
	Stderr string

	// ExitCode is the status it exited with. Anything but zero makes Run
	// return an [exec.ExitError] carrying it, exactly as the real runner does,
	// so a provider cannot pass its tests by ignoring a failure it would fail
	// on in production.
	ExitCode int

	// Err replaces the result entirely, for the failures that are not an exit
	// status: [exec.ErrNotFound] for a tool that is not installed,
	// context.Canceled, an [exec.TimeoutError]. When it is set, ExitCode is
	// ignored.
	Err error

	// Delay is how long Run pretends to take, honouring the context while it
	// waits.
	//
	// It is here for M5, whose criterion asks for a test with deliberately slow
	// fakes proving that detect and plan across providers take as long as the
	// slowest rather than the sum. Building it now means that milestone does
	// not have to reopen this package.
	Delay time.Duration
}

// Fake is an [exec.Runner] that returns canned results and records what it was
// asked to run. See the package comment for how to use one.
type Fake struct {
	mu        sync.Mutex
	responses map[string]Response
	fallback  *Response
	calls     []exec.Command
}

// Verify at compile time that the fake is substitutable for the real thing. A
// signature change in exec.Runner should fail here rather than in every test
// that uses this.
var _ exec.Runner = (*Fake)(nil)

// New returns a Fake with nothing registered, which fails any command until
// [Fake.On] or [Fake.Default] says otherwise.
func New() *Fake {
	return &Fake{responses: make(map[string]Response)}
}

// key is how an argv is filed.
//
// The separator is a NUL because a NUL cannot appear inside an argv element on
// either operating system: both hand the kernel NUL-terminated strings. Joining
// with a space would file []string{"a b"} and []string{"a", "b"} under the same
// key, and those are different commands.
func key(argv []string) string {
	return strings.Join(argv, "\x00")
}

// On files a response for exactly this argv. It returns the Fake so
// registrations can be chained.
func (f *Fake) On(argv []string, r Response) *Fake {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.responses[key(argv)] = r
	return f
}

// Default files a response for any argv nothing else matched, turning an
// unexpected command from a failure into this. It returns the Fake so it can be
// chained.
func (f *Fake) Default(r Response) *Fake {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.fallback = &r
	return f
}

// Run implements [exec.Runner]. It records the command and answers from what
// was filed.
func (f *Fake) Run(ctx context.Context, c exec.Command) (exec.Output, error) {
	if len(c.Argv) == 0 {
		return exec.Output{ExitCode: -1}, exec.ErrNoCommand
	}

	r, err := f.record(c)
	if err != nil {
		return exec.Output{ExitCode: -1}, err
	}

	if r.Delay > 0 {
		select {
		case <-time.After(r.Delay):
		case <-ctx.Done():
			// A slow fake has to be interruptible, or the cancellation tests at
			// M6 would have nothing to cancel.
			return exec.Output{ExitCode: -1}, ctx.Err()
		}
	}

	out := exec.Output{
		Stdout:   []byte(r.Stdout),
		Stderr:   []byte(r.Stderr),
		ExitCode: r.ExitCode,
		Duration: r.Delay,
	}

	switch {
	case r.Err != nil:
		return out, r.Err
	case r.ExitCode != 0:
		return out, &exec.ExitError{
			Argv:   slices.Clone(c.Argv),
			Code:   r.ExitCode,
			Stderr: out.Stderr,
		}
	default:
		return out, nil
	}
}

// record files the invocation and returns the response for it, holding the lock
// for as little as possible and never across the delay.
func (f *Fake) record(c exec.Command) (Response, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Cloned because the caller owns these and may reuse the backing array as
	// soon as Run returns, which would rewrite history under the assertion.
	f.calls = append(f.calls, exec.Command{
		Argv:    slices.Clone(c.Argv),
		Dir:     c.Dir,
		Env:     slices.Clone(c.Env),
		Timeout: c.Timeout,
	})

	if r, found := f.responses[key(c.Argv)]; found {
		return r, nil
	}
	if f.fallback != nil {
		return *f.fallback, nil
	}

	return Response{}, f.unexpected(c.Argv)
}

// unexpected explains a command nothing was filed against, listing what was, so
// the failure names the mismatch instead of leaving it to be hunted.
func (f *Fake) unexpected(argv []string) error {
	registered := slices.Sorted(maps.Keys(f.responses))
	for i, r := range registered {
		registered[i] = strings.Join(strings.Split(r, "\x00"), " ")
	}

	if len(registered) == 0 {
		return fmt.Errorf("exectest: unexpected command %q, and nothing was registered", argv)
	}
	return fmt.Errorf("exectest: unexpected command %q; registered: %q", argv, registered)
}

// Calls returns the commands that were run, in order.
//
// The slice is a copy, so reading it while another goroutine is still running
// commands is safe and gives a snapshot rather than a moving target.
func (f *Fake) Calls() []exec.Command {
	f.mu.Lock()
	defer f.mu.Unlock()

	return slices.Clone(f.calls)
}

// Ran reports whether exactly this argv was run at least once.
func (f *Fake) Ran(argv ...string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	want := key(argv)
	for _, c := range f.calls {
		if key(c.Argv) == want {
			return true
		}
	}
	return false
}
