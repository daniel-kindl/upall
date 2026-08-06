package exec

import (
	"context"
	"errors"
	osexec "os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestCancelKillsTheWholeProcessTree is the M3 criterion that cancelling the
// context kills the process, proven against a command two processes deep.
//
// The proof is the captured stdout pipe, not a check on the pid.
//
// Capturing stdout makes os/exec create a pipe and a goroutine copying from it,
// and Wait cannot return until that goroutine sees end of file. The read end
// only reaches end of file when every duplicate of the write end is closed, and
// the grandchild inherited one. So Run returning at all is proof the grandchild
// is dead — not evidence of it, proof, because there is no other way for that
// pipe to end.
//
// Asking the operating system whether the pid is alive would be the obvious
// alternative and it is unsound on both platforms. Windows reuses pids
// aggressively, so between the kill and the question the number can name
// something else entirely. On Linux kill(pid, 0) succeeds for a zombie, a
// process that has exited but not yet been reaped. Both are rare in one run and
// near-certain across a matrix run hundreds of times, which is the definition of
// a flake.
func TestCancelKillsTheWholeProcessTree(t *testing.T) {
	ready := filepath.Join(t.TempDir(), "grandchild.started")

	// Long WaitDelay on purpose. It exists so that Run cannot hang in
	// production, but here it must not fire: if it did, Run would return
	// because os/exec force-closed the pipes rather than because the grandchild
	// died, and the assertion below would be measuring the wrong thing.
	r := testRunner(100*time.Millisecond, time.Minute)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		_, err := r.Run(ctx, helperCommand(t, "tree", ready))
		done <- err
	}()

	// The grandchild writes this, so waiting for it means the tree really is
	// two deep before anything is cancelled.
	if _, err := awaitFile(ready, 30*time.Second); err != nil {
		t.Fatalf("the grandchild never started, so there was no tree to kill: %v", err)
	}
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Run() returned %v, want an error wrapping context.Canceled", err)
		}
		if errors.Is(err, osexec.ErrWaitDelay) {
			t.Error("the output pipes had to be force-closed, so something still held the write end: the tree outlived the kill")
		}

	case <-time.After(20 * time.Second):
		t.Fatal("Run() has not returned: the captured stdout pipe has not reached end of file, " +
			"so the grandchild survived cancellation and only the direct child was killed")
	}
}

// TestProcessTreeFallsBackWhenUnconfined checks that a command whose
// confinement never completed is still killed.
//
// Job object creation can fail on a restricted Windows token, and the design
// treats that as a degradation rather than a failure: the command runs, and
// cancelling it reaches the command itself even though it can no longer reach
// what the command spawned. A run on such a machine must still be
// interruptible.
func TestProcessTreeFallsBackWhenUnconfined(t *testing.T) {
	ready := filepath.Join(t.TempDir(), "running")

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		// An empty tree is exactly what a platform returns when confinement
		// could not be set up, so this exercises the fallback directly instead
		// of trying to contrive a restricted environment.
		r := testRunner(100*time.Millisecond, defaultWaitDelay)
		r.confine = func(*osexec.Cmd) *processTree { return &processTree{} }

		_, err := r.Run(ctx, helperCommand(t, "sleep", "30s", ready))
		done <- err
	}()

	if _, err := awaitFile(ready, 30*time.Second); err != nil {
		t.Fatalf("the command never started: %v", err)
	}
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("the unconfined command returned %v, want an error wrapping context.Canceled", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("an unconfined command was not killed by cancelling its context")
	}
}
