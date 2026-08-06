//go:build windows

package exec

import (
	"errors"
	osexec "os/exec"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// errNoJobObject reports that the command could not be confined to a job, so
// cancelling it reaches the command itself and not its descendants.
var errNoJobObject = errors.New("no job object")

// processTree is a Win32 job object holding a command and everything it
// spawned.
//
// A package manager is rarely one process. winget hands work to installers and
// msiexec, and killing only the one upall started leaves those running. A
// process assigned to a job cannot escape it, and neither can anything it
// creates, so one call terminates all of them.
type processTree struct {
	// mu guards everything below. os/exec calls Cancel, and so kill, from its
	// own goroutine as soon as the context is done, which can be while attach
	// is still running or while release is dropping the handle. Without this
	// the three race, and the consequence is not a corrupt field but a
	// terminate that misses or a handle closed twice.
	mu sync.Mutex

	// job is the job object, or zero once it has been released or if one could
	// not be created.
	job windows.Handle

	// assigned reports whether the command actually made it into the job.
	// When it did not, kill falls back to the direct child rather than
	// terminating an empty job and leaving the command running.
	assigned bool
}

// newProcessTree creates the job the command will be confined to.
//
// Failing here is deliberately not an error. A machine where job objects are
// unavailable should still be able to update itself, so the command runs with a
// weaker cancellation guarantee rather than not at all.
func newProcessTree(_ *osexec.Cmd) *processTree {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return &processTree{}
	}

	// KILL_ON_JOB_CLOSE does two jobs. The obvious one is that terminating the
	// job kills the tree. The quieter and more valuable one is that release
	// closes this handle on every path, including a clean exit, so an installer
	// that outlived a winget which exited zero is cleaned up too.
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}
	if _, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		_ = windows.CloseHandle(job)
		return &processTree{}
	}

	return &processTree{job: job}
}

// attach puts the started process into the job.
//
// It uses [os.Process.WithHandle] rather than reopening the process by pid.
// Windows reuses pids aggressively, so between Start returning and an
// OpenProcess call the number can already name something else, and assigning
// the wrong process to a job upall is about to terminate is a considerably
// worse bug than the one it was trying to fix. WithHandle borrows the handle
// os/exec already holds, which is guaranteed to refer to this process even if
// it has since exited.
//
// There is one window this does not close: between the process being created
// and being assigned, anything it spawns is outside the job. Closing it needs
// PROC_THREAD_ATTRIBUTE_JOB_LIST, which syscall.SysProcAttr does not expose, so
// reaching it would mean calling CreateProcess directly and giving up
// everything os/exec handles around cancellation and pipe teardown. The window
// is microseconds wide and package managers spend milliseconds starting up, so
// it is accepted knowingly rather than overlooked.
func (t *processTree) attach(cmd *osexec.Cmd) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.job == 0 {
		return errNoJobObject
	}

	var assignErr error
	if err := cmd.Process.WithHandle(func(handle uintptr) {
		assignErr = windows.AssignProcessToJobObject(t.job, windows.Handle(handle))
	}); err != nil {
		return err
	}
	if assignErr != nil {
		return assignErr
	}

	t.assigned = true
	return nil
}

// kill terminates the job, and with it every process in it.
//
// The grace period and done channel are unused here. Windows has no portable
// way to ask a non-console process to stop politely: GenerateConsoleCtrlEvent
// needs a shared console and a process group, cannot reach a GUI-subsystem
// process, and neither msiexec nor winget acts on it. Inventing a graceful
// phase that works occasionally would be worse than terminating honestly, so
// the asymmetry with Linux is deliberate and lives in these two files.
func (t *processTree) kill(cmd *osexec.Cmd, _ time.Duration, _ <-chan struct{}) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.assigned {
		return cmd.Process.Kill()
	}
	return windows.TerminateJobObject(t.job, 1)
}

// release closes the job handle, which terminates anything still in it.
//
// Zeroing the handle under the lock makes this safe to call more than once and
// stops a late kill from terminating a job that is no longer ours.
func (t *processTree) release() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.job != 0 {
		_ = windows.CloseHandle(t.job)
		t.job = 0
		t.assigned = false
	}
}
