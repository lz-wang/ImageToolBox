package cmd

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"
)

func newVersionCommand(version string) *cli.Command {
	return &cli.Command{
		Name:     "version",
		Usage:    "Show version information",
		Category: categoryUtility,
		Action: func(ctx context.Context, cmd *cli.Command) error {
			fmt.Printf("itb version %s\n", version)
			return nil
		},
	}
}
