package provider

import (
	"embed"
	"fmt"
	"io/fs"
	"path"

	"github.com/daniel-kindl/upall/internal/exec"
)

// builtinFS holds the manifests upall ships, compiled into the binary.
//
// Embedded rather than read from disk because upall promises to be a downloaded
// binary that runs, with no runtime, no interpreter, and no files beside it. A
// manifest read from a directory at startup would be one more thing to install
// and one more thing to go missing, and — since a manifest is arbitrary command
// execution by definition — one more thing an attacker could put there. These
// cannot be changed without replacing the binary, which is the security model in
// docs/ARCHITECTURE.md.
//
// User-supplied overrides are a separate, off-by-default mechanism and are not
// this.
//
//go:embed manifests/*.toml
var builtinFS embed.FS

// builtinDir is where the manifests sit inside [builtinFS]. embed.FS always uses
// slashes, on every platform, so this is not a filepath.
const builtinDir = "manifests"

// Builtin returns a registry of every provider upall ships, each running its
// commands through runner.
//
// An error means a manifest that shipped is broken, which is a bug in the
// binary rather than something wrong with the machine — so it names the file and
// the field and does not degrade. A user cannot fix it and should not be asked
// to; the test below is what keeps it from ever being seen.
//
// Native providers are not here yet. When the first one arrives at M8 it is
// registered alongside these, and nothing downstream will be able to tell which
// is which.
func Builtin(runner exec.Runner) (*Registry, error) {
	entries, err := fs.ReadDir(builtinFS, builtinDir)
	if err != nil {
		return nil, fmt.Errorf("reading the built-in manifests: %w", err)
	}

	registry := NewRegistry()
	for _, entry := range entries {
		name := entry.Name()

		data, err := builtinFS.ReadFile(path.Join(builtinDir, name))
		if err != nil {
			return nil, fmt.Errorf("reading the built-in manifest %s: %w", name, err)
		}

		manifest, err := Load(name, data)
		if err != nil {
			return nil, err
		}

		provider, err := manifest.Provider(runner)
		if err != nil {
			return nil, err
		}

		if err := registry.Add(provider); err != nil {
			return nil, fmt.Errorf("registering the built-in manifest %s: %w", name, err)
		}
	}

	return registry, nil
}

// builtinNames lists the embedded manifest files, for tests that need to know
// what shipped without loading it.
func builtinNames() ([]string, error) {
	entries, err := fs.ReadDir(builtinFS, builtinDir)
	if err != nil {
		return nil, err
	}

	names := make([]string, len(entries))
	for i, entry := range entries {
		names[i] = entry.Name()
	}
	return names, nil
}
