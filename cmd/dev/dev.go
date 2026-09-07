// Package dev implements `mooncake dev` — the maintainer-only tooling
// tier.
//
// `docs`, `schema`, and `selfbuild` generate this project's own
// documentation, its JSON Schema / TypeScript exports, and its
// cross-compiled release binaries. None of them are part of provisioning
// a machine, but all three sat at the top level next to `apply`, in the
// command list every operator reads first.
//
// They move here and `dev` is Hidden: still fully functional when typed,
// still discoverable via `mooncake dev --help`, no longer competing for
// attention with the verbs an operator actually needs. See
// specs/cli-surface.md.
package dev

import (
	docscmd "github.com/alehatsman/mooncake/cmd/docs"
	schemacmd "github.com/alehatsman/mooncake/cmd/schema"
	selfbuildcmd "github.com/alehatsman/mooncake/cmd/selfbuild"
	"github.com/urfave/cli/v2"
)

// Command returns the hidden `dev` parent carrying the maintainer tools.
func Command() *cli.Command {
	return &cli.Command{
		Name:   "dev",
		Usage:  "Maintainer tooling (docs, schema, cross-build)",
		Hidden: true,
		Description: "Tooling for developing mooncake itself, not for using it. " +
			"Hidden from the top-level command list so `mooncake --help` shows " +
			"only the verbs that provision and manage machines.",
		Subcommands: []*cli.Command{
			docscmd.Command(),
			schemacmd.Command(),
			selfbuildcmd.Command(),
		},
	}
}
