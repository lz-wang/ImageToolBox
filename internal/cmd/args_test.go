package cmd

import (
	"context"
	"testing"

	"github.com/urfave/cli/v3"
)

func TestSourceDestinationArgs(t *testing.T) {
	for _, tt := range []struct {
		name     string
		args     []string
		required bool
		wantErr  bool
	}{
		{"optional destination omitted", []string{"input.png"}, false, false},
		{"optional destination present", []string{"input.png", "output.png"}, false, false},
		{"required destination omitted", []string{"input.png"}, true, true},
		{"too many paths", []string{"a", "b", "c"}, false, true},
		{"no paths", nil, false, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			app := &cli.Command{Action: func(_ context.Context, cmd *cli.Command) error {
				_, _, err := sourceDestinationArgs(cmd, tt.required)
				if (err != nil) != tt.wantErr {
					t.Fatalf("sourceDestinationArgs() error = %v, wantErr %v", err, tt.wantErr)
				}
				return nil
			}}
			if err := app.Run(context.Background(), append([]string{"itb"}, tt.args...)); err != nil {
				t.Fatalf("run command: %v", err)
			}
		})
	}
}

func TestSourceArg(t *testing.T) {
	for _, args := range [][]string{{"input.png"}, nil, {"input.png", "output.png"}} {
		app := &cli.Command{Action: func(_ context.Context, cmd *cli.Command) error {
			_, err := sourceArg(cmd)
			return err
		}}
		err := app.Run(context.Background(), append([]string{"itb"}, args...))
		if (len(args) == 1) != (err == nil) {
			t.Fatalf("sourceArg(%v) error = %v", args, err)
		}
	}
}
