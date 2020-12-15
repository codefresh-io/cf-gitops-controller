package cmd

import "github.com/spf13/cobra"

var installCmd = &cobra.Command{}

func init() {
	rootCmd.AddCommand(installCmd)
}
