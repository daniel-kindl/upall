package exec

import (
	"context"
	"fmt"
	"io"
	"os"
	osexec "os/exec"
	"strconv"
	"testing"
	"time"
)

// helperEnv is the sentinel that turns a re-executed copy of this test binary
// into the subprocess under test.
const helperEnv = "UPALL_EXEC_HELPER"

// helperArgv returns the argv that re-executes this test binary as the helper,
// running mode with args.
//
// The subprocess these tests run is the test binary itself, re-executed. This is
// the os/exec standard library's own idiom, and it is here for a reason the
// ROADMAP states as a criterion: the tests must pass on both operating systems
// using a command that exists on both. No such command exists — cmd.exe and sh
// share nothing — so the honest answer is a command that exists on both by
// construction, which is this binary.
//
// Pair it with [helperEnv] in [Command].Env; without that the helper returns
// immediately and the test binary does nothing.
func helperArgv(t *testing.T, mode string, args ...string) []string {
	t.Helper()

	self, err := os.Executable()
	if err != nil {
		t.Fatalf("locating the test binary to re-execute: %v", err)
	}

	// The path is absolute because Command.Dir would otherwise resolve a
	// relative one against the wrong directory, which is precisely what the Dir
	// test changes.
	argv := []string{self, "-test.run=^TestHelperProcess$", "--", mode}
	return append(argv, args...)
}

// helperEnviron is the environment that arms the helper, plus any additions.
func helperEnviron(extra ...string) []string {
	return append([]string{helperEnv + "=1"}, extra...)
}

// testRunner is [New] with the timings a test needs instead of the defaults.
//
// It exists so that a test cannot construct an osRunner and forget to wire
// confine, which fails as a nil dereference deep inside Run rather than as
// anything that names the omission.
func testRunner(killGrace, waitDelay time.Duration) *osRunner {
	return &osRunner{
		killGrace: killGrace,
		waitDelay: waitDelay,
		confine:   newProcessTree,
	}
}

// helperCommand is the [Command] that runs the helper in mode, armed and ready
// to pass to a [Runner].
func helperCommand(t *testing.T, mode string, args ...string) Command {
	t.Helper()
	return Command{Argv: helperArgv(t, mode, args...), Env: helperEnviron()}
}

// awaitFile blocks until the helper has written path, and returns what it wrote.
//
// The helper announces itself this way so that a test can act on a command that
// is genuinely running. Without it a test races the child's startup, and a
// cancellation that only ever killed a process still initialising would pass
// while proving very little. Re-executing a race-instrumented test binary takes
// the better part of a second, so it is a race the test would usually win.
//
// It reports an error rather than taking a *testing.T because its callers wait
// in a goroutine while Run blocks in the test's own, and t.Fatal from anywhere
// but the test goroutine only stops the goroutine that called it.
func awaitFile(path string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		switch content, err := os.ReadFile(path); {
		case err == nil && len(content) > 0:
			return string(content), nil
		case err != nil && !os.IsNotExist(err):
			return "", fmt.Errorf("reading %s: %w", path, err)
		}
		time.Sleep(5 * time.Millisecond)
	}
	return "", fmt.Errorf("%s never appeared within %s", path, timeout)
}

// awaitStart waits for the helper to announce itself and then cancels it,
// failing the test if it never got going.
//
// It always cancels, including on failure, so that a helper which never started
// leaves the test reporting that rather than blocked on a command nobody is
// going to stop.
func awaitStart(t *testing.T, ready string, cancel context.CancelFunc) {
	if _, err := awaitFile(ready, 30*time.Second); err != nil {
		t.Errorf("the helper never started: %v", err)
	}
	cancel()
}

// TestHelperProcess is not a test. It is the body of the subprocess that the
// tests in this package run, and it does nothing at all unless [helperEnv] is
// set in its environment, which only [helperEnviron] does.
//
// Every mode exits through os.Exit rather than returning, so that the testing
// framework never gets to print its own "PASS" into output a test is about to
// assert on.
func TestHelperProcess(t *testing.T) {
	if os.Getenv(helperEnv) != "1" {
		return
	}
	defer os.Exit(0)

	args := os.Args
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}
	if len(args) == 0 {
		emit(os.Stderr, "helper: no mode given")
		os.Exit(2)
	}

	mode, args := args[0], args[1:]
	switch mode {
	case "echo":
		// Deliberately without a newline, so a test can assert on exact bytes.
		emit(os.Stdout, args[0])

	case "warn":
		emit(os.Stderr, args[0])

	case "both":
		emit(os.Stdout, args[0])
		emit(os.Stderr, args[1])

	case "exit":
		code, err := strconv.Atoi(args[0])
		if err != nil {
			emitf(os.Stderr, "helper: unparseable exit code %q", args[0])
			os.Exit(2)
		}
		// Anything the caller wants to read alongside a failure.
		if len(args) > 1 {
			emit(os.Stderr, args[1])
		}
		os.Exit(code)

	case "env":
		emit(os.Stdout, os.Getenv(args[0]))

	case "pwd":
		dir, err := os.Getwd()
		if err != nil {
			emitf(os.Stderr, "helper: %v", err)
			os.Exit(2)
		}
		emit(os.Stdout, dir)

	case "stdin":
		// Reports what reading standard input does. It should be an immediate
		// EOF, because this package never gives a command one.
		var buf [16]byte
		n, err := os.Stdin.Read(buf[:])
		emitf(os.Stdout, "read %d bytes, err %v", n, err)

	case "sleep":
		d, err := time.ParseDuration(args[0])
		if err != nil {
			emitf(os.Stderr, "helper: unparseable duration %q", args[0])
			os.Exit(2)
		}
		// Announce that this process is up before blocking, so a test can wait
		// for it to really be running rather than racing its own startup.
		if len(args) > 1 {
			if err := os.WriteFile(args[1], []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
				emitf(os.Stderr, "helper: %v", err)
				os.Exit(2)
			}
		}
		time.Sleep(d)

	case "tree":
		// Start a grandchild and then block, so the runner is cancelling a
		// command that is two processes deep.
		//
		// The grandchild inherits this process's stdout, which is the runner's
		// capture pipe, and it is the one that writes the readiness file. Both
		// facts are load-bearing: the file appearing means the grandchild
		// specifically is running, and the inherited pipe is what the test
		// reads the answer from. See TestCancelKillsTheWholeProcessTree.
		//
		// Deliberately no process group, no job, and no Pdeathsig on it. The
		// grandchild must survive its parent unless the runner kills it, or
		// the test would pass without the code under test doing anything.
		self, err := os.Executable()
		if err != nil {
			emitf(os.Stderr, "helper: %v", err)
			os.Exit(2)
		}

		sub := osexec.Command(self, "-test.run=^TestHelperProcess$", "--", "sleep", "5m", args[0])
		sub.Env = append(os.Environ(), helperEnv+"=1")
		sub.Stdout = os.Stdout
		sub.Stderr = os.Stderr
		if err := sub.Start(); err != nil {
			emitf(os.Stderr, "helper: starting the grandchild: %v", err)
			os.Exit(2)
		}

		time.Sleep(5 * time.Minute)

	default:
		emitf(os.Stderr, "helper: unknown mode %q", mode)
		os.Exit(2)
	}
}

// emit and emitf write the helper's output, discarding the write error.
//
// Producing output is the helper's entire job, so a failure to write leaves it
// nothing to report and nowhere to report it. Discarding is explicit here
// because errcheck fails the build on an ignored error, and an ignored one that
// is genuinely unactionable should say so rather than be excluded in config.
func emit(w io.Writer, a ...any) {
	_, _ = fmt.Fprint(w, a...)
}

func emitf(w io.Writer, format string, a ...any) {
	_, _ = fmt.Fprintf(w, format, a...)
}
