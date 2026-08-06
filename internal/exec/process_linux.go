//go:build linux

package exec

import (
	"errors"
	"os"
	osexec "os/exec"
	"sync"
	"syscall"
	"time"
)

// processTree is a POSIX process group holding a command and everything it
// spawned.
//
// A package manager is rarely one process. apt drives dpkg, which drives
// maintainer scripts, and killing only the one upall started leaves the rest
// running with the package database locked. Putting the command in its own
// process group means one signal reaches all of them.
type processTree struct {
	// mu guards pgid. os/exec calls Cancel, and so kill, from its own
	// goroutine as soon as the context is done, which can be while attach is
	// still recording the group.
	mu sync.Mutex

	// pgid is the group, which equals the command's own pid because it was
	// made the group leader. It is zero until [processTree.attach] runs, and
	// [processTree.kill] treats that as "not confined" rather than risking a
	// signal to group zero, which is upall's own.
	pgid int
}

// newProcessTree puts the command in a new process group of its own. It is
// called before Start, because SysProcAttr is only read when the process is
// created.
func newProcessTree(cmd *osexec.Cmd) *processTree {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	// Setpgid with Pgid left at zero means "a new group led by this child", so
	// the group id ends up equal to the child's pid.
	cmd.SysProcAttr.Setpgid = true

	return &processTree{}
}

// attach records the group now that the process exists. It cannot fail on
// Linux: the group was created by the kernel as part of starting the process.
func (t *processTree) attach(cmd *osexec.Cmd) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.pgid = cmd.Process.Pid
	return nil
}

// kill signals the whole group, escalating from SIGTERM to SIGKILL if the grace
// period passes without the command unwinding.
//
// The two phases are not politeness. SIGKILL to dpkg part-way through unpacking
// leaves a machine that needs `dpkg --configure -a` before it can install
// anything again, which is a considerably worse outcome than waiting a few
// seconds for it to put itself down.
//
// done is closed once Wait has returned, and bounds the escalation goroutine so
// a cancelled command cannot leave one behind.
func (t *processTree) kill(cmd *osexec.Cmd, grace time.Duration, done <-chan struct{}) error {
	t.mu.Lock()
	pgid := t.pgid
	t.mu.Unlock()

	if pgid <= 0 {
		// Never signal group zero. That is upall's own process group, and
		// killing it would take down the run, the CLI, and any sibling command
		// along with whatever went wrong here.
		return cmd.Process.Kill()
	}

	if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil {
		if errors.Is(err, syscall.ESRCH) {
			// The command finished between the context ending and this signal.
			// Reporting os.ErrProcessDone is what makes Wait return the real
			// exit status: os/exec documents that any other error from Cancel
			// replaces it, which would turn a command that succeeded a
			// microsecond too early into a spurious failure.
			return os.ErrProcessDone
		}
		return err
	}

	go func() {
		select {
		case <-done:
			// It unwound on its own. Nothing left to escalate to.
		case <-time.After(grace):
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
		}
	}()

	return nil
}

// release has nothing to drop. A process group is not a handle; it stops
// existing when its last member does.
func (t *processTree) release() {}
