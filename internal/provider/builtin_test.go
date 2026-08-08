package provider

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/daniel-kindl/upall/internal/exec/exectest"
)

// TestEveryBuiltinManifestLoads is the test that stops a broken manifest
// shipping.
//
// A manifest is validated when it is loaded, and every other test in this
// package loads the one it is about — so the gap this closes is the manifest
// nobody wrote a test for. Builtin loads all of them, so a new file that is
// added and forgotten is caught here on the first run.
//
// It is also the reason Builtin returns an error rather than skipping what it
// cannot read. A manifest that shipped broken is a bug in the binary, which a
// user cannot fix and should never see.
func TestEveryBuiltinManifestLoads(t *testing.T) {
	registry, err := Builtin(exectest.New())
	if err != nil {
		t.Fatalf("loading the built-in manifests: %v", err)
	}

	names, err := builtinNames()
	if err != nil {
		t.Fatalf("listing the built-in manifests: %v", err)
	}

	// Without this the test passes on an embed pattern that matched nothing,
	// which is the one failure it exists to catch.
	if len(names) == 0 {
		t.Fatal("no manifests are embedded; the go:embed pattern matched nothing")
	}
	if registry.Len() != len(names) {
		t.Errorf("%d manifests embedded but %d providers registered: %v",
			len(names), registry.Len(), registry.IDs())
	}

	t.Logf("built-in providers: %v", registry.IDs())
}

// TestTheEmbeddedManifestsAreTheOnesOnDisk checks that the embed pattern is
// keeping up with the directory.
//
// Embedding is resolved at compile time from a pattern, so a manifest added
// with a name the pattern does not match — a .tml, or a file in a subdirectory —
// compiles cleanly and simply is not there. The provider would then be missing
// from every run, with nothing to notice it.
func TestTheEmbeddedManifestsAreTheOnesOnDisk(t *testing.T) {
	entries, err := os.ReadDir(builtinDir)
	if err != nil {
		t.Fatalf("reading the manifests directory: %v", err)
	}

	var onDisk []string
	for _, entry := range entries {
		if entry.IsDir() {
			t.Errorf("%s is a directory; go:embed manifests/*.toml does not descend into one",
				filepath.Join(builtinDir, entry.Name()))
			continue
		}
		if filepath.Ext(entry.Name()) != ".toml" {
			t.Errorf("%s is not a .toml file, so it is not embedded",
				filepath.Join(builtinDir, entry.Name()))
			continue
		}
		onDisk = append(onDisk, entry.Name())
	}

	embedded, err := builtinNames()
	if err != nil {
		t.Fatalf("listing the embedded manifests: %v", err)
	}

	slices.Sort(onDisk)
	slices.Sort(embedded)
	if !slices.Equal(onDisk, embedded) {
		t.Errorf("the manifests directory holds %v but %v is embedded", onDisk, embedded)
	}
}

// TestBuiltinNeedsNoFilesOnDisk is the criterion itself, stated as an assertion.
//
// The manifests are read from the embedded filesystem rather than from the
// working directory, so the binary carries them. Running from a directory that
// has no manifests in it is the closest a test can get to running the shipped
// binary somewhere else, and it is enough to catch the mistake that matters:
// reading from os.DirFS or a relative path instead of from the embed.
func TestBuiltinNeedsNoFilesOnDisk(t *testing.T) {
	// t.Chdir restores the working directory when the test ends, and marks the
	// test as unable to run in parallel with anything else.
	t.Chdir(t.TempDir())

	registry, err := Builtin(exectest.New())
	if err != nil {
		t.Fatalf("loading the built-in manifests from a directory with none: %v", err)
	}
	if registry.Len() == 0 {
		t.Error("no providers were built from a directory with no manifests in it")
	}
}

// TestBuiltinProvidersAreUsable checks that what Builtin registers is the same
// thing the rest of the system expects, rather than a value that happens to
// satisfy the interface.
func TestBuiltinProvidersAreUsable(t *testing.T) {
	registry, err := Builtin(exectest.New())
	if err != nil {
		t.Fatalf("loading the built-in manifests: %v", err)
	}

	for _, p := range registry.All() {
		if p.ID() == "" {
			t.Error("a built-in provider has no ID")
		}
		if len(p.Platforms()) == 0 {
			t.Errorf("%q declares no platforms, so it would never run", p.ID())
		}
	}
}
