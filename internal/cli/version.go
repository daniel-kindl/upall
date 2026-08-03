package cli

import (
	"github.com/spf13/cobra"

	"github.com/daniel-kindl/upall/internal/buildinfo"
)

const versionLong = `Print which upall this is: the version it claims to be, the commit it was built
from, and when, followed by the toolchain and target that produced it.

A build that was never given a version number calls itself "dev". One built from
a working tree with uncommitted changes marks its commit "-dirty", because the
commit alone would not account for what is in the binary.

Exit codes:
  0  always`

// newVersionCommand returns the `upall version` command.
func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version, commit, and build date",
		Long:  versionLong,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Printed through the command's own writer rather than straight to
			// stdout, so a test can capture it.
			cmd.Println(buildinfo.Get())
			return nil
		},
	}
}
