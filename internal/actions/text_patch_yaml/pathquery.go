// Path-parser thin shim — the actual parsing lives in
// internal/textpatchpath alongside the json sibling. The package-
// local aliases keep call sites in handler.go / tree.go (segment,
// parsePath, validatePath) working without churn.
//
//nolint:revive // package name follows action convention
package text_patch_yaml

import "github.com/alehatsman/mooncake/internal/textpatchpath"

type segment = textpatchpath.Segment

func parsePath(path string) ([]segment, error) { return textpatchpath.Parse(path) }

func validatePath(path string) error { return textpatchpath.Validate(path) }
