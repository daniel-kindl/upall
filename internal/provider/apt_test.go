package provider

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/daniel-kindl/upall/internal/core"
	"github.com/daniel-kindl/upall/internal/exec"
	"github.com/daniel-kindl/upall/internal/exec/exectest"
)

// aptProvider builds the shipped apt provider against a fake runner.
func aptProvider(t *testing.T, runner exec.Runner) (*Manifest, core.Provider) {
	t.Helper()

	m := loadManifest(t, "apt.toml")
	p, err := m.Provider(runner)
	if err != nil {
		t.Fatalf("building the apt provider: %v", err)
	}
	return m, p
}

// TestAptManifestDeclaresWhatItShould checks the fields nothing else catches.
//
// The elevation flag is the one that matters here and it points the other way
// from winget's: apt-get upgrade writes under /usr and needs root, so a run that
// did not elevate would fail every provider that mattered on a Linux box.
func TestAptManifestDeclaresWhatItShould(t *testing.T) {
	m, p := aptProvider(t, exectest.New())

	if p.ID() != "apt" {
		t.Errorf("ID is %q, want apt", p.ID())
	}
	if got := m.PlatformSet(); !slices.Equal(got, core.PlatformSet{core.Linux}) {
		t.Errorf("platforms are %v, want [linux]", got)
	}
	if !p.NeedsElevation() {
		t.Error("apt does not declare elevation; apt-get upgrade needs root")
	}

	if err := NewRegistry().Add(p); err != nil {
		t.Errorf("the registry refuses the shipped apt provider: %v", err)
	}
}

// TestAptPlansFromRealOutput drives the shipped manifest against output captured
// from apt-get 2.4.11 inside ubuntu:jammy-20240111.
//
// The preamble is the part worth having a real fixture for. apt-get's simulate
// prints four lines of NOTE about being a simulation, then three more about
// reading lists and building a tree, and an unanchored pattern reports those as
// packages.
func TestAptPlansFromRealOutput(t *testing.T) {
	captured, err := os.ReadFile(filepath.Join("testdata", "apt-simulate-upgrade.txt"))
	if err != nil {
		t.Fatalf("reading the fixture: %v", err)
	}

	m := loadManifest(t, "apt.toml")
	runner := exectest.New().On(m.Plan.Command, exectest.Response{Stdout: string(captured)})

	p, err := m.Provider(runner)
	if err != nil {
		t.Fatalf("building the provider: %v", err)
	}

	got, err := p.Plan(t.Context())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	// The fixture holds 52 Inst lines. Conf lines name the same packages again,
	// so a pattern that matched those too would report 104 updates.
	if len(got) != 52 {
		t.Errorf("planned %d updates, want 52", len(got))
	}

	for _, u := range got {
		if u.ID == "" || u.Installed == "" || u.Available == "" {
			t.Errorf("an update came out incomplete: %+v", u)
		}
		if u.Name != u.ID {
			t.Errorf("apt reports one name per package, so Name should equal ID: %+v", u)
		}
	}

	// A package spelled out, including the epoch in its version, which is where
	// a pattern that assumed digits would stop matching.
	want := core.Update{
		Name:      "libc6",
		ID:        "libc6",
		Installed: "2.35-0ubuntu3.6",
		Available: "2.35-0ubuntu3.14",
	}
	i := slices.IndexFunc(got, func(u core.Update) bool { return u.ID == "libc6" })
	if i < 0 {
		t.Fatal("libc6 is in the fixture but was not planned")
	}
	if got[i] != want {
		t.Errorf("libc6 planned as %+v, want %+v", got[i], want)
	}

	// Nothing from the preamble reached the plan.
	for _, u := range got {
		switch u.ID {
		case "NOTE:", "Reading", "Building", "Calculating", "Inst":
			t.Errorf("a preamble line was planned as a package: %+v", u)
		}
	}
}

// TestAptPlansNothingWhenThereIsNothing covers a machine that is up to date,
// captured from ubuntu:22.04, which the official image rebuilds with its updates
// already applied. The simulate still prints its whole preamble, so this is the
// case where an unanchored pattern would invent updates out of nothing.
func TestAptPlansNothingWhenThereIsNothing(t *testing.T) {
	captured, err := os.ReadFile(filepath.Join("testdata", "apt-simulate-no-updates.txt"))
	if err != nil {
		t.Fatalf("reading the fixture: %v", err)
	}

	m := loadManifest(t, "apt.toml")
	runner := exectest.New().On(m.Plan.Command, exectest.Response{Stdout: string(captured)})

	p, err := m.Provider(runner)
	if err != nil {
		t.Fatalf("building the provider: %v", err)
	}

	got, err := p.Plan(t.Context())
	if err != nil {
		t.Fatalf("Plan on an up-to-date machine returned an error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("planned %d updates on an up-to-date machine: %+v", len(got), got)
	}
}

// TestAptRunsTheArgvItDeclares pins the commands and the environment.
//
// The environment is not incidental. LC_ALL=C is what keeps the pattern a
// statement about apt rather than about the machine's language, and without
// DEBIAN_FRONTEND and NEEDRESTART_MODE an upgrade stops on a prompt that upall
// gives no standard input to answer.
func TestAptRunsTheArgvItDeclares(t *testing.T) {
	wantDetect := []string{"apt-get", "--version"}
	wantPlan := []string{"apt-get", "--simulate", "upgrade"}
	wantApply := []string{"apt-get", "upgrade", "--yes"}

	runner := exectest.New().
		On(wantDetect, exectest.Response{Stdout: "apt 2.4.11 (amd64)\n"}).
		On(wantPlan, exectest.Response{Stdout: "NOTE: This is only a simulation!\n"}).
		On(wantApply, exectest.Response{})

	_, p := aptProvider(t, runner)
	ctx := t.Context()

	present, err := p.Detect(ctx)
	if err != nil || !present {
		t.Fatalf("Detect returned (%v, %v), want (true, nil)", present, err)
	}
	if _, err := p.Plan(ctx); err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if err := p.Apply(ctx, []core.Update{{Name: "anything"}}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	calls := runner.Calls()
	if len(calls) != 3 {
		t.Fatalf("ran %d commands, want 3", len(calls))
	}

	// Detect asks a question that needs no locale pinning, so it carries no
	// environment; the other two do.
	if got := calls[0].Env; len(got) != 0 {
		t.Errorf("the detect command carries env %v, want none", got)
	}
	if got := calls[1].Env; !slices.Contains(got, "LC_ALL=C") {
		t.Errorf("the plan command does not pin the locale: %v", got)
	}
	for _, want := range []string{"DEBIAN_FRONTEND=noninteractive", "NEEDRESTART_MODE=a"} {
		if !slices.Contains(calls[2].Env, want) {
			t.Errorf("the apply command is missing %s: %v", want, calls[2].Env)
		}
	}
}

// TestAptPlanChangesNothing is the read-only half of the plan contract.
//
// --simulate is the entire reason the plan command is safe to run before the
// user has confirmed anything, so it is asserted rather than assumed, and the
// flags that would install something are checked for by name.
func TestAptPlanChangesNothing(t *testing.T) {
	m := loadManifest(t, "apt.toml")

	if !slices.Contains(m.Plan.Command, "--simulate") {
		t.Errorf("the plan command has no --simulate, so it is not read-only: %v", m.Plan.Command)
	}
	for _, arg := range []string{"--yes", "-y", "install", "remove"} {
		if slices.Contains(m.Plan.Command, arg) {
			t.Errorf("the plan command contains %q, which changes the machine: %v", arg, m.Plan.Command)
		}
	}
}

// TestAptIsAbsentWithoutError covers the absent case from the faked side, so it
// is checked on both operating systems. The unfaked side, on the OS where apt
// genuinely does not exist, is in apt_absent_test.go.
func TestAptIsAbsentWithoutError(t *testing.T) {
	runner := exectest.New().Default(exectest.Response{Err: exec.ErrNotFound})

	_, p := aptProvider(t, runner)

	present, err := p.Detect(t.Context())
	if err != nil {
		t.Errorf("Detect returned an error for an absent tool: %v", err)
	}
	if present {
		t.Error("Detect reported apt as present on a machine without it")
	}
}
