package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of owui-proxy",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("owui-proxy %s\n", version)
		fmt.Printf("  commit:     %s\n", commit)
		fmt.Printf("  built:      %s\n", buildDate)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
