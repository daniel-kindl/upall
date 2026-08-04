package core

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// modulePath is this module, used to catch an import of anything else in upall.
const modulePath = "github.com/daniel-kindl/upall"

// bannedImports are standard library packages core may not have, each with the
// reason, so a failure explains itself rather than sending the reader here.
var bannedImports = map[string]string{
	"os":       "the domain types may not reach the process, its environment, or its streams",
	"os/exec":  "subprocesses belong to internal/exec, which is the seam tests replace",
	"log":      "nothing below internal/cli may write to a terminal",
	"net":      "core computes; nothing here has a reason to reach the network",
	"net/http": "core computes; nothing here has a reason to reach the network",
}

// sourceFiles parses the package's non-test files.
//
// Test files are excluded deliberately: this file imports go/parser and
// path/filepath, and the guard is about what the package ships, not about what
// proves it.
func sourceFiles(t *testing.T) map[string]*ast.File {
	t.Helper()

	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("listing the package's files: %v", err)
	}

	fset := token.NewFileSet()
	files := make(map[string]*ast.File)
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parsing %s: %v", path, err)
		}
		files[path] = file
	}

	if len(files) == 0 {
		t.Fatal("found no source files to check; the guard would pass by doing nothing")
	}
	return files
}

// TestCoreImportsOnlyTheStandardLibrary enforces the first M2 criterion, that
// core depends on nothing outside the standard library and nothing elsewhere in
// this module.
//
// It is a test rather than a review note because the property is load-bearing
// and silent when broken: an import added here compiles, passes every other
// test, and only shows up later as a dependency cycle or as a type that cannot
// be used from the GUI.
func TestCoreImportsOnlyTheStandardLibrary(t *testing.T) {
	for path, file := range sourceFiles(t) {
		for _, spec := range file.Imports {
			imported, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				t.Fatalf("%s: unparseable import path %s", path, spec.Path.Value)
			}

			if reason, banned := bannedImports[imported]; banned {
				t.Errorf("%s imports %q: %s", path, imported, reason)
				continue
			}

			if imported == modulePath || strings.HasPrefix(imported, modulePath+"/") {
				t.Errorf("%s imports %q; core is the bottom of the module and depends on nothing in it", path, imported)
				continue
			}

			// A standard library import path's first element never contains a
			// dot, because a dot means a domain name, which means somebody
			// else's code.
			if root, _, _ := strings.Cut(imported, "/"); strings.Contains(root, ".") {
				t.Errorf("%s imports %q, which is outside the standard library", path, imported)
			}
		}
	}
}

// TestCoreCannotPrint enforces the second M2 criterion. The frontend boundary in
// docs/ARCHITECTURE.md says nothing below internal/cli may assume a terminal,
// and core is the furthest thing from one.
func TestCoreCannotPrint(t *testing.T) {
	for path, file := range sourceFiles(t) {
		ast.Inspect(file, func(n ast.Node) bool {
			selector, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := selector.X.(*ast.Ident)
			if !ok {
				return true
			}
			if pkg.Name == "fmt" && strings.HasPrefix(selector.Sel.Name, "Print") {
				t.Errorf("%s calls fmt.%s; core returns values and never renders them", path, selector.Sel.Name)
			}
			return true
		})
	}
}
