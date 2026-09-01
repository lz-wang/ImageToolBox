package cmd

import (
	"fmt"

	"github.com/urfave/cli/v3"
)

func sourceDestinationArgs(cmd *cli.Command, requireDestination bool) (src, dst string, err error) {
	count := cmd.NArg()
	if count < 1 || count > 2 || (requireDestination && count != 2) {
		if requireDestination {
			return "", "", fmt.Errorf("需要提供 <src> <dst>")
		}
		return "", "", fmt.Errorf("需要提供 <src> [dst]")
	}
	return cmd.Args().Get(0), cmd.Args().Get(1), nil
}

func sourceArg(cmd *cli.Command) (string, error) {
	if cmd.NArg() != 1 {
		return "", fmt.Errorf("需要提供 <src>")
	}
	return cmd.Args().Get(0), nil
}
