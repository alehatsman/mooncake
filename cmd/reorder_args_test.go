package main

import (
	"reflect"
	"testing"

	"github.com/urfave/cli/v2"
)

// fixture returns a minimal app whose subcommand mirrors the shape of
// `mooncake fleet apply`: one string flag and one bool flag, both with
// short aliases. Used to exercise reorderArgs for the value-bearing and
// bool-flag branches.
func reorderTestApp() *cli.App {
	return &cli.App{
		Commands: []*cli.Command{
			{
				Name: "fleet",
				Subcommands: []*cli.Command{
					{
						Name: "apply",
						Flags: []cli.Flag{
							&cli.StringFlag{Name: "step-filter", Aliases: []string{"f"}},
							&cli.BoolFlag{Name: "no-color"},
						},
					},
				},
			},
		},
	}
}

func TestReorderArgs(t *testing.T) {
	app := reorderTestApp()

	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "flag-bearing arg already in front — no change",
			in:   []string{"mooncake", "fleet", "apply", "--step-filter", "tag=wsl", "main_pc"},
			want: []string{"mooncake", "fleet", "apply", "--step-filter", "tag=wsl", "main_pc"},
		},
		{
			name: "flag after positional — moved before",
			in:   []string{"mooncake", "fleet", "apply", "main_pc", "--step-filter", "tag=wsl"},
			want: []string{"mooncake", "fleet", "apply", "--step-filter", "tag=wsl", "main_pc"},
		},
		{
			name: "--flag=value form after positional",
			in:   []string{"mooncake", "fleet", "apply", "main_pc", "--step-filter=tag=wsl"},
			want: []string{"mooncake", "fleet", "apply", "--step-filter=tag=wsl", "main_pc"},
		},
		{
			name: "bool flag does not slurp following positional",
			in:   []string{"mooncake", "fleet", "apply", "--no-color", "main_pc"},
			want: []string{"mooncake", "fleet", "apply", "--no-color", "main_pc"},
		},
		{
			name: "bool flag after positional",
			in:   []string{"mooncake", "fleet", "apply", "main_pc", "--no-color"},
			want: []string{"mooncake", "fleet", "apply", "--no-color", "main_pc"},
		},
		{
			name: "-- terminator preserves everything after as positional",
			in:   []string{"mooncake", "fleet", "apply", "main_pc", "--", "--step-filter", "tag=wsl"},
			want: []string{"mooncake", "fleet", "apply", "main_pc", "--", "--step-filter", "tag=wsl"},
		},
		{
			name: "no subcommand match — args untouched",
			in:   []string{"mooncake", "nosuch", "main_pc", "--step-filter", "tag=wsl"},
			want: []string{"mooncake", "nosuch", "main_pc", "--step-filter", "tag=wsl"},
		},
		{
			name: "short alias still classified as value-bearing",
			in:   []string{"mooncake", "fleet", "apply", "main_pc", "-f", "tag=wsl"},
			want: []string{"mooncake", "fleet", "apply", "-f", "tag=wsl", "main_pc"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reorderArgs(tt.in, app)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("reorderArgs:\n got  %v\n want %v", got, tt.want)
			}
		})
	}
}
