//go:build !windows

package provider

import (
	"testing"

	"github.com/daniel-kindl/upall/internal/exec"
)

// TestWingetDetectsAsAbsentForReal is the M4 criterion that Detect returns false
// rather than an error when the tool is absent, "proven by a test on the OS
// where the tool does not exist".
//
// It is the one test in this package that uses the real runner, and that is the
// point: faking exec.ErrNotFound proves that the adapter handles the sentinel,
// while this proves the sentinel is what a missing tool actually produces. Those
// are different claims, and only the second one fails when the standard library
// changes what it returns for a program that is not on PATH.
//
// It invokes no package manager, because there is none to invoke — winget ships
// with Windows and this file is excluded there by its build tag. Nothing is
// started, nothing is mutated, and the machine is not touched. That is what
// makes it compatible with the rule in AGENTS.md, which exists to stop a test
// suite from upgrading the machine running it.
//
// The apt half of the same criterion is in apt_absent_test.go, tagged the other
// way.
func TestWingetDetectsAsAbsentForReal(t *testing.T) {
	// A nil logger discards. Nothing below internal/cli may write to a
	// terminal, and a test is no exception.
	_, p := wingetProvider(t, exec.New(nil))

	present, err := p.Detect(t.Context())
	if err != nil {
		t.Errorf("Detect returned an error where winget does not exist: %v", err)
	}
	if present {
		t.Error("Detect reported winget as present on a machine that cannot have it")
	}
}
