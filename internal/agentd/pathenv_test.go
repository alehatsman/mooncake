package agentd

import (
	"os"
	"strings"
	"testing"
)

func TestAugmentPathEnv(t *testing.T) {
	sep := string(os.PathListSeparator)
	home := "/home/aleh"

	t.Run("prepends missing user bin dirs", func(t *testing.T) {
		got := AugmentPathEnv("/usr/bin"+sep+"/bin", home)
		want := strings.Join([]string{
			"/home/aleh/.local/bin", "/home/aleh/bin", "/home/aleh/go/bin", "/usr/bin", "/bin",
		}, sep)
		if got != want {
			t.Errorf("got %q want %q", got, want)
		}
	})

	t.Run("skips dirs already present", func(t *testing.T) {
		in := "/home/aleh/.local/bin" + sep + "/usr/bin"
		got := AugmentPathEnv(in, home)
		// .local/bin already present → only bin + go/bin prepended.
		want := strings.Join([]string{
			"/home/aleh/bin", "/home/aleh/go/bin", "/home/aleh/.local/bin", "/usr/bin",
		}, sep)
		if got != want {
			t.Errorf("got %q want %q", got, want)
		}
	})

	t.Run("empty PATH", func(t *testing.T) {
		got := AugmentPathEnv("", home)
		want := strings.Join([]string{
			"/home/aleh/.local/bin", "/home/aleh/bin", "/home/aleh/go/bin",
		}, sep)
		if got != want {
			t.Errorf("got %q want %q", got, want)
		}
	})

	t.Run("empty home is a no-op", func(t *testing.T) {
		if got := AugmentPathEnv("/usr/bin", ""); got != "/usr/bin" {
			t.Errorf("empty home should not change PATH, got %q", got)
		}
	})
}
