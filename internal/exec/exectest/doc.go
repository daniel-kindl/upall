// Package exectest provides a fake [exec.Runner] for tests.
//
// It is the other half of the seam. internal/exec exists so that every
// subprocess in upall goes through one place; this package is what that place
// is replaced with when the code under test would otherwise run a real package
// manager. AGENTS.md puts it plainly: no test invokes a real package manager,
// because a test suite that mutates the machine running it is not a test suite.
//
// It is a subpackage rather than a file in internal/exec so that the fake does
// not ship in the binary. [Fake] satisfies an interface the production code
// uses, so the linker cannot prove it unreachable, and test scaffolding
// compiled into a tool that runs as root is worth avoiding for its own sake.
// Living outside the package also keeps the fake honest: it can only reach the
// exported API, so it cannot pretend to do something a real caller could not.
//
// # Using it
//
// File a response against the exact argv a provider should run, then assert on
// what it actually ran:
//
//	runner := exectest.New().
//		On([]string{"apt", "list", "--upgradable"}, exectest.Response{Stdout: fixture}).
//		On([]string{"apt-get", "upgrade", "-y"}, exectest.Response{ExitCode: 100})
//
//	// ... exercise the provider with runner ...
//
//	if !runner.Ran("apt", "list", "--upgradable") {
//		t.Error("the provider never asked apt what was upgradable")
//	}
//
// Matching is on the whole argv, exactly. That is deliberate, and it is most of
// what a manifest test at M4 is actually checking: not that a provider ran
// something, but that it built the argv it claims to. A fake that matched on a
// prefix or a program name would pass for a provider that quietly dropped a
// flag.
//
// An argv nothing was filed against is an error naming what was run and what
// was registered, unless [Fake.Default] set a fallback. A test whose provider
// runs a command it did not expect should fail rather than receive a silent
// success.
//
// # Concurrency
//
// A Fake is safe for concurrent use, which is not a nicety. The pipeline runs
// detect and plan concurrently across providers from M5, they share one runner,
// and CI runs go test -race. [Fake.Calls] returns a copy, and recorded commands
// clone their argv and environment, because the caller owns those and may reuse
// the backing array the moment Run returns.
//
// Registration is guarded too. The documented shape is to configure a Fake and
// then share it, but a test that does otherwise should fail its own assertion
// rather than trip the race detector somewhere unrelated.
package exectest
