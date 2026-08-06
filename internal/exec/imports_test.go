package exec

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// modulePath is this module, used to catch an import of anything else in upall.
const modulePath = "github.com/daniel-kindl/upall"

// bannedImports are packages this one may not have, each with the reason, so a
// failure explains itself rather than sending the reader here.
//
// os and os/exec are conspicuously absent: this is the one package permitted to
// have them, which is the whole point of it existing.
var bannedImports = map[string]string{
	"log":      "logging here is log/slog with an injected logger; the log package writes to stderr",
	"net":      "this package starts processes; nothing here has a reason to reach the network",
	"net/http": "this package starts processes; nothing here has a reason to reach the network",
}

// allowedNonStdlib are the third-party import prefixes this package may have.
//
// It is one entry, and it is here rather than in a review comment because
// widening it is a decision: everything below internal/cli has to build on both
// operating systems from a stock toolchain, and each dependency added here is
// one every provider inherits.
var allowedNonStdlib = []string{
	// The Windows job object API, which is how a cancelled run kills the
	// installers a package manager spawned rather than only the package manager.
	"golang.org/x/sys/",
}

// sourceFiles parses the package's non-test files.
//
// Test files are excluded deliberately: this file imports go/parser and
// path/filepath, and exec_test.go imports internal/core to check the seam
// between the two. The guard is about what the package ships, not about what
// proves it.
//
// Build constraints are not applied, so the windows and linux files are both
// checked whichever machine the suite is running on.
func sourceFiles(t *testing.T) map[string]*ast.File {
	t.Helper()

	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("listing the package's files: %v", err)
	}

	fset := token.NewFileSet()
	files := make(map[string]*ast.File)
	for _, p := range paths {
		if strings.HasSuffix(p, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, p, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parsing %s: %v", p, err)
		}
		files[p] = file
	}

	if len(files) == 0 {
		t.Fatal("found no source files to check; the guard would pass by doing nothing")
	}
	return files
}

// TestExecDependsOnNothingInThisModule keeps this package at the bottom
// alongside internal/core.
//
// core says what a run means and this says how a process is started, and neither
// needs the other. Importing core from here would make the seam every test
// replaces depend on the vocabulary, and the first provider to want a domain
// type in an argv would turn that into a cycle.
func TestExecDependsOnNothingInThisModule(t *testing.T) {
	for name, file := range sourceFiles(t) {
		for _, spec := range file.Imports {
			imported, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				t.Fatalf("%s: unparseable import path %s", name, spec.Path.Value)
			}

			if reason, banned := bannedImports[imported]; banned {
				t.Errorf("%s imports %q: %s", name, imported, reason)
				continue
			}

			if imported == modulePath || strings.HasPrefix(imported, modulePath+"/") {
				t.Errorf("%s imports %q; exec sits beside core at the bottom and depends on nothing in this module", name, imported)
				continue
			}

			// A standard library import path's first element never contains a
			// dot, because a dot means a domain name, which means somebody
			// else's code.
			if root, _, _ := strings.Cut(imported, "/"); !strings.Contains(root, ".") {
				continue
			}
			if !allowed(imported) {
				t.Errorf("%s imports %q, which is outside the standard library and not on the allowed list", name, imported)
			}
		}
	}
}

// allowed reports whether a third-party import path is one this package may
// have.
func allowed(imported string) bool {
	for _, prefix := range allowedNonStdlib {
		if strings.HasPrefix(imported, prefix) {
			return true
		}
	}
	return false
}

// TestExecCannotWriteToATerminal enforces the frontend boundary from the bottom
// of the module. Nothing below internal/cli may assume a terminal, and this
// package holds the streams of every subprocess in upall: it captures them and
// hands them back, and a debugging print left here would appear in the middle of
// the GUI's output with nowhere to go.
func TestExecCannotWriteToATerminal(t *testing.T) {
	// The selectors that reach a terminal, each as package and member.
	banned := map[string]map[string]string{
		"fmt": {
			"Print": "captured output is returned, never rendered here",
		},
		"os": {
			"Stdout": "the command's streams are captured; this package owns neither of upall's own",
			"Stderr": "the command's streams are captured; this package owns neither of upall's own",
			"Stdin":  "commands are given no standard input, and this package never reads upall's",
		},
		"slog": {
			// The one that would slip past every import check, because
			// log/slog is a package this one legitimately has.
			"Default": "slog.Default writes to standard error; New takes a logger and discards when given none",
		},
	}

	for name, file := range sourceFiles(t) {
		ast.Inspect(file, func(n ast.Node) bool {
			// The builtins, which are called bare and so are an unqualified
			// identifier in call position rather than a selector.
			if call, ok := n.(*ast.CallExpr); ok {
				if fun, ok := call.Fun.(*ast.Ident); ok && (fun.Name == "print" || fun.Name == "println") {
					t.Errorf("%s calls the %s builtin, which writes to stderr", name, fun.Name)
				}
			}

			selector, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := selector.X.(*ast.Ident)
			if !ok {
				return true
			}

			members, watched := banned[pkg.Name]
			if !watched {
				return true
			}
			for member, reason := range members {
				if selector.Sel.Name == member || (member == "Print" && strings.HasPrefix(selector.Sel.Name, "Print")) {
					t.Errorf("%s uses %s.%s: %s", name, pkg.Name, selector.Sel.Name, reason)
				}
			}
			return true
		})
	}
}

// TestNothingElseRunsSubprocesses is the first M3 criterion, that all subprocess
// execution in the codebase goes through this package.
//
// It is a test rather than a review note because the property is worth exactly
// as much as its weakest violation. One package reaching for os/exec directly is
// one command no test can fake, and the suite goes on passing while quietly no
// longer covering it.
func TestNothingElseRunsSubprocesses(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("locating the module root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("%s is not the module root: %v", root, err)
	}

	// Slash-separated throughout, because that is what io/fs walks in, on every
	// platform.
	const self = "internal/exec"

	fset := token.NewFileSet()
	checked := 0

	err = fs.WalkDir(os.DirFS(root), ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch {
			case name == ".":
				// The root itself, which the rule below would otherwise skip,
				// ending the walk before it started.
				return nil
			case strings.HasPrefix(d.Name(), "."), strings.HasPrefix(d.Name(), "_"),
				d.Name() == "bin", d.Name() == "testdata":
				// The same directories the Go toolchain itself ignores, plus
				// build output.
				return fs.SkipDir
			}
			return nil
		}
		if path.Ext(name) != ".go" {
			return nil
		}
		// Only this package is exempt, matched on the file's own directory
		// rather than by pruning the tree. Skipping the directory would take
		// internal/exec/exectest with it, and the fake has no more business
		// starting a process than anything else does.
		if path.Dir(name) == self {
			return nil
		}
		// Test files are checked too, unlike in the guards above. Those ask
		// what this package ships; this one asks what the repository is allowed
		// to start, and AGENTS.md answers for tests as well: no test invokes a
		// real package manager. A test reaching for os/exec directly is exactly
		// how that stops being true.

		file, err := parser.ParseFile(fset, filepath.Join(root, filepath.FromSlash(name)), nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		checked++

		for _, spec := range file.Imports {
			imported, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return err
			}
			if imported == "os/exec" {
				t.Errorf("%s imports os/exec; every subprocess goes through internal/exec, which is the seam tests replace", name)
			}
		}

		ast.Inspect(file, func(n ast.Node) bool {
			selector, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := selector.X.(*ast.Ident)
			if !ok {
				return true
			}
			if pkg.Name == "os" && selector.Sel.Name == "StartProcess" {
				t.Errorf("%s calls os.StartProcess; every subprocess goes through internal/exec", name)
			}
			return true
		})

		return nil
	})
	if err != nil {
		t.Fatalf("walking the module: %v", err)
	}

	if checked == 0 {
		t.Fatal("found no source files outside internal/exec; the guard would pass by doing nothing")
	}
}
