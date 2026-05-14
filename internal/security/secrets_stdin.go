package security

// secrets_stdin.go ships the `stdin:` secret provider. Prompts the
// operator interactively on the controlling TTY, reads with echo
// disabled (term.ReadPassword), and caches the result per-process so
// multiple references to the same `stdin:KEY` only prompt once.
//
// Refuses to run when stdin is not a TTY (CI / pipe / nohup) rather
// than silently hanging on the read — the user would otherwise have no
// indication the apply was stuck waiting on a read that will never
// arrive.
//
// Example:
//
//	content: !secret stdin:postgres-root-password
//
// First reference prompts `Enter secret for stdin:postgres-root-password:`;
// subsequent references in the same apply reuse the cached value.

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"golang.org/x/term"
)

// StdinProvider implements the stdin: provider. The key after the
// colon is a display name — anything reasonable. Two refs with the
// same key share one prompt (per-process cache).
type StdinProvider struct {
	mu    sync.Mutex
	cache map[string]string

	// in / out / errOut / fd are injectable for tests. nil → real
	// terminal. promptFn likewise — production uses term.ReadPassword
	// on fd; tests inject a stub.
	in       io.Reader
	out      io.Writer
	errOut   io.Writer
	fd       int
	promptFn func(fd int) ([]byte, error)
}

// NewStdinProvider returns a provider hooked to the real controlling
// TTY. Wrapped in init() so the DefaultRegistry has it available out
// of the box; tests instantiate their own via the exported fields.
func NewStdinProvider() *StdinProvider {
	return &StdinProvider{
		cache: make(map[string]string),
	}
}

// Resolve prompts the operator (or returns the cached value) for `key`.
// Returns an error rather than hanging if stdin isn't a TTY.
func (p *StdinProvider) Resolve(key string) (string, error) {
	if strings.TrimSpace(key) == "" {
		return "", errors.New("stdin provider: empty key")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cache == nil {
		p.cache = make(map[string]string)
	}
	if v, ok := p.cache[key]; ok {
		return v, nil
	}

	out := p.out
	if out == nil {
		out = os.Stderr // never stdout — apply output is parseable
	}
	fd := p.fd
	if fd == 0 {
		fd = int(os.Stdin.Fd())
	}
	read := p.promptFn
	if read == nil {
		read = term.ReadPassword
	}
	// Refuse non-TTY rather than hang. The test path sets promptFn
	// directly, so this check only fires in real-world runs.
	if p.promptFn == nil && !term.IsTerminal(fd) {
		return "", errors.New(
			"stdin provider: stdin is not a TTY — cannot prompt for secret. " +
				"Use file: or env: providers in non-interactive runs.")
	}

	_, _ = fmt.Fprintf(out, "Enter secret for stdin:%s: ", key)
	raw, err := read(fd)
	// Print a newline after the (echo-suppressed) input so subsequent
	// output isn't on the same line as the prompt.
	_, _ = fmt.Fprintln(out)
	if err != nil {
		return "", errors.New("stdin provider: read failed")
	}
	val := strings.TrimRight(string(raw), "\r\n")
	if val == "" {
		return "", errors.New("stdin provider: empty input")
	}
	p.cache[key] = val
	return val, nil
}

// defaultStdinProvider is the singleton wired into DefaultRegistry.
// Exposed so test code can pre-seed its cache for hermetic tests.
var defaultStdinProvider = NewStdinProvider()

func init() {
	DefaultRegistry.Register("stdin", defaultStdinProvider)
}
