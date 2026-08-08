package provider

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/daniel-kindl/upall/internal/core"
	"github.com/daniel-kindl/upall/internal/exec"
	"github.com/daniel-kindl/upall/internal/exec/exectest"
)

// The commands both kinds of provider below run, and the output they get back.
// They are shared so that the two providers differ in how they are built and in
// nothing else, which is what the indistinguishability test needs to be about
// the construction rather than about the fixture.
var (
	detectArgv = []string{"tool", "--version"}
	planArgv   = []string{"tool", "list", "--upgradable"}
	applyArgv  = []string{"tool", "upgrade", "--yes"}

	planOutput = "firefox 1.0 -> 2.0\nvim 8.0 -> 9.0\n"

	wantUpdates = []core.Update{
		{Name: "firefox", ID: "firefox", Installed: "1.0", Available: "2.0"},
		{Name: "vim", ID: "vim", Installed: "8.0", Available: "9.0"},
	}
)

// declarative builds the provider from a manifest, which is the common case.
func declarative(t *testing.T, runner exec.Runner) core.Provider {
	t.Helper()

	const manifest = `
id        = "tool"
platforms = ["linux", "windows"]
elevate   = true

[detect]
command = ["tool", "--version"]

[plan]
command = ["tool", "list", "--upgradable"]
parser  = "lines"

  [plan.lines]
  pattern = '^(?P<id>\S+) (?P<installed>\S+) -> (?P<available>\S+)$'

[apply]
command = ["tool", "upgrade", "--yes"]
`

	m, err := Load("tool.toml", []byte(manifest))
	if err != nil {
		t.Fatalf("loading the manifest: %v", err)
	}

	p, err := m.Provider(runner)
	if err != nil {
		t.Fatalf("building the provider: %v", err)
	}
	return p
}

// nativeProvider is what a provider written in Go looks like: it implements
// core.Provider directly, with no manifest anywhere.
//
// It is written the way a real one is — commands through an injected
// exec.Runner, ErrNotFound and a non-zero exit both meaning "not usable" — so
// that comparing it against the adapter compares two implementations rather than
// an implementation against a stub.
type nativeProvider struct {
	runner exec.Runner
}

var _ core.Provider = (*nativeProvider)(nil)

func (n *nativeProvider) ID() string { return "tool" }
func (n *nativeProvider) Platforms() core.PlatformSet {
	return core.PlatformSet{core.Linux, core.Windows}
}
func (n *nativeProvider) NeedsElevation() bool { return true }

func (n *nativeProvider) Detect(ctx context.Context) (bool, error) {
	if _, err := n.runner.Run(ctx, exec.Command{Argv: detectArgv}); err != nil {
		var exit *exec.ExitError
		if errors.Is(err, exec.ErrNotFound) || errors.As(err, &exit) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (n *nativeProvider) Plan(ctx context.Context) ([]core.Update, error) {
	out, err := n.runner.Run(ctx, exec.Command{Argv: planArgv})
	if err != nil {
		return nil, err
	}

	var updates []core.Update
	for _, line := range splitLines(out.Stdout) {
		var id, installed, available string
		if err := scanUpdate(line, &id, &installed, &available); err != nil {
			continue
		}
		updates = append(updates, core.Update{
			Name: id, ID: id, Installed: installed, Available: available,
		})
	}
	return updates, nil
}

func (n *nativeProvider) Apply(ctx context.Context, updates []core.Update) error {
	if len(updates) == 0 {
		return nil
	}
	_, err := n.runner.Run(ctx, exec.Command{Argv: applyArgv})
	return err
}

// scanUpdate reads "<id> <installed> -> <available>", failing on a line that is
// not one. Hand-written rather than a fmt.Sscanf call because the arrow is a
// literal that Sscanf handles awkwardly, and because this is what the native
// half of the comparison is meant to look like: parsing done in Go.
func scanUpdate(line string, id, installed, available *string) error {
	fields := strings.Fields(line)
	if len(fields) != 4 || fields[2] != "->" {
		return errors.New("not an update line")
	}
	*id, *installed, *available = fields[0], fields[1], fields[3]
	return nil
}

// working is a fake where every command succeeds and the plan command prints the
// two updates above.
func working() *exectest.Fake {
	return exectest.New().
		On(detectArgv, exectest.Response{Stdout: "tool 1.0\n"}).
		On(planArgv, exectest.Response{Stdout: planOutput}).
		On(applyArgv, exectest.Response{Stdout: "done\n"})
}

// bothKinds is the pair the tests below run identical assertions against. The
// only difference between them is how they were built.
func bothKinds(t *testing.T, runner exec.Runner) map[string]core.Provider {
	t.Helper()
	return map[string]core.Provider{
		"manifest": declarative(t, runner),
		"native":   &nativeProvider{runner: runner},
	}
}

// TestTheRegistryCannotDistinguishTheTwoKinds is the M4 criterion, and the
// assertion ADR-0002 rests on.
//
// The registry is handed one of each and asked every question it can ask. If a
// manifest provider and a native one were distinguishable, this is where it
// would show: the registry has no type switch, no interface upgrade, and no
// field that says which it got, so the two are the same thing to it or the test
// fails.
func TestTheRegistryCannotDistinguishTheTwoKinds(t *testing.T) {
	runner := working()

	manifest := declarative(t, runner)
	native := &nativeProvider{runner: runner}

	// Different IDs only so that both fit in one registry; everything else
	// about them is identical by construction.
	renamed := renameTo(t, "tool-native", native)

	r := NewRegistry()
	for _, p := range []core.Provider{manifest, renamed} {
		if err := r.Add(p); err != nil {
			t.Fatalf("registering %q: %v", p.ID(), err)
		}
	}

	if got := r.Len(); got != 2 {
		t.Fatalf("the registry holds %d providers, want 2", got)
	}

	for _, id := range []string{"tool", "tool-native"} {
		p, found := r.Lookup(id)
		if !found {
			t.Fatalf("%q did not resolve", id)
		}

		// Everything the registry and the pipeline ever ask, answered the same
		// way by both.
		if !p.Platforms().Supports(core.Linux) || !p.Platforms().Supports(core.Windows) {
			t.Errorf("%q supports %v", id, p.Platforms())
		}
		if !p.NeedsElevation() {
			t.Errorf("%q does not need elevation", id)
		}
	}

	// The platform filter treats them alike, which is the only decision the
	// registry makes about a provider.
	if got := len(r.For(core.Linux)); got != 2 {
		t.Errorf("the Linux filter kept %d of 2 providers", got)
	}
	if got := len(r.For(core.Darwin)); got != 0 {
		t.Errorf("the Darwin filter kept %d providers, want 0", got)
	}
}

// TestBothKindsBehaveTheSame runs the provider lifecycle against each kind and
// asserts they agree. Indistinguishable to the registry is worth little if they
// diverge the moment something calls them.
func TestBothKindsBehaveTheSame(t *testing.T) {
	for kind, provider := range bothKinds(t, working()) {
		t.Run(kind, func(t *testing.T) {
			ctx := t.Context()

			present, err := provider.Detect(ctx)
			if err != nil || !present {
				t.Fatalf("Detect returned (%v, %v), want (true, nil)", present, err)
			}

			got, err := provider.Plan(ctx)
			if err != nil {
				t.Fatalf("Plan: %v", err)
			}
			if !slices.Equal(got, wantUpdates) {
				t.Errorf("Plan returned\n%+v\nwant\n%+v", got, wantUpdates)
			}

			if err := provider.Apply(ctx, got); err != nil {
				t.Errorf("Apply: %v", err)
			}
		})
	}
}

// TestBothKindsTreatAnAbsentToolTheSame covers the case upall meets most often.
// Most machines have most providers missing, and that is not an error for either
// kind.
func TestBothKindsTreatAnAbsentToolTheSame(t *testing.T) {
	absent := func() *exectest.Fake {
		return exectest.New().Default(exectest.Response{Err: exec.ErrNotFound})
	}

	for kind, provider := range bothKinds(t, absent()) {
		t.Run(kind, func(t *testing.T) {
			present, err := provider.Detect(t.Context())
			if err != nil {
				t.Errorf("Detect on a machine without the tool returned an error: %v", err)
			}
			if present {
				t.Error("Detect reported a tool that is not installed as present")
			}
		})
	}
}

// TestBothKindsTreatABrokenToolTheSame covers a tool that is on PATH and answers
// non-zero: present, not usable, and not a failed run.
func TestBothKindsTreatABrokenToolTheSame(t *testing.T) {
	broken := func() *exectest.Fake {
		return exectest.New().On(detectArgv, exectest.Response{ExitCode: 1, Stderr: "broken\n"})
	}

	for kind, provider := range bothKinds(t, broken()) {
		t.Run(kind, func(t *testing.T) {
			present, err := provider.Detect(t.Context())
			if err != nil {
				t.Errorf("Detect on a broken tool returned an error: %v", err)
			}
			if present {
				t.Error("Detect reported an unusable tool as present")
			}
		})
	}
}

// TestBothKindsPropagateCancellation is the other half of the Detect contract. A
// context that was cancelled means the question could not be answered, which is
// not the same as answering no — otherwise Ctrl-C during detection would be
// reported as a machine with no package managers on it.
func TestBothKindsPropagateCancellation(t *testing.T) {
	cancelled := func() *exectest.Fake {
		return exectest.New().Default(exectest.Response{Err: context.Canceled})
	}

	for kind, provider := range bothKinds(t, cancelled()) {
		t.Run(kind, func(t *testing.T) {
			if _, err := provider.Detect(t.Context()); !errors.Is(err, context.Canceled) {
				t.Errorf("Detect returned %v, want context.Canceled", err)
			}
		})
	}
}

// TestTheManifestBuildsTheArgvItDeclares is why exectest matches on the whole
// argv. A manifest's entire job is to say what to run, so a test that let a near
// miss pass would be checking nothing.
func TestTheManifestBuildsTheArgvItDeclares(t *testing.T) {
	runner := working()
	provider := declarative(t, runner)
	ctx := t.Context()

	if _, err := provider.Detect(ctx); err != nil {
		t.Fatalf("Detect: %v", err)
	}
	updates, err := provider.Plan(ctx)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if err := provider.Apply(ctx, updates); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	for _, argv := range [][]string{detectArgv, planArgv, applyArgv} {
		if !runner.Ran(argv...) {
			t.Errorf("the manifest never ran %v; it ran %v", argv, argvs(runner.Calls()))
		}
	}
	if got := len(runner.Calls()); got != 3 {
		t.Errorf("the manifest ran %d commands, want 3: %v", got, argvs(runner.Calls()))
	}
}

// TestApplyWithNothingToDoRunsNothing keeps a confirmed-but-empty plan from
// raising an elevation prompt for no work.
func TestApplyWithNothingToDoRunsNothing(t *testing.T) {
	for kind, provider := range bothKinds(t, working()) {
		t.Run(kind, func(t *testing.T) {
			if err := provider.Apply(t.Context(), nil); err != nil {
				t.Errorf("Apply with no updates: %v", err)
			}
		})
	}

	// Asserted against the fake rather than the return value, because "did
	// nothing" is the claim.
	runner := working()
	if err := declarative(t, runner).Apply(t.Context(), nil); err != nil {
		t.Fatalf("Apply with no updates: %v", err)
	}
	if got := len(runner.Calls()); got != 0 {
		t.Errorf("Apply with no updates ran %v", argvs(runner.Calls()))
	}
}

// TestManifestEnvReachesTheCommand covers the field apt needs for
// DEBIAN_FRONTEND, since a manifest that declared it and did not pass it on
// would hang on a prompt nobody can see.
func TestManifestEnvReachesTheCommand(t *testing.T) {
	const manifest = `
id        = "tool"
platforms = ["linux"]

[detect]
command = ["tool", "--version"]

[plan]
command = ["tool", "list"]
parser  = "lines"
env     = ["QUIET=1"]

  [plan.lines]
  pattern = '^(?P<id>\S+)$'

[apply]
command = ["tool", "upgrade"]
env     = ["QUIET=1", "ASSUME_YES=1"]
`

	m, err := Load("tool.toml", []byte(manifest))
	if err != nil {
		t.Fatalf("loading: %v", err)
	}

	runner := exectest.New().Default(exectest.Response{Stdout: "firefox\n"})
	p, err := m.Provider(runner)
	if err != nil {
		t.Fatalf("building the provider: %v", err)
	}

	updates, err := p.Plan(t.Context())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if err := p.Apply(t.Context(), updates); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	calls := runner.Calls()
	if len(calls) != 2 {
		t.Fatalf("ran %d commands, want 2", len(calls))
	}
	if got := calls[0].Env; !slices.Equal(got, []string{"QUIET=1"}) {
		t.Errorf("the plan command got env %v, want [QUIET=1]", got)
	}
	if got := calls[1].Env; !slices.Equal(got, []string{"QUIET=1", "ASSUME_YES=1"}) {
		t.Errorf("the apply command got env %v", got)
	}
}

// TestProviderNeedsARunner checks the one thing Provider refuses, since the
// alternative default would be a real runner and a test that silently updated
// the machine running it.
func TestProviderNeedsARunner(t *testing.T) {
	m, err := Load("tool.toml", []byte(validManifest))
	if err != nil {
		t.Fatalf("loading: %v", err)
	}
	if _, err := m.Provider(nil); err == nil {
		t.Error("built a provider with no runner")
	}
}

// argvs renders recorded calls for a failure message.
func argvs(calls []exec.Command) [][]string {
	out := make([][]string, len(calls))
	for i, c := range calls {
		out[i] = c.Argv
	}
	return out
}

// renamed wraps a provider to answer to a different ID, so that two providers
// which are identical by construction can be registered together.
type renamed struct {
	core.Provider
	id string
}

func (r *renamed) ID() string { return r.id }

func renameTo(t *testing.T, id string, p core.Provider) core.Provider {
	t.Helper()
	return &renamed{Provider: p, id: id}
}
