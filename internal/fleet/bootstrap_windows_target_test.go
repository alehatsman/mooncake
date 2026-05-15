package fleet

import (
	"strings"
	"testing"
)

func TestWinContext_PathRenderers(t *testing.T) {
	wc := winContext{
		LocalAppData: `C:\Users\aleh\AppData\Local`,
		UserName:     "aleh",
		ComputerName: "DESKTOP-R809R54",
		TempDir:      `C:\Users\aleh\AppData\Local\Temp`,
	}
	cases := map[string]string{
		wc.FullUserID():   `DESKTOP-R809R54\aleh`,
		wc.BinaryPath():   `C:\Users\aleh\AppData\Local\Mooncake\bin\mooncake.exe`,
		wc.TokenPath():    `C:\Users\aleh\AppData\Local\Mooncake\agentd.token`,
		wc.TaskXMLPath(): `C:\Users\aleh\AppData\Local\Mooncake\agentd-task.xml`,
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
}

func TestParseWinContext_HappyPath(t *testing.T) {
	out := "C:\\Users\\aleh\\AppData\\Local\r\n" +
		"aleh\r\n" +
		"DESKTOP-R809R54\r\n" +
		"C:\\Users\\aleh\\AppData\\Local\\Temp\r\n"
	wc, err := parseWinContext(out)
	if err != nil {
		t.Fatal(err)
	}
	if wc.LocalAppData != `C:\Users\aleh\AppData\Local` {
		t.Errorf("LocalAppData = %q", wc.LocalAppData)
	}
	if wc.UserName != "aleh" {
		t.Errorf("UserName = %q", wc.UserName)
	}
	if wc.ComputerName != "DESKTOP-R809R54" {
		t.Errorf("ComputerName = %q", wc.ComputerName)
	}
	if wc.TempDir == "" {
		t.Errorf("TempDir empty")
	}
}

func TestParseWinContext_LF(t *testing.T) {
	// LF-only newlines should also work (some PowerShell hosts).
	out := "A\nB\nC\nD\n"
	wc, err := parseWinContext(out)
	if err != nil {
		t.Fatal(err)
	}
	if wc.LocalAppData != "A" || wc.UserName != "B" || wc.ComputerName != "C" || wc.TempDir != "D" {
		t.Errorf("unexpected parse: %+v", wc)
	}
}

func TestParseWinContext_TooFewLines(t *testing.T) {
	_, err := parseWinContext("only one line")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseWinContext_RejectsEmptyValue(t *testing.T) {
	// Any of the four being empty is a hard error: bootstrap can't
	// continue without a real %LOCALAPPDATA% / etc.
	out := "C:\\foo\r\n\r\nDESKTOP\r\nC:\\Temp\r\n"
	if _, err := parseWinContext(out); err == nil {
		t.Fatal("expected error for empty username")
	}
}

func TestPsQuote(t *testing.T) {
	cases := map[string]string{
		"":             "''",
		"plain":        "'plain'",
		"with space":   "'with space'",
		`it's`:         `'it''s'`,
		`C:\path\foo`:  `'C:\path\foo'`,
		`'leading`:     `'''leading'`,
		`trailing'`:    `'trailing'''`,
	}
	for in, want := range cases {
		if got := psQuote(in); got != want {
			t.Errorf("psQuote(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPsWrap_ContainsPowerShellInvocation(t *testing.T) {
	got := psWrap("Get-Item " + psQuote(`C:\foo`))
	for _, want := range []string{
		`powershell -NoProfile -Command`,
		`Get-Item 'C:\foo'`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("psWrap missing %q: %s", want, got)
		}
	}
}
