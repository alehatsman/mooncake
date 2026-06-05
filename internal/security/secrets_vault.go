package security

// secrets_vault.go ships the `vault:` secret provider. Decrypts Age-encrypted
// files from a vault directory so secrets can be committed to the config repo
// (encrypted at rest) rather than placed as gitignored 0600 files.
//
// Wire path:
//
//	!secret vault:db/password → decrypt $MOONCAKE_VAULT_DIR/db/password.age
//	                             using identity from $MOONCAKE_VAULT_IDENTITY
//
// Identity file:   $MOONCAKE_VAULT_IDENTITY  (default: ~/.config/mooncake/vault-identity.txt)
// Vault directory: $MOONCAKE_VAULT_DIR       (default: ~/.config/mooncake/vault/)
//
// Decrypted values are cached per-VaultProvider instance for the lifetime of
// the apply (same secret referenced twice → one decrypt, same value).

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"filippo.io/age"
)

const (
	// VaultIdentityEnv is the env var pointing to the Age identity file.
	VaultIdentityEnv = "MOONCAKE_VAULT_IDENTITY"
	// VaultDirEnv is the env var pointing to the vault directory.
	VaultDirEnv = "MOONCAKE_VAULT_DIR"
)

// VaultProvider resolves `vault:<path>` refs by decrypting Age-encrypted files.
type VaultProvider struct {
	mu       sync.Mutex
	cache    map[string]string
	identFn  func() ([]age.Identity, error) // injectable for tests
	vaultDir func() (string, error)         // injectable for tests
}

// NewVaultProvider returns a VaultProvider wired to the real identity and
// vault directory (from env vars or defaults). Tests inject their own fns.
func NewVaultProvider() *VaultProvider {
	return &VaultProvider{
		cache:    make(map[string]string),
		identFn:  defaultIdentities,
		vaultDir: defaultVaultDir,
	}
}

// Resolve decrypts the Age file at <vaultDir>/<path>.age and returns its
// contents (trailing newline stripped, same convention as FileProvider).
func (p *VaultProvider) Resolve(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("vault provider: empty path")
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	if v, ok := p.cache[path]; ok {
		return v, nil
	}

	dir, err := p.vaultDir()
	if err != nil {
		return "", fmt.Errorf("vault provider: %w", err)
	}

	// Prevent path traversal: reject any path component that is ".." or that
	// resolves outside the vault directory.
	cleaned := filepath.Clean(path)
	if strings.HasPrefix(cleaned, "..") {
		return "", errors.New("vault provider: path traversal not allowed")
	}
	full := filepath.Join(dir, cleaned+".age")
	abs, err := filepath.Abs(full)
	if err != nil || !strings.HasPrefix(abs, dir) {
		return "", errors.New("vault provider: path traversal not allowed")
	}

	identities, err := p.identFn()
	if err != nil {
		return "", fmt.Errorf("vault provider: %w", err)
	}

	ct, err := os.ReadFile(full) // #nosec G304 -- vault path, path traversal guarded above
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("vault provider: secret not found")
		}
		return "", fmt.Errorf("vault provider: read failed")
	}

	pt, err := ageDecrypt(ct, identities)
	if err != nil {
		return "", fmt.Errorf("vault provider: decrypt failed")
	}

	s := strings.TrimRight(string(pt), "\r\n")
	if s == "" {
		return "", errors.New("vault provider: decrypted secret is empty")
	}
	p.cache[path] = s
	return s, nil
}

// ageDecrypt is a thin wrapper around age.Decrypt to keep Resolve readable.
func ageDecrypt(ciphertext []byte, ids []age.Identity) ([]byte, error) {
	r, err := age.Decrypt(bytes.NewReader(ciphertext), ids...)
	if err != nil {
		return nil, err
	}
	return io.ReadAll(r)
}

// AgeEncryptBytes encrypts plaintext for the given recipients and returns the
// Age ciphertext. Used by the vault CLI and tests.
func AgeEncryptBytes(plaintext []byte, recips ...age.Recipient) ([]byte, error) {
	var buf bytes.Buffer
	w, err := age.Encrypt(&buf, recips...)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(plaintext); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// defaultIdentities loads Age identities from the identity file pointed to by
// MOONCAKE_VAULT_IDENTITY or the default location.
func defaultIdentities() ([]age.Identity, error) {
	path := os.Getenv(VaultIdentityEnv)
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, errors.New("cannot resolve home directory for vault identity")
		}
		path = filepath.Join(home, ".config", "mooncake", "vault-identity.txt")
	}
	return loadIdentitiesFromFile(path)
}

// loadIdentitiesFromFile parses an Age identity file (one AGE-SECRET-KEY-… per line).
func loadIdentitiesFromFile(path string) ([]age.Identity, error) {
	f, err := os.Open(path) // #nosec G304 -- operator-controlled identity path
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("vault identity file not found at %s (run: mooncake vault init)", path)
		}
		return nil, fmt.Errorf("vault identity file open failed")
	}
	defer func() { _ = f.Close() }()
	ids, err := age.ParseIdentities(f)
	if err != nil {
		return nil, fmt.Errorf("vault identity file parse failed")
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("vault identity file contains no identities")
	}
	return ids, nil
}

// defaultVaultDir returns the vault directory path from env or default.
func defaultVaultDir() (string, error) {
	dir := os.Getenv(VaultDirEnv)
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", errors.New("cannot resolve home directory for vault dir")
		}
		dir = filepath.Join(home, ".config", "mooncake", "vault")
	}
	// Expand ~ if the user set the env var with a tilde.
	if strings.HasPrefix(dir, "~/") {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, dir[2:])
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("vault dir resolution failed")
	}
	return abs, nil
}

// defaultVaultProvider is the singleton wired into DefaultRegistry.
var defaultVaultProvider = NewVaultProvider()

func init() {
	DefaultRegistry.Register("vault", defaultVaultProvider)
}
