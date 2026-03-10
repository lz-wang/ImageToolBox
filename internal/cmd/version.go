package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "显示版本信息",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("imagetoolbox version %s\n", version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
