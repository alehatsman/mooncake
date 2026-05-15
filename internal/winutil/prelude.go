package winutil

// UTF8OutputPrelude is the PowerShell preamble that forces UTF-8
// encoding on both the console output stream and the implicit pipe
// encoding `$OutputEncoding` uses when PowerShell hands bytes to
// native EXEs.
//
// Why: on default Windows installs PowerShell's `[Console]::Out`
// defaults to the OEM codepage (cp437/850 on en-US). Cmdlets that
// emit JSON (`Get-NetFirewallRule | ConvertTo-Json`,
// `Export-ScheduledTask`, etc.) round-trip their text through that
// encoding before stdout sees it, and any non-ASCII byte that the
// codepage can't represent becomes 0x1A (ASCII SUB). The Go side
// then fails to decode the corrupted JSON with `invalid character
// '\x1a' in string literal` — see issue #13.
//
// Both lines are necessary:
//
//   - `[Console]::OutputEncoding` governs Win32 console writes (the
//     stream the child process inherits as stdout).
//   - `$OutputEncoding` governs the encoding PowerShell uses when
//     piping to native EXEs (some cmdlets, including ConvertTo-Json's
//     internal pipeline, consult it).
//
// Prepend this to every PowerShell script before encoding it via
// -EncodedCommand. The prelude is idempotent and harmless on PS7,
// which defaults to UTF-8 already.
const UTF8OutputPrelude = "" +
	"[Console]::OutputEncoding = [System.Text.Encoding]::UTF8;\n" +
	"$OutputEncoding = [System.Text.Encoding]::UTF8;\n"

// WithUTF8Output prepends UTF8OutputPrelude to script. Convenience
// wrapper so callers don't import the bare constant.
func WithUTF8Output(script string) string {
	return UTF8OutputPrelude + script
}
