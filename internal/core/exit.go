package core

// ExitCode is the number upall hands back to the shell.
//
// The set below is a public interface under semver, documented in
// docs/ARCHITECTURE.md, and scripts depend on it. Changing what one of these
// means is a breaking change, so they live here, in one place, rather than being
// spelled out wherever a process happens to end.
type ExitCode int

// The exit codes upall can terminate with.
//
// [ExitOK], [ExitFailure], and [ExitInterrupted] are derived from a finished run
// by [Result.ExitCode]. [ExitUsage] is not: nothing ran, so there is no result
// to derive it from. It is declared here anyway, because the contract is one
// thing and half of it kept somewhere else is half of it free to drift.
const (
	// ExitOK means the run did what was asked. It covers a run that found
	// nothing to update, and a run the user declined at the prompt: in both,
	// upall did exactly what it was for.
	ExitOK ExitCode = 0

	// ExitFailure means one or more providers did not succeed. Others may
	// have. The summary says which.
	ExitFailure ExitCode = 1

	// ExitUsage means the request itself was wrong: an unknown command, a flag
	// that would not parse, a malformed config file, or a non-interactive run
	// that would have had to prompt and was not given --yes. Nothing was
	// applied.
	ExitUsage ExitCode = 2

	// ExitInterrupted means Ctrl-C arrived mid-run. Whatever finished is
	// reported and journaled. 130 is the shell convention for termination by
	// SIGINT, and upall follows it rather than inventing a number.
	ExitInterrupted ExitCode = 130
)

// ExitCode derives the process exit code from a finished run.
//
// The precedence is interrupted, then failed, then OK:
//
//   - any provider [Cancelled] gives [ExitInterrupted], even alongside failures.
//     The run was cut short, so the results are incomplete, and reporting a
//     complete failure would misdescribe what happened.
//   - otherwise anything [Outcome.OK] rejects gives [ExitFailure]. That
//     includes [Blocked] and [TimedOut], which never ran or never finished but
//     are still work the user asked for that did not get done.
//   - otherwise [ExitOK].
//
// A run with no providers at all is [ExitOK]. A machine with none of upall's
// providers installed has nothing to update, which is a successful answer to the
// question rather than a failure to answer it.
func (r Result) ExitCode() ExitCode {
	code := ExitOK
	for _, pr := range r.Providers {
		if pr.Outcome == Cancelled {
			return ExitInterrupted
		}
		if !pr.Outcome.OK() {
			code = ExitFailure
		}
	}
	return code
}
