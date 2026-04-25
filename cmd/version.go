package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of owui-proxy",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("owui-proxy %s\n", Version)
		if Commit != "none" && Commit != "" {
			fmt.Printf("  commit:     %s\n", Commit)
		}
		if BuildDate != "unknown" && BuildDate != "" {
			fmt.Printf("  built:      %s\n", BuildDate)
		}
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
