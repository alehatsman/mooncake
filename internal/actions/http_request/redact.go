package http_request

// defaultSensitiveHeaders returns the lower-cased header names whose
// values are always redacted in logs/diffs/events. The auth form adds
// to this set at render time (e.g. an `X-Api-Key:` from auth.header is
// appended). Returned as a *new* map each call so callers can mutate.
func defaultSensitiveHeaders() map[string]bool {
	out := make(map[string]bool, len(baseSensitiveHeaders))
	for _, h := range baseSensitiveHeaders {
		out[h] = true
	}
	return out
}

// baseSensitiveHeaders is the canonical "do not log this header value"
// list. Conservative by design: false positives just produce noise in
// audit logs, false negatives leak credentials.
var baseSensitiveHeaders = []string{
	"authorization",
	"proxy-authorization",
	"cookie",
	"set-cookie",
	"x-api-key",
	"x-auth-token",
	"x-token",
	"x-access-token",
	"x-csrf-token",
	"x-csrftoken",
	"x-vault-token",
	"x-amz-security-token",
}
