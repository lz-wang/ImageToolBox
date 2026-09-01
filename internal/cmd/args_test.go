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

func TestSourceDestinationArgsAcceptsFlagsAroundPaths(t *testing.T) {
	for _, args := range [][]string{
		{"--width", "100", "src.jpg", "dst.jpg"},
		{"src.jpg", "--width", "100", "dst.jpg"},
		{"src.jpg", "dst.jpg", "--width", "100"},
	} {
		var src, dst string
		app := &cli.Command{
			Flags: []cli.Flag{&cli.IntFlag{Name: "width"}},
			Action: func(_ context.Context, cmd *cli.Command) error {
				var err error
				src, dst, err = sourceDestinationArgs(cmd, false)
				return err
			},
		}
		if err := app.Run(context.Background(), append([]string{"itb"}, args...)); err != nil {
			t.Fatalf("Run(%v) error = %v", args, err)
		}
		if src != "src.jpg" || dst != "dst.jpg" {
			t.Fatalf("Run(%v) paths = %q, %q", args, src, dst)
		}
	}
}

func TestS3OperandArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		parse   func(*cli.Command) error
		wantErr bool
	}{
		{"upload no operands", nil, func(cmd *cli.Command) error { _, _, err := s3UploadArgs(cmd); return err }, true},
		{"upload source", []string{"input.txt"}, func(cmd *cli.Command) error { _, _, err := s3UploadArgs(cmd); return err }, false},
		{"upload source and key", []string{"input.txt", "objects/input.txt"}, func(cmd *cli.Command) error { _, _, err := s3UploadArgs(cmd); return err }, false},
		{"upload too many operands", []string{"a", "b", "c"}, func(cmd *cli.Command) error { _, _, err := s3UploadArgs(cmd); return err }, true},
		{"download no operands", nil, func(cmd *cli.Command) error { _, _, err := s3DownloadArgs(cmd); return err }, true},
		{"download key", []string{"objects/input.txt"}, func(cmd *cli.Command) error { _, _, err := s3DownloadArgs(cmd); return err }, false},
		{"download key and destination", []string{"objects/input.txt", "output.txt"}, func(cmd *cli.Command) error { _, _, err := s3DownloadArgs(cmd); return err }, false},
		{"download too many operands", []string{"a", "b", "c"}, func(cmd *cli.Command) error { _, _, err := s3DownloadArgs(cmd); return err }, true},
		{"key no operands", nil, func(cmd *cli.Command) error { _, err := s3KeyArg(cmd); return err }, true},
		{"key one operand", []string{"object"}, func(cmd *cli.Command) error { _, err := s3KeyArg(cmd); return err }, false},
		{"key too many operands", []string{"a", "b"}, func(cmd *cli.Command) error { _, err := s3KeyArg(cmd); return err }, true},
		{"prefix no operands", nil, func(cmd *cli.Command) error { _, err := s3PrefixArg(cmd); return err }, false},
		{"prefix one operand", []string{"images/"}, func(cmd *cli.Command) error { _, err := s3PrefixArg(cmd); return err }, false},
		{"prefix too many operands", []string{"a", "b"}, func(cmd *cli.Command) error { _, err := s3PrefixArg(cmd); return err }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &cli.Command{Action: func(_ context.Context, cmd *cli.Command) error { return tt.parse(cmd) }}
			err := app.Run(context.Background(), append([]string{"itb"}, tt.args...))
			if (err != nil) != tt.wantErr {
				t.Fatalf("Run(%v) error = %v, wantErr %v", tt.args, err, tt.wantErr)
			}
		})
	}
}
