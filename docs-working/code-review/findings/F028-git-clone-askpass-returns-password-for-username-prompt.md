---
id: F028
title: git_clone askpass script returns password for the username prompt too — auth fails for HTTPS URLs without user@ embedded
severity: bug
package: internal/actions/git_clone
file: internal/actions/git_clone/credentials.go
lines: 78-85, 128-153
status: done
resolved_by: worktree-fix-f028
---

## What

`credentialEnv` (line 31) builds an askpass script and wires it
to git via `GIT_ASKPASS`. The script body (line 136):

```go
body := "#!/bin/sh\nprintf '%s' '" + escaped + "'\n"
```

i.e.

```sh
#!/bin/sh
printf '%s' 'THE-PASSWORD'
```

The script **does not branch on argv** — it always returns the
same string. But git invokes askpass twice for a bare HTTPS URL
(no `user:token@` embedded):

1. **"Username for 'https://host/repo': "** — git wants the
   username.
2. **"Password for 'username@https://host/repo': "** — git wants
   the password.

Today's askpass returns the password for **both** prompts. Git
then attempts to authenticate with `(password, password)` —
which the remote rejects (unless the password happens to be a
valid username, which it isn't for a real token / API key).

The doc-comment at line 77-84 even **describes** the intended
behavior:

```go
// Configure the username via the URL credential helper. When the
// HTTPS URL is bare (no embedded user), git uses the `Username`
// env var, which we surface via the askpass script's first
// invocation. The askpass receives the prompt text on argv[1]
// and may dispatch on it.
```

…but the implementation never reads `GIT_USERNAME` and never
dispatches on `$1`. The comment is **aspirational, not
descriptive**.

## Why it's a bug

Reproducible:

```yaml
- vars:
    git_token: ghp_xxxxxxxxxxxx
- git.clone:
    repo: https://github.com/owner/private.git   # ← bare URL, no user@
    dest: /tmp/private
    credentials:
      username: oauth2
      password: "{{ git_token }}"
```

Today: git fails with `fatal: Authentication failed for
'https://github.com/owner/private.git/'`. The user has the
right credentials but the askpass tries `password = "oauth2"` (no
— it tries `username = "ghp_xxx", password = "ghp_xxx"`, since
both prompts return the password).

Workaround: include the user in the URL
(`https://oauth2@github.com/...`). That's an undocumented
gotcha; users following the schema-suggested `credentials:` block
will hit this.

## Why the comment exists pointing the right direction

Git's `GIT_ASKPASS` protocol is documented in `gitcredentials(7)`:

> Git treats the program's standard output as the credential
> value, while its first argument is the prompt that git would
> have shown the user.

So the script SHOULD examine `$1` and return the username when
the prompt starts with "Username", the password otherwise. The
comment author knew this; the implementation didn't get there.

## Suggested fix

Replace the askpass body with a branching version. Two viable
shapes:

**Option A — dispatch on the prompt:**

```go
body := fmt.Sprintf(`#!/bin/sh
case "$1" in
  Username*) printf '%%s' %s ;;
  *)         printf '%%s' %s ;;
esac
`, shellQuote(username), shellQuote(password))
```

`shellQuote` produces a single-quoted-with-`'\''`-escaping string
(same shape as `shellEscape` at line 189 — could be reused).
Handles both bare URLs and embedded-user URLs.

When `username` is empty (HTTPS auth-with-token-as-username, e.g.
GitHub PAT where the username is ignored): the case statement
returns `''` for "Username", which then prompts git to fall back
to its credential helper. For GitHub PATs the convention is
`username = oauth2` or any non-empty string, so we should require
username in this code path:

```go
if password != "" && username == "" {
    return nil, func() {}, errors.New("credentials.password requires credentials.username to be set (use any string for token-based auth)")
}
```

**Option B — only set the askpass when password is provided,
require user@ in the URL:**

Document that HTTPS auth requires `https://user@host/...`. Simpler
script, simpler debugging. But less user-friendly — schema
suggests a `username:` field that doesn't actually work bare.

**Option A is the better fix** — matches the comment's stated
behavior + lets the `credentials:` block be self-contained.

## Adjacent observations

- `GIT_TERMINAL_PROMPT=0` (line 75) prevents interactive prompts,
  which is correct. With the bare-URL bug, git fails fast with
  "could not read Username" instead of hanging — that's the only
  saving grace today.
- `GIT_USERNAME` (line 83) is set but **read by nothing**. Either
  remove it or make the askpass actually consume it via
  `$GIT_USERNAME` substitution in Option A. The current code
  exports a never-used env var.
- `shellEscape` at line 189 is the right helper to factor out for
  Option A; it's currently only used for ssh-key paths. Rename
  to `shellSingleQuote` to make the policy obvious.

## Verification

- New test `TestCredentialEnv_AskpassReturnsUsernameForUsernamePrompt`:
  unit-tests the askpass script directly by `exec`-ing it with
  argv `["Username for 'https://host':"]` and asserting stdout
  equals the configured username. Pre-fix returns the password.
- New test `TestCredentialEnv_AskpassReturnsPasswordForPasswordPrompt`:
  same but with `["Password for ...":]` argv.
- Manual: `mooncake apply` a config that clones a private HTTPS
  repo with a bare URL + credentials block. Today: 401. After
  fix: works.

## References

- `gitcredentials(7)` — askpass protocol.
- `internal/actions/git_clone/credentials.go:77-84` — the
  comment that describes the intended behavior the impl doesn't
  follow.
