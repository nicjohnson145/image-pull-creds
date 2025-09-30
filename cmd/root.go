package cmd

import "github.com/spf13/cobra"

func Root() *cobra.Command {
	cmd := &cobra.Command{
		Use: "image-pull-creds",
	}

	cmd.AddCommand(Serve())

	return cmd
}
