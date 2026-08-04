package core

import (
	"context"
	"errors"
)

// ErrNeedsElevation is returned by a provider that cannot do what was asked
// without admin or root, when elevation was refused or is unavailable.
//
// [Classify] turns it into [Blocked], which is reported rather than retried: the
// provider is named, the command to run manually is shown, and the rest of the
// run proceeds. Wrap it with fmt.Errorf and %w to add context; errors.Is finds
// it either way.
var ErrNeedsElevation = errors.New("needs elevation")

// Classify turns an error from a provider into the [Outcome] it represents.
//
// It is the single place upall decides what a failure was, so that a timeout
// looks the same whichever provider hit it and both frontends can render from
// the outcome instead of matching on error strings.
//
// The mapping:
//
//	nil                       → Succeeded
//	context.Canceled          → Cancelled
//	context.DeadlineExceeded  → TimedOut
//	ErrNeedsElevation         → Blocked
//	anything else             → Failed
//
// Matching is by errors.Is, so wrapped errors classify as what they wrap. There
// is deliberately no upall-specific timeout error: a per-command deadline in
// internal/exec is a context deadline, and it must wrap
// context.DeadlineExceeded rather than introduce a second thing meaning the same
// thing that this function would then have to know about too.
//
// Note that Classify never returns [Absent]. "Not installed" is not an error and
// does not arrive here: it is [Provider.Detect] returning false, which the
// pipeline records directly.
func Classify(err error) Outcome {
	switch {
	case err == nil:
		return Succeeded
	case errors.Is(err, context.Canceled):
		return Cancelled
	case errors.Is(err, context.DeadlineExceeded):
		return TimedOut
	case errors.Is(err, ErrNeedsElevation):
		return Blocked
	default:
		return Failed
	}
}
