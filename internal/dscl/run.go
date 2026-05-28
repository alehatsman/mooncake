package dscl

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/security"
)

// cmdTimeout bounds the wallclock for every dscl write. Same 10s
// ceiling os_user / os_group used before this helper landed (F051).
const cmdTimeout = 10 * time.Second

// Run executes a dscl write against the local directory node through
// the PrivilegedRunner. dscl writes to /Local/Default need root; the
// runner takes care of the sudo wrap when mooncake isn't already
// running as root. Empty stderr on success; non-zero exit surfaces
// with the captured output for diagnosis. F2: the parent ctx is the
// run-wide cancel; the 10s cmdTimeout chains on top.
func Run(parent context.Context, runner *security.Privileged, args ...string) error {
	fullArgs := append([]string{"."}, args...)
	ctx, cancel := context.WithTimeout(parent, cmdTimeout)
	defer cancel()
	out, err := runner.Run(ctx, "dscl", fullArgs...)
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return fmt.Errorf("dscl %s: %w: %s", strings.Join(args, " "), err, msg)
		}
		return fmt.Errorf("dscl %s: %w", strings.Join(args, " "), err)
	}
	return nil
}
