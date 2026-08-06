//go:build linux

package exec

import (
	"errors"
	"os"
	osexec "os/exec"
	"testing"
	"time"
)

// TestKillReportsAProcessThatAlreadyFinished pins the ESRCH translation.
//
// os/exec documents that an error returned from Cmd.Cancel replaces the
// command's exit status, unless it wraps os.ErrProcessDone. So a command that
// finished in the moment between its context ending and the signal arriving
// would report a failure it never had, purely because the kernel said there was
// nothing left to signal.
//
// The race is real but not something a test can arrange reliably, so this
// checks the translation directly on a tree whose process is definitely gone.
func TestKillReportsAProcessThatAlreadyFinished(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatalf("locating the test binary: %v", err)
	}

	cmd := osexec.Command(self, "-test.run=^TestHelperProcess$", "--", "echo", "done")
	cmd.Env = append(os.Environ(), helperEnv+"=1")

	tree := newProcessTree(cmd)
	defer tree.release()

	if err := cmd.Start(); err != nil {
		t.Fatalf("starting the command: %v", err)
	}
	if err := tree.attach(cmd); err != nil {
		t.Fatalf("attaching the command to its group: %v", err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("the command failed: %v", err)
	}

	// The process has exited and been reaped, so its group is empty and the
	// kernel answers ESRCH.
	unwound := make(chan struct{})
	close(unwound)

	err = tree.kill(cmd, time.Second, unwound)
	if !errors.Is(err, os.ErrProcessDone) {
		t.Errorf("kill on a finished command returned %v, want an error wrapping os.ErrProcessDone; "+
			"anything else replaces the exit status the command actually had", err)
	}
}
