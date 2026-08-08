//go:build !linux

package provider

import (
	"testing"

	"github.com/daniel-kindl/upall/internal/exec"
)

// TestAptDetectsAsAbsentForReal is the Windows half of the M4 criterion that
// Detect returns false rather than an error when the tool is absent, "proven by
// a test on the OS where the tool does not exist".
//
// The reasoning is on TestWingetDetectsAsAbsentForReal, which is the same test
// tagged the other way. Between them the criterion is proven on both platforms
// CI runs, each against the tool the other one has.
//
// No package manager is invoked, because apt is not there to invoke. Nothing is
// started and nothing is mutated.
func TestAptDetectsAsAbsentForReal(t *testing.T) {
	// A nil logger discards. Nothing below internal/cli may write to a
	// terminal, and a test is no exception.
	_, p := aptProvider(t, exec.New(nil))

	present, err := p.Detect(t.Context())
	if err != nil {
		t.Errorf("Detect returned an error where apt does not exist: %v", err)
	}
	if present {
		t.Error("Detect reported apt as present on a machine that cannot have it")
	}
}
