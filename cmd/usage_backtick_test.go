package main

import (
	"strings"
	"testing"

	"github.com/urfave/cli/v2"
)

// urfave/cli treats the first backtick-quoted token in a flag's Usage
// string as the NAME of that flag's value placeholder. Markdown emphasis
// written in prose therefore leaks into the rendered flag signature:
//
//	--dry-run mooncake plan, -n mooncake plan   (a bool flag!)
//	--allow-undefined {{ var }}                 (also a bool flag)
//	--peer key=value [ --peer key=value ]
//
// 27 Usage strings across apply/plan/fleet/agentd/task/tool shipped this
// way. Walk the whole command tree and keep it from coming back (#173).
func TestUsageStringsHaveNoBackticks(t *testing.T) {
	app := createApp()

	var walk func(prefix string, cmds []*cli.Command)
	walk = func(prefix string, cmds []*cli.Command) {
		for _, c := range cmds {
			path := strings.TrimSpace(prefix + " " + c.Name)
			if strings.Contains(c.Usage, "`") {
				t.Errorf("command %q: Usage contains a backtick — it renders literally in --help:\n  %s",
					path, c.Usage)
			}
			for _, f := range c.Flags {
				usage := flagUsage(f)
				if !strings.Contains(usage, "`") {
					continue
				}
				t.Errorf("flag %q on %q: Usage contains a backtick — urfave hoists it "+
					"into the flag's value placeholder:\n  %s",
					strings.Join(f.Names(), "/"), path, usage)
			}
			walk(path, c.Subcommands)
		}
	}
	walk("", app.Commands)
}

// flagUsage pulls the Usage field off the concrete urfave flag types the
// CLI actually uses. cli.Flag itself doesn't expose it.
func flagUsage(f cli.Flag) string {
	switch v := f.(type) {
	case *cli.StringFlag:
		return v.Usage
	case *cli.BoolFlag:
		return v.Usage
	case *cli.IntFlag:
		return v.Usage
	case *cli.Int64Flag:
		return v.Usage
	case *cli.DurationFlag:
		return v.Usage
	case *cli.StringSliceFlag:
		return v.Usage
	case *cli.PathFlag:
		return v.Usage
	default:
		return ""
	}
}
