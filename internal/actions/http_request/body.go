package http_request

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/alehatsman/mooncake/internal/config"
)

// buildBody resolves the one-of body field on the HTTPRequest into the
// bytes that will be sent, plus a default Content-Type. The user-set
// Content-Type header (if any) takes precedence and is applied later;
// the contentType returned here is the *fallback*.
//
// Validate() has already guaranteed at most one body form is set.
func buildBody(r *config.HTTPRequest, renderedBody string, renderedForm map[string]string) ([]byte, string, error) {
	switch {
	case r.JSON != nil:
		raw, err := json.Marshal(r.JSON)
		if err != nil {
			return nil, "", fmt.Errorf("%s: marshal json body: %w", actionName, err)
		}
		return raw, "application/json", nil
	case len(r.Form) > 0:
		values := url.Values{}
		for k, v := range renderedForm {
			values.Set(k, v)
		}
		return []byte(values.Encode()), "application/x-www-form-urlencoded", nil
	case r.File != "":
		// File path is template-rendered upstream; bytes are sent
		// verbatim. No Content-Type default — caller sets it.
		raw, err := readFile(r.File)
		if err != nil {
			return nil, "", fmt.Errorf("%s: read body file %q: %w", actionName, r.File, err)
		}
		return raw, "", nil
	case renderedBody != "":
		return []byte(renderedBody), "", nil
	default:
		return nil, "", nil
	}
}

// decodeJSON wraps json.Unmarshal for response-body parsing. Lives
// here so the import stays local to body.go.
func decodeJSON(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
