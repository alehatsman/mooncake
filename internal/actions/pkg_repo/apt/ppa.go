package apt

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/actions/pkg_repo/shared"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/httputil"
)

// PPARE constrains the `ppa:` shorthand to launchpad's `<owner>/<name>`
// shape. Both segments must start with [a-z0-9] and may contain
// lowercase alphanumerics, dot, underscore, dash — the same charset
// launchpad accepts. Uppercase / slashes inside the segments are
// rejected so we can't be tricked into building paths like
// `evil/../escape`.
var PPARE = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*/[a-z0-9][a-z0-9._-]*$`)

// LaunchpadAPIBase is the prefix used to discover a PPA's signing-key
// fingerprint. Exposed as a package-level var so tests can point it at
// httptest.Server URLs.
var LaunchpadAPIBase = "https://api.launchpad.net/1.0"

// KeyserverBase is the prefix used to fetch the actual key body once
// the fingerprint is known. Also overridable for tests.
var KeyserverBase = "https://keyserver.ubuntu.com/pks/lookup"

// discoverPPAKey resolves a launchpad PPA shorthand to (fingerprint,
// keyserver URL). Defaults to a real HTTP fetch; tests override the
// hook to avoid network I/O.
var discoverPPAKey = discoverPPAKeyDefault

func discoverPPAKeyDefault(ctx context.Context, owner, ppa string) (fingerprint, keyURL string, err error) {
	apiURL := fmt.Sprintf("%s/~%s/+archive/ubuntu/%s", LaunchpadAPIBase, owner, ppa)
	body, err := httputil.Get(ctx, apiURL)
	if err != nil {
		return "", "", fmt.Errorf("launchpad lookup %s/%s: %w", owner, ppa, err)
	}
	// Launchpad's REST surfaces signing_key_fingerprint as a top-level
	// string. We don't need the rest of the document, so a permissive
	// decode is enough.
	var doc struct {
		SigningKeyFingerprint string `json:"signing_key_fingerprint"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return "", "", fmt.Errorf("launchpad lookup %s/%s: decode response: %w", owner, ppa, err)
	}
	fpr := strings.TrimSpace(doc.SigningKeyFingerprint)
	if fpr == "" {
		return "", "", fmt.Errorf("launchpad lookup %s/%s: no signing_key_fingerprint in response", owner, ppa)
	}
	return fpr, KeyserverURL(fpr), nil
}

// KeyserverURL builds the canonical pks/lookup URL for `fpr`.
// Exported so tests and operators can predict the URL the apt driver
// will fetch.
func KeyserverURL(fpr string) string {
	return fmt.Sprintf("%s?op=get&search=0x%s", KeyserverBase, fpr)
}

// parsePPAShortcut splits "owner/name" into its halves. Returns ("",
// "", false) when the input doesn't match the launchpad shape.
func parsePPAShortcut(s string) (owner, name string, ok bool) {
	if !PPARE.MatchString(s) {
		return "", "", false
	}
	slash := strings.IndexByte(s, '/')
	return s[:slash], s[slash+1:], true
}

// ppaURI builds the standard launchpad ppa URI from owner/name.
func ppaURI(owner, name string) string {
	return fmt.Sprintf("http://ppa.launchpad.net/%s/%s/ubuntu", owner, name)
}

// expandPPA fills in the rendered_ fields that the `ppa:` shorthand
// implies. The base render() has already template-expanded any
// user-supplied overrides; we only fill defaults for fields the
// operator left empty.
//
// Network: a launchpad REST call is issued when the operator didn't
// pre-pin a fingerprint AND state=present (state=absent removes the
// sources file and doesn't touch the key).
func expandPPA(ctx actions.Context, out *rendered_, a *config.PkgRepoApt, state string) error {
	owner, ppa, ok := parsePPAShortcut(strings.TrimSpace(a.PPA))
	if !ok {
		// Validate is the canonical gate; this is defensive in case a
		// caller wires a Step past Validate.
		return fmt.Errorf("pkg.repo.apt: ppa %q must match <owner>/<name>", a.PPA)
	}

	if out.uri == "" {
		out.uri = ppaURI(owner, ppa)
	}
	if len(out.suites) == 0 {
		codename := ubuntuCodename(ctx.Variables())
		if codename == "" {
			return fmt.Errorf("pkg.repo.apt: ppa shorthand needs a suite; host distribution_codename is unset — supply `suites:` explicitly")
		}
		out.suites = []string{codename}
	}
	if len(out.components) == 0 {
		out.components = []string{"main"}
	}

	if state != shared.StatePresent {
		return nil
	}

	// Operator may pre-pin the fingerprint (skips the launchpad call).
	// In that case we still need the keyserver URL to fetch the key.
	if out.gpgKeyFingerprint != "" && out.gpgKeyURL == "" {
		out.gpgKeyURL = KeyserverURL(out.gpgKeyFingerprint)
		return nil
	}
	if out.gpgKeyURL != "" {
		// Already set by the operator (e.g. self-hosted launchpad
		// mirror). Validate already enforced fingerprint pinning in
		// this case.
		return nil
	}

	fpr, keyURL, err := discoverPPAKey(ctx.Ctx(), owner, ppa)
	if err != nil {
		return fmt.Errorf("pkg.repo.apt: %w", err)
	}
	out.gpgKeyFingerprint = fpr
	out.gpgKeyURL = keyURL
	return nil
}

// ubuntuCodename pulls the host's apt-style suite name out of the
// rendered variable map. Prefers the explicit codename fact; falls
// back to nothing (caller errors).
func ubuntuCodename(vars map[string]interface{}) string {
	if v, ok := vars["distribution_codename"].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}
