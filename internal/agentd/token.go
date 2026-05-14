package agentd

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// tokenBytes is the entropy length of generated bearer tokens. 32 random
// bytes → 43 base64url chars; comfortably resistant to guessing while small
// enough to paste into a TOML file.
const tokenBytes = 32

// LoadOrCreateToken returns the bearer token at path, generating one if the
// file does not exist or is empty.
//
// The token is `tokenBytes` of `crypto/rand` output, base64url-encoded
// without padding. The file is written atomically (temp file + rename) with
// mode 0600 and a trailing newline. The parent directory is created with
// mode 0700.
//
// Idempotent: repeat calls against the same existing non-empty file return
// the same token.
func LoadOrCreateToken(path string) (string, error) {
	if path == "" {
		return "", errors.New("token path is empty")
	}

	if data, err := os.ReadFile(path); err == nil {
		tok := strings.TrimSpace(string(data))
		if tok != "" {
			return tok, nil
		}
		// Empty file — fall through to regenerate.
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("read token file %s: %w", path, err)
	}

	raw := make([]byte, tokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	tok := base64.RawURLEncoding.EncodeToString(raw)

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create token dir %s: %w", dir, err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(tok+"\n"), 0o600); err != nil {
		return "", fmt.Errorf("write token: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("rename token: %w", err)
	}
	return tok, nil
}
