package provider

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/daniel-kindl/upall/internal/core"
	"github.com/daniel-kindl/upall/internal/exec"
)

// adapter is a [Manifest] wearing the [core.Provider] interface.
//
// It is the whole of what makes a declarative provider indistinguishable from a
// native one, and it is deliberately thin: the manifest has already been
// validated, the parser has already been built, and what is left is running
// three commands and reading one of them. Everything that could have been a
// decision was made at load time, where the error can name a file and a field.
//
// It holds no mutable state, so the registry may hand the same value to the
// goroutines detect and plan fan out into.
type adapter struct {
	manifest *Manifest
	runner   exec.Runner
	parser   Parser
}

// Verify at compile time that a manifest really does satisfy the interface a
// native provider implements. This is the assertion ADR-0002 rests on, and a
// change to core.Provider should fail here rather than somewhere downstream.
var _ core.Provider = (*adapter)(nil)

// Provider returns this manifest as a [core.Provider], running its commands
// through runner.
//
// The runner is injected rather than constructed because it is the seam every
// test replaces: a provider built with internal/exec/exectest runs no
// subprocess, which is what lets a manifest be tested for the argv it builds
// without a package manager being present. A nil runner is a programming error
// rather than a default, since the default would have to be a real one.
func (m *Manifest) Provider(runner exec.Runner) (core.Provider, error) {
	if runner == nil {
		return nil, fmt.Errorf("provider %q: a runner is required", m.ID)
	}

	parser, err := m.Parser()
	if err != nil {
		// Unreachable through Load, which builds the parser to validate it.
		// Reachable through a Manifest assembled in Go, which is why it is
		// checked rather than assumed.
		return nil, &ManifestError{File: m.ID, Field: "plan.parser", Err: err}
	}

	return &adapter{manifest: m, runner: runner, parser: parser}, nil
}

// ID implements [core.Provider].
func (a *adapter) ID() string { return a.manifest.ID }

// Platforms implements [core.Provider].
func (a *adapter) Platforms() core.PlatformSet { return a.manifest.PlatformSet() }

// NeedsElevation implements [core.Provider].
func (a *adapter) NeedsElevation() bool { return a.manifest.Elevate }

// Detect implements [core.Provider].
//
// It runs the detect command and reads the answer from how that went. A tool
// that is not on PATH is [exec.ErrNotFound], and a tool that is there but
// answered non-zero is present and not usable; both are (false, nil), because
// "not installed" is not an error and neither is "installed and broken" — a
// machine with a half-configured package manager should still have the rest of
// its providers run.
//
// Everything else is (false, err): a cancelled context, an exceeded deadline, a
// working directory that does not exist. Those mean the question could not be
// answered, which is a different thing from answering no, and the distinction is
// what stops Ctrl-C from being reported as an absent provider.
func (a *adapter) Detect(ctx context.Context) (bool, error) {
	if _, err := a.run(ctx, a.manifest.Detect); err != nil {
		var exit *exec.ExitError
		if errors.Is(err, exec.ErrNotFound) || errors.As(err, &exit) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Plan implements [core.Provider].
//
// The command is whatever the manifest's plan step says, and it is read-only by
// the manifest's word rather than by anything enforceable here. That is why a
// manifest is reviewed: `[plan] command` is the field where "list what is out of
// date" could be written as something that installs it.
//
// A non-zero exit fails the plan rather than being parsed anyway. Some tools do
// exit non-zero to mean "nothing to update", and the manifest for such a tool
// will need somewhere to say so; inventing that field before a real provider
// needs it would be guessing at its shape.
func (a *adapter) Plan(ctx context.Context) ([]core.Update, error) {
	out, err := a.run(ctx, a.manifest.Plan.Step)
	if err != nil {
		return nil, err
	}
	return a.parser.Parse(out)
}

// Apply implements [core.Provider].
//
// The updates are not passed to the command. Every tool upall drives updates
// everything it knows about in one invocation, and none of them accepts the list
// back, so the argument is what the pipeline confirmed rather than what gets
// spliced into an argv — which is also the only reason a manifest cannot be
// coerced into running an arbitrary package name.
//
// An empty list runs nothing and succeeds. There is nothing to do, and running
// the apply command anyway would raise an elevation prompt for no work.
func (a *adapter) Apply(ctx context.Context, updates []core.Update) error {
	if len(updates) == 0 {
		return nil
	}
	_, err := a.run(ctx, a.manifest.Apply)
	return err
}

// run executes one of the manifest's steps.
func (a *adapter) run(ctx context.Context, step Step) (exec.Output, error) {
	return a.runner.Run(ctx, exec.Command{
		// Cloned because the manifest outlives the call and is shared by every
		// invocation of this provider. Nothing downstream is supposed to write
		// to the argv, and this is what makes it not matter if something does.
		Argv: slices.Clone(step.Command),
		Env:  slices.Clone(step.Env),
	})
}
