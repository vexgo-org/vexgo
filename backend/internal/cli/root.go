package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newRootCmd builds the vexgo root command.
// And provides a `--version` flag to display
// version information.
func newRootCmd(version string) *cobra.Command {
	root := &cobra.Command{
		Use:           "vexgo",
		Short:         "Self-hosted blog CMS server",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if showVersion, _ := cmd.Flags().GetBool("version"); showVersion {
				_, err := fmt.Fprintf(cmd.OutOrStdout(), "vexgo %s\n", version)
				return err
			}
			return cmd.Help()
		},
	}

	root.Flags().BoolP("version", "V", false, "print version and exit")
	return root
}
