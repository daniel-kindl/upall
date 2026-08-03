// Command upall updates everything on this machine: OS package managers, OS
// updates, and containers, behind one command.
//
// This package is wiring and nothing else. It hands the process arguments to
// internal/cli and exits with the code that comes back. Every decision the
// program makes is made below this line, which is what keeps the CLI and the
// GUI honest about sharing one core.
//
// Run `upall --help` for usage, or see docs/ARCHITECTURE.md for how the pieces
// fit together.
package main
