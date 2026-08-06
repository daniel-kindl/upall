// Package exec is the one way upall starts a subprocess.
//
// Every command any provider runs goes through a [Runner], and nothing else in
// the module imports os/exec. That is enforced by a test rather than by review,
// because it is the property the rest of the design rests on: one seam means
// tests substitute canned output for a real package manager, and no test suite
// upgrades the machine it is running on. The fake lives in
// internal/exec/exectest.
//
// It sits beside internal/core at the bottom of the module and imports nothing
// else in it. core describes what a run means; this package describes how a
// process is started. Neither needs the other.
//
// # Argv, never a command line
//
// [Command.Argv] is a single []string and there is no field anywhere here that
// takes a command as a string. This is the strongest form of the rule in
// docs/ARCHITECTURE.md: there is no quoting that is correct on both cmd.exe and
// sh, and interpolating into a shell is an injection surface in a tool that runs
// elevated. A rule reviewers enforce gets broken eventually; an API with no
// string form cannot be.
//
// # Cancellation and deadlines
//
// Every call takes a context, and cancelling it kills the process. A
// per-command [Command.Timeout] is expressed as a context deadline rather than a
// separate mechanism, so both unwind by the same path.
//
// When a command fails, the parent context is examined before the derived one.
// The order decides what a run reports when Ctrl-C lands on a command that also
// had a deadline: both contexts are done, and only checking the parent first
// distinguishes an interrupt, which exits 130, from a timeout, which exits 1.
//
// A timeout surfaces as [TimeoutError], which unwraps to
// context.DeadlineExceeded. See its documentation for why that is required
// rather than merely convenient.
//
// # What "kills the process" means
//
// It means the process and everything it started. A package manager is rarely
// one process: apt drives dpkg, winget hands work to installers and msiexec,
// and killing only the one upall launched leaves the rest running with the
// package database locked. The next run then fails for a reason that has
// nothing to do with anything the user did.
//
// Each command is therefore confined before it starts, and the confinement is
// what gets terminated. On Linux that is a process group of its own, signalled
// with SIGTERM and then SIGKILL if it does not unwind within a grace period,
// because SIGKILL to dpkg mid-transaction leaves a machine needing
// `dpkg --configure -a`. On Windows it is a job object, terminated at once:
// there is no portable polite stop for a non-console process, and inventing one
// that works occasionally is worse than terminating honestly. The asymmetry is
// deliberate and lives in process_linux.go and process_windows.go.
//
// Confinement that cannot be set up degrades rather than fails. The command
// still runs and cancelling it still kills the command; what is lost is the
// reach into what the command spawned. A machine where job objects are
// unavailable should still be able to update itself.
//
// [Command.Timeout] and cancellation both arrive here, so a timed-out command
// takes its descendants with it exactly as an interrupted one does.
//
// # Failure is loud
//
// A command that exits non-zero returns [ExitError]. It is an error and not a
// field on the result because errcheck fails the build on an ignored error,
// while a field is something a provider can simply forget to read — and a
// provider that forgets reports a successful upgrade that did not happen.
// Tolerating a known-good exit code is an explicit errors.As, which is the
// point.
//
// A command that could not start at all, most often because the tool is not
// installed, returns an error wrapping [ErrNotFound]. That is not a failure of
// the machine: it is the answer to Detect, and "not installed" is not an error.
//
// [Output] is returned populated in every one of these cases, because the reason
// a command failed is usually in its output rather than in its error.
//
// # Stdin is closed, always
//
// There is no way to give a command standard input. It is connected to the null
// device, so a package manager that decides to prompt reads EOF and fails
// immediately.
//
// This is a deliberate constraint rather than an omission. A tool that blocks
// forever on a question nobody can see it asking is the worst failure an
// unattended updater has, and the cheapest way to guarantee it cannot happen is
// to leave nothing for it to wait on.
//
// # What is captured
//
// stdout and stderr are captured separately and returned as bytes. Neither is
// written to a terminal by this package, nor by anything else below
// internal/cli.
//
// Both are bounded by [MaxCapture]. stdout keeps its beginning, because a parser
// reads from the front; stderr keeps its end, because that is where the error
// that stopped the command is. [Output.Truncated] reports when either had to
// drop anything, so the loss is never silent.
//
// # What is not here yet
//
// Debug logging of argv, duration, and exit code. The logger will be injected
// and will default to discarding, never to slog.Default, which writes to stderr
// and would breach the frontend boundary from the bottom of the module.
//
// The fake runner in internal/exec/exectest, which arrives with it.
//
// # Platforms
//
// This package builds on linux and windows and nowhere else, because process
// confinement has no portable form and each platform's is a separate file.
// macOS is Post-1.0 in the ROADMAP, and a build failure naming the missing file
// is a better answer there than a silent fallback that cancels a run without
// reaching what it started.
//
// Elevation is not here and will not be. Running a command as root or
// Administrator is internal/elevate's job at M7, and it will describe what it
// wants as a [Command] like everything else.
package exec
