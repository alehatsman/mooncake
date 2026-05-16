---
id: F043
title: cmd/fleet init prompts for bearer tokens via plain readline — token is echoed to the terminal and lands in scrollback / tmux capture / screen share
severity: bug
package: cmd
file: cmd/fleet_init.go
lines: 272-275, 305-320
status: done
verified: 2026-05-16 — confirmed real on master @ e78553ae. cmd/fleet_init.go:272 reads bearer token via promptDefault (line 303-320), which uses reader.ReadString at line 311 — no terminal echo suppression. Token characters land in tmux capture / scrollback / screen share. Fix: term.ReadPassword(int(os.Stdin.Fd())) on the token-prompt path
fixed: 2026-05-16 — added promptSecret(w, reader, prompt) in fleet_init.go using term.IsTerminal+term.ReadPassword for TTY; non-TTY falls back to reader with stderr warning. Same fix applied to fleet.go readToken stdin case (F031 adjacent). All fleet init tests pass.
---

## What

`promptOneCandidate` (fleet_init.go:272) reads a bearer token
from the operator via `promptDefault`:

```go
token, err := promptDefault(w, reader,
    fmt.Sprintf("? %s — paste bearer token (cat /etc/mooncake/agentd.token on %s)", cand.Name, cand.Name),
    "")
```

`promptDefault` (line 305-320) is a generic readline helper:

```go
func promptDefault(w io.Writer, reader *bufio.Reader, prompt, def string) (string, error) {
    if def != "" {
        fmt.Fprintf(w, "%s (default %s): ", prompt, def)
    } else {
        fmt.Fprintf(w, "%s: ", prompt)
    }
    line, err := reader.ReadString('\n')
    // ...
    return line, nil
}
```

`bufio.Reader.ReadString('\n')` against `os.Stdin` does **not**
disable terminal echo. The token characters appear on screen as
the operator types/pastes them.

## Why it's a bug

Compare with mooncake's existing sudo-password input
(`internal/security/password.go:32-38`):

```go
fmt.Fprint(os.Stderr, "BECOME password: ")
password, err := term.ReadPassword(fd)
fmt.Fprintln(os.Stderr) // New line after password input
```

`term.ReadPassword(fd)` puts the TTY in non-echo mode for the
duration of the read. Sudo password input gets the right
treatment; bearer token input does not — even though tokens
have **equivalent or worse** blast radius (they authenticate
the daemon's HTTP control plane, which can submit and execute
arbitrary plans).

Concrete exposure paths:

1. **Terminal scrollback** — every modern terminal (iTerm2,
   gnome-terminal, Alacritty, Windows Terminal) keeps thousands
   of lines of history. The token sits there until the
   scrollback rotates.
2. **tmux/screen capture** — multiplexer pane buffers persist
   across detach/reattach; an attacker who later attaches to
   the same session sees the token.
3. **Screen sharing** — a Zoom/Slack call during fleet
   onboarding broadcasts the token.
4. **Shoulder surfing** — coworker glances at the screen at
   the wrong moment.
5. **Backup software** that snapshots terminal state
   (uncommon, but `expect` / asciinema-style recorders do it
   on purpose).

This is exactly why every well-known interactive auth flow
(ssh password, sudo password, git credential helper) suppresses
echo.

## Adjacent concerns

### (a) The prompt says "paste"

The text `paste bearer token` (line 272) explicitly invites the
operator to paste a multi-character token. Pasting into a
non-echo prompt works fine (the characters go to stdin without
display). Today's echo-on prompt means a pasted token is
**displayed** before Enter even gets pressed; the operator can
read along.

### (b) The token then lands in peers.toml in cleartext

`fleet.Upsert` (line 288) persists the token to peers.toml at
mode 0600 (good). But the operator-side exposure during prompt
is before that.

### (c) `--accept-all` errors out (good)

Line 256-261: non-interactive mode refuses mDNS-only candidates
because there's no token source. So the only way to hit the
echoed prompt is the interactive path. That bounds the impact
(no CI script accidentally surfacing tokens) but doesn't fix
the interactive case.

## Suggested fix

Replace the token-specific call with a non-echo variant. Easiest
shape: a new `promptSecret` helper that uses `term.ReadPassword`
when stdin is a TTY, falls back to `ReadString` (with a clear
"WARNING: stdin is not a TTY; token will be echoed if your
client mirrors it" log) when stdin is piped.

```go
// In cmd/fleet_init.go:

import "golang.org/x/term"

// promptSecret reads a value without echoing it to the terminal.
// Returns the trimmed line. Caller is responsible for the
// trailing newline UX (print it after the read).
func promptSecret(w io.Writer, prompt string) (string, error) {
    fmt.Fprint(w, prompt+": ")
    fd := int(os.Stdin.Fd())
    if !term.IsTerminal(fd) {
        // Non-TTY: degrade to plain read but warn — caller can
        // decide whether to refuse instead.
        fmt.Fprintln(os.Stderr, "(stdin is not a TTY; token will be echoed if the client mirrors it)")
        reader := bufio.NewReader(os.Stdin)
        line, err := reader.ReadString('\n')
        if err != nil && !errors.Is(err, io.EOF) {
            return "", err
        }
        return strings.TrimRight(line, "\r\n"), nil
    }
    raw, err := term.ReadPassword(fd)
    fmt.Fprintln(w) // newline after the hidden input
    if err != nil {
        return "", err
    }
    return strings.TrimSpace(string(raw)), nil
}
```

Then in `promptOneCandidate`:

```go
// Before:
token, err := promptDefault(w, reader, fmt.Sprintf("? %s — paste bearer token (cat /etc/mooncake/agentd.token on %s)", cand.Name, cand.Name), "")

// After:
token, err := promptSecret(w, fmt.Sprintf("? %s — paste bearer token (cat /etc/mooncake/agentd.token on %s)", cand.Name, cand.Name))
```

The fix is ~20 lines + import.

## Adjacent — fleet pair has the same pattern?

Let me note: `cmd/fleet.go` has `fleetPairCommand` / `readToken`
(F031's territory). After F031's fix landed, the `stdin` path of
`readToken` uses `fmt.Fscanln`. That **also** doesn't suppress
echo. So **F031's stdin path has the same exposure as F043**.
F031 was filed as a `smell` for the `literal:` and `file:` modes;
the stdin path was implicit and may have the same bug. Worth a
follow-up audit when fixing F043.

## Verification

- Add `TestFleetInit_TokenPromptDoesNotEcho` — hard to test in
  a unit (TTY behavior is interactive), but a `script(1)` /
  `expect` integration test can verify no echo bytes flow
  back during the token prompt.
- Manual: run `mooncake fleet init` interactively; type a known
  token like `aaaaaaaaaa` slowly and confirm the screen shows
  only the prompt and a blank line, not the token characters.

## References

- `internal/security/password.go:32-38` — the correct pattern,
  already used for sudo passwords.
- F031 — same family. F031 hardened `--token-via literal:` and
  `--token-via file:`; the `stdin` path's echo behavior wasn't
  in its scope.
- F030 — adjacent: file-perm enforcement for password files.
  Tokens deserve the same care across all input surfaces.
