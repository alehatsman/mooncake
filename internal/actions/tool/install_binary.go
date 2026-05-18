package tool

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// installBareBinary copies a downloaded bare binary into destDir and
// marks it executable. The destination filename is, in order of
// preference:
//
//  1. spec.Bin (when set — already the lockfile-recorded bin name)
//  2. plan.BinRel (when set — basename of the configured bin path)
//  3. spec.Name (the canonical fallback)
//
// destDir must exist. Permission bits on the final file are 0o755 so
// the tool is executable for the installing user and readable by
// everyone — matches the conventional mode of extracted release tarballs.
func installBareBinary(srcPath, destDir string, spec Spec, plan Plan) error {
	binName := bareBinaryName(spec, plan)
	dest := filepath.Join(destDir, binName)

	// #nosec G304 -- srcPath is a mooncake-managed temp file
	in, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open downloaded binary: %w", err)
	}
	defer func() { _ = in.Close() }()

	// #nosec G304 -- dest is inside the mooncake-managed install dir
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return fmt.Errorf("create %s: %w", dest, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return fmt.Errorf("copy to %s: %w", dest, err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("close %s: %w", dest, err)
	}
	if err := os.Chmod(dest, 0o755); err != nil {
		return fmt.Errorf("chmod +x %s: %w", dest, err)
	}
	return nil
}

// bareBinaryName picks the final filename inside the install dir.
func bareBinaryName(spec Spec, plan Plan) string {
	if spec.Bin != "" {
		return filepath.Base(spec.Bin)
	}
	if plan.BinRel != "" {
		return filepath.Base(plan.BinRel)
	}
	return spec.Name
}
