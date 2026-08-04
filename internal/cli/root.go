package cli

import (
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/daniel-kindl/upall/internal/core"
)

const rootLong = `upall updates everything on this machine: OS package managers, OS updates, and
containers, behind one command.

It runs unprivileged by default and elevates only the providers that say they
need it. Nothing changes until you have seen the plan and answered a prompt.

This build is the skeleton. The commands that plan and apply updates are not in
it yet; see docs/ROADMAP.md for what lands when.`

// Execute parses args, runs the command they select, and returns the exit code
// the process should terminate with. Pass os.Args[1:].
//
// The codes are [core.ExitCode] values, which is where the contract lives; this
// package chooses among them rather than defining them.
//
// It does not call os.Exit. Keeping process lifetime with the caller is what
// lets tests assert on the code, and what keeps cmd/upall down to one line.
// Anything the user needs to see, including errors, has been written to stdout
// or stderr by the time this returns.
func Execute(args []string) int {
	return execute(args, os.Stdout, os.Stderr)
}

// execute is [Execute] with the terminal handed in, so tests can read what was
// written instead of letting it escape into the test log.
func execute(args []string, out, errOut io.Writer) int {
	root := newRootCommand()
	root.SetArgs(args)
	root.SetOut(out)
	root.SetErr(errOut)

	if err := root.Execute(); err != nil {
		// Every error the current tree can produce is cobra rejecting the
		// request itself: an unknown command, or a flag that would not parse.
		// The codes describing a run's outcome arrive with the pipeline that
		// produces one.
		return int(core.ExitUsage)
	}
	return int(core.ExitOK)
}

// newRootCommand builds the upall command tree and returns its root.
//
// The tree is constructed per call rather than kept in a package-level variable
// so that tests can build an isolated one, redirect its output, and run it
// without the flag values of a previous test still attached.
func newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "upall",
		Short: "Update everything on this machine",
		Long:  rootLong,

		// Bare `upall` will mean `upall apply` once there is something to
		// apply. Until then it prints help, which is the honest answer to
		// "what can this do". NoArgs is what turns a mistyped command into
		// exit 2 rather than a silently ignored argument.
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	root.AddCommand(newVersionCommand())

	return root
}
