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

// loadManifest reads one of the shipped manifests from disk.
//
// From disk rather than from the embedded copy because embedding arrives in the
// next change, and because a test that reads the file is checking the file
// rather than a copy of it either way.
func loadManifest(t *testing.T, name string) *Manifest {
	t.Helper()

	path := filepath.Join("manifests", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the manifest: %v", err)
	}

	m, err := Load(name, data)
	if err != nil {
		t.Fatalf("loading %s: %v", name, err)
	}
	return m
}

// wingetProvider builds the shipped winget provider against a fake runner.
func wingetProvider(t *testing.T, runner exec.Runner) (*Manifest, core.Provider) {
	t.Helper()

	m := loadManifest(t, "winget.toml")
	p, err := m.Provider(runner)
	if err != nil {
		t.Fatalf("building the winget provider: %v", err)
	}
	return m, p
}

// TestWingetManifestDeclaresWhatItShould checks the fields nothing else would
// catch. A wrong platform list is a provider that never runs, and a wrong
// elevation flag is a UAC prompt that should not have happened.
func TestWingetManifestDeclaresWhatItShould(t *testing.T) {
	m, p := wingetProvider(t, exectest.New())

	if p.ID() != "winget" {
		t.Errorf("ID is %q, want winget", p.ID())
	}
	if got := m.PlatformSet(); !slices.Equal(got, core.PlatformSet{core.Windows}) {
		t.Errorf("platforms are %v, want [windows]", got)
	}
	if p.NeedsElevation() {
		t.Error("winget declares that it needs elevation; the manifest says otherwise and explains why")
	}

	// The registry's rules are the ones an ID has to survive, so the shipped
	// manifest is checked against them rather than against a guess.
	if err := NewRegistry().Add(p); err != nil {
		t.Errorf("the registry refuses the shipped winget provider: %v", err)
	}
}

// TestWingetPlansFromRealOutput drives the shipped manifest against output
// captured from winget 1.29.280, which is the end-to-end half of the M4
// criterion that CI can run: the manifest's own parser configuration, applied to
// bytes the real tool actually wrote.
func TestWingetPlansFromRealOutput(t *testing.T) {
	captured, err := os.ReadFile(filepath.Join("testdata", "winget-upgrade.txt"))
	if err != nil {
		t.Fatalf("reading the fixture: %v", err)
	}

	m := loadManifest(t, "winget.toml")
	runner := exectest.New().On(m.Plan.Command, exectest.Response{Stdout: string(captured)})

	p, err := m.Provider(runner)
	if err != nil {
		t.Fatalf("building the provider: %v", err)
	}

	got, err := p.Plan(t.Context())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	want := []core.Update{
		{
			Name:      "Epic Online Services",
			ID:        "EpicGames.EpicOnlineServices",
			Installed: "4.1.0",
			Available: "4.3.1",
		},
		{
			Name:      "Logitech G HUB",
			ID:        "Logitech.GHUB",
			Installed: "2025.9.814157",
			Available: "2026.4.919028",
		},
	}
	if !slices.Equal(got, want) {
		t.Errorf("the shipped manifest planned\n%+v\nwant\n%+v", got, want)
	}
}

// TestWingetRunsTheArgvItDeclares is the reason exectest matches on the whole
// argv rather than on the program name.
//
// A manifest's entire job is to say what to run. The flags here are not
// decoration: without --accept-source-agreements winget asks a question, and
// upall gives its commands no standard input, so the command fails rather than
// waits. A test that let a near miss pass would be checking nothing.
func TestWingetRunsTheArgvItDeclares(t *testing.T) {
	wantDetect := []string{"winget", "--version"}
	wantPlan := []string{
		"winget", "upgrade",
		"--accept-source-agreements",
		"--disable-interactivity",
	}
	wantApply := []string{
		"winget", "upgrade",
		"--all",
		"--silent",
		"--accept-source-agreements",
		"--accept-package-agreements",
		"--disable-interactivity",
	}

	runner := exectest.New().
		On(wantDetect, exectest.Response{Stdout: "v1.29.280\n"}).
		On(wantPlan, exectest.Response{Stdout: "Name   Id   Version   Available\n"}).
		On(wantApply, exectest.Response{})

	_, p := wingetProvider(t, runner)
	ctx := t.Context()

	present, err := p.Detect(ctx)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if !present {
		t.Fatal("Detect said winget is absent against a fake that answered")
	}

	if _, err := p.Plan(ctx); err != nil {
		t.Fatalf("Plan: %v", err)
	}

	// A non-empty list, because Apply with nothing to do deliberately runs
	// nothing and would prove the wrong thing here.
	if err := p.Apply(ctx, []core.Update{{Name: "anything"}}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// The fake fails an unregistered argv, so reaching here already proves the
	// three commands matched. This says which one is missing if they stop.
	for _, argv := range [][]string{wantDetect, wantPlan, wantApply} {
		if !runner.Ran(argv...) {
			t.Errorf("winget never ran %v", argv)
		}
	}
}

// TestWingetPlanChangesNothing is the read-only half of the plan contract,
// checked against the shipped argv rather than against the documentation.
//
// It is a weak test by construction — it can only assert that the flags which
// change a machine are absent — but it is the one that would catch --all being
// pasted into the wrong step, which is the mistake this manifest is one
// character away from at all times.
func TestWingetPlanChangesNothing(t *testing.T) {
	m := loadManifest(t, "winget.toml")

	mutating := []string{"--all", "--silent", "install", "uninstall"}
	for _, arg := range m.Plan.Command {
		if slices.Contains(mutating, arg) {
			t.Errorf("the plan command contains %q, which changes the machine: %v", arg, m.Plan.Command)
		}
	}
}

// TestWingetIsAbsentWithoutError covers the M4 criterion from the faked side, so
// that it is checked on both operating systems. The unfaked side, on the OS
// where winget genuinely does not exist, is in winget_absent_test.go.
func TestWingetIsAbsentWithoutError(t *testing.T) {
	runner := exectest.New().Default(exectest.Response{Err: exec.ErrNotFound})

	_, p := wingetProvider(t, runner)

	present, err := p.Detect(t.Context())
	if err != nil {
		t.Errorf("Detect returned an error for an absent tool: %v", err)
	}
	if present {
		t.Error("Detect reported winget as present on a machine without it")
	}
}
