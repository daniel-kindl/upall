package exectest_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/daniel-kindl/upall/internal/exec"
	"github.com/daniel-kindl/upall/internal/exec/exectest"
)

func TestFakeAnswersFromWhatWasFiled(t *testing.T) {
	tests := []struct {
		name       string
		response   exectest.Response
		wantStdout string
		wantErr    bool
		wantCode   int
	}{
		{
			name:       "canned output",
			response:   exectest.Response{Stdout: "apt is up to date"},
			wantStdout: "apt is up to date",
		},
		{
			name:     "the zero value is a silent success",
			response: exectest.Response{},
		},
		{
			// A fake that returned a bare zero value for a failing command
			// would let a provider pass its tests by ignoring an error it
			// would fail on in production.
			name:     "a non-zero exit is an ExitError, as it is for real",
			response: exectest.Response{ExitCode: 100, Stderr: "E: could not get lock"},
			wantErr:  true,
			wantCode: 100,
		},
		{
			name:     "a canned error replaces the result",
			response: exectest.Response{Err: exec.ErrNotFound},
			wantErr:  true,
		},
	}

	argv := []string{"apt", "upgrade"}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := exectest.New().On(argv, tt.response)

			out, err := f.Run(t.Context(), exec.Command{Argv: argv})

			if tt.wantErr && err == nil {
				t.Fatal("Run() returned no error, want one")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Run() returned %v, want no error", err)
			}
			if got := string(out.Stdout); got != tt.wantStdout {
				t.Errorf("Stdout = %q, want %q", got, tt.wantStdout)
			}

			if tt.wantCode != 0 {
				var exit *exec.ExitError
				if !errors.As(err, &exit) {
					t.Fatalf("Run() returned %[1]T (%[1]v), want an *exec.ExitError", err)
				}
				if exit.Code != tt.wantCode {
					t.Errorf("ExitError.Code = %d, want %d", exit.Code, tt.wantCode)
				}
				if string(exit.Stderr) != tt.response.Stderr {
					t.Errorf("ExitError.Stderr = %q, want %q", exit.Stderr, tt.response.Stderr)
				}
			}
		})
	}
}

// TestFakeMatchesTheWholeArgv is most of why this fake is worth having. A
// manifest test at M4 is not checking that a provider ran something, it is
// checking that the provider built the argv it claims to, so a near miss has to
// be a miss.
func TestFakeMatchesTheWholeArgv(t *testing.T) {
	f := exectest.New().On([]string{"apt-get", "upgrade", "-y"}, exectest.Response{Stdout: "done"})

	tests := []struct {
		name string
		argv []string
		want bool
	}{
		{name: "exactly what was filed", argv: []string{"apt-get", "upgrade", "-y"}, want: true},
		{name: "a dropped flag is a different command", argv: []string{"apt-get", "upgrade"}},
		{name: "an extra flag is a different command", argv: []string{"apt-get", "upgrade", "-y", "--quiet"}},
		{name: "a different program", argv: []string{"apt", "upgrade", "-y"}},
		{
			// Joining on a space would file this under the same key as the
			// registration, and they are not the same command.
			name: "one argument that merely spells the same thing",
			argv: []string{"apt-get upgrade -y"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := f.Run(t.Context(), exec.Command{Argv: tt.argv})
			if matched := err == nil; matched != tt.want {
				t.Errorf("running %q matched = %t, want %t (err: %v)", tt.argv, matched, tt.want, err)
			}
		})
	}
}

// TestFakeRefusesAnUnexpectedCommand checks that a test whose code runs
// something nobody planned for fails, and fails legibly. Returning a silent
// success would let a typo in a provider's argv go unnoticed for as long as the
// provider exists.
func TestFakeRefusesAnUnexpectedCommand(t *testing.T) {
	f := exectest.New().On([]string{"apt", "list"}, exectest.Response{})

	_, err := f.Run(t.Context(), exec.Command{Argv: []string{"dnf", "check-update"}})
	if err == nil {
		t.Fatal("Run() accepted a command nothing was registered for, want an error")
	}

	// The message has to name both halves, or it sends the reader hunting.
	for _, want := range []string{"dnf", "check-update", "apt", "list"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error %q does not mention %q", err, want)
		}
	}
}

func TestFakeDefaultCatchesTheRest(t *testing.T) {
	f := exectest.New().Default(exectest.Response{Stdout: "anything"})

	out, err := f.Run(t.Context(), exec.Command{Argv: []string{"never", "registered"}})
	if err != nil {
		t.Fatalf("Run() returned %v, want the default", err)
	}
	if got := string(out.Stdout); got != "anything" {
		t.Errorf("Stdout = %q, want %q", got, "anything")
	}
}

func TestFakeRecordsWhatItRan(t *testing.T) {
	f := exectest.New().Default(exectest.Response{})

	argv := []string{"apt", "list", "--upgradable"}
	if _, err := f.Run(t.Context(), exec.Command{
		Argv:    argv,
		Dir:     "/tmp",
		Env:     []string{"LC_ALL=C"},
		Timeout: time.Minute,
	}); err != nil {
		t.Fatalf("Run() returned %v, want no error", err)
	}

	calls := f.Calls()
	if len(calls) != 1 {
		t.Fatalf("Calls() has %d entries, want 1", len(calls))
	}
	if !slices.Equal(calls[0].Argv, argv) {
		t.Errorf("recorded argv %q, want %q", calls[0].Argv, argv)
	}
	if calls[0].Dir != "/tmp" || calls[0].Timeout != time.Minute {
		t.Errorf("recorded Dir %q and Timeout %s, want %q and %s", calls[0].Dir, calls[0].Timeout, "/tmp", time.Minute)
	}
	if !slices.Equal(calls[0].Env, []string{"LC_ALL=C"}) {
		t.Errorf("recorded Env %q, want %q", calls[0].Env, []string{"LC_ALL=C"})
	}

	if !f.Ran("apt", "list", "--upgradable") {
		t.Error("Ran() says the command it just recorded was not run")
	}
	if f.Ran("apt", "list") {
		t.Error("Ran() matched a prefix; it must match the whole argv")
	}
}

// TestFakeRecordsACopyOfTheArgv guards against the caller reusing the slice it
// passed. A fake that kept the caller's backing array would let a provider
// rewrite the history a test is about to assert on.
func TestFakeRecordsACopyOfTheArgv(t *testing.T) {
	f := exectest.New().Default(exectest.Response{})

	argv := []string{"apt", "upgrade"}
	if _, err := f.Run(t.Context(), exec.Command{Argv: argv}); err != nil {
		t.Fatalf("Run() returned %v, want no error", err)
	}

	argv[1] = "remove"

	if got := f.Calls()[0].Argv[1]; got != "upgrade" {
		t.Errorf("the recorded argv changed to %q when the caller reused its slice", got)
	}
}

func TestFakeDelayHonoursTheContext(t *testing.T) {
	f := exectest.New().On([]string{"slow"}, exectest.Response{Delay: 30 * time.Second})

	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	started := time.Now()
	_, err := f.Run(ctx, exec.Command{Argv: []string{"slow"}})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Run() returned %v, want context.Canceled", err)
	}
	if elapsed := time.Since(started); elapsed > 10*time.Second {
		t.Errorf("Run() took %s; a slow fake must be interruptible or M6 has nothing to cancel", elapsed)
	}
}

// TestFakeIsSafeForConcurrentUse is the guarantee the pipeline depends on from
// M5, where detect and plan run concurrently across providers sharing one
// runner. It is only meaningful under -race, which is how CI runs.
func TestFakeIsSafeForConcurrentUse(t *testing.T) {
	f := exectest.New().Default(exectest.Response{Stdout: "ok"})

	const runners = 16

	var wg sync.WaitGroup
	for i := range runners {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = f.Run(t.Context(), exec.Command{Argv: []string{"provider", string(rune('a' + i))}})
			_ = f.Calls()
		}()
	}
	wg.Wait()

	if got := len(f.Calls()); got != runners {
		t.Errorf("Calls() has %d entries after %d concurrent runs, want %d", got, runners, runners)
	}
}

func TestFakeRejectsACommandWithNoArgv(t *testing.T) {
	f := exectest.New().Default(exectest.Response{})

	if _, err := f.Run(t.Context(), exec.Command{}); !errors.Is(err, exec.ErrNoCommand) {
		t.Errorf("Run() returned %v, want exec.ErrNoCommand, as the real runner does", err)
	}
}
