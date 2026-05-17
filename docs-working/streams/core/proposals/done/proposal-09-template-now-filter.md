# Request — Template engine: a working `now`/time facility

**Status**: Shipped 2026-05-16 (commit `07eee974`, merged `24201c00`) — `apply_started_at` per-run variable + pongo2 `strftime` filter registered. Use `{{ apply_started_at | strftime:"%Y%m%d_%H%M%S" }}` to interpolate a timestamp.
**Filed**: 2026-05-16 by aleh (discovered while migrating dotfiles backup steps from `shell` to `file.copy`)

---

## The user-facing ask, in one sentence

> Let me write `dest: ~/.dotfiles-backup/.zshrc.{{ now('%Y%m%d_%H%M%S') }}`
> and have it render to a real timestamp at apply time.

## What I observed

Pongo2 (the engine mooncake uses) ships a built-in `now` tag, but in
mooncake's renderer it's a no-op — the format string echoes back
verbatim:

```yaml
- log:
    msg: "ts={% now 'Ymd_His' %}"
# renders: ts=
# (the tag emits nothing; the format string is silently dropped)

- file.write:
    path: /tmp/result
    content: |
      ts={% now 'Ymd_His' %}
# file contains: ts=Ymd_His
# (the tag passes the format through unchanged)
```

Verified by running both forms on mooncake `dev` (build from
2026-05-15). Behavior is consistent with pongo2's `now` tag being
registered but not given a working format implementation.

## Why it matters

Any "backup before overwrite with a timestamped name" pattern needs
this — five steps in my dotfiles look like:

```yaml
# components/zsh/index.yml — today
- name: Backup existing .zshrc
  shell: |
    if [ -f ~/.zshrc ]; then
      mkdir -p ~/.dotfiles-backup
      cp ~/.zshrc ~/.dotfiles-backup/.zshrc.$(date +%Y%m%d_%H%M%S)
    fi
  tags: [zsh, backup]
```

Same shape for `.gitconfig`, `.tmux.conf`, `.ignore`, and nvim's
`init.lua`. Each is a perfect `file.copy` step **modulo the timestamp
in the dest path** — without `now`, the only pure-action equivalent
collapses to a single `.bak` file (overwriting each apply), losing the
rolling history.

## What I want

Pick one of these — I have no preference, as long as it works in any
string field that goes through the renderer:

### Option A — make pongo2's built-in `now` tag work

```yaml
dest: ~/.dotfiles-backup/.zshrc.{% now "Ymd_His" %}
```

Format codes use Django's strftime-style; well-documented but historic.

### Option B — register a `now` filter

```yaml
dest: ~/.dotfiles-backup/.zshrc.{{ "Ymd_His" | now }}
# or
dest: ~/.dotfiles-backup/.zshrc.{{ now | strftime:"Ymd_His" }}
```

Filter form fits the existing `expanduser` / `tojson` shape better.

### Option C — expose `apply_started_at` as a variable

```yaml
dest: ~/.dotfiles-backup/.zshrc.{{ apply_started_at | strftime:"Ymd_His" }}
```

Bonus property: every step in one apply gets the **same** timestamp,
which is what you usually want (so the .zshrc / .gitconfig / .nvim
backups from one run group together by suffix). The current shell form
uses `$(date)` per step and can produce mismatched suffixes if a step
crosses a second boundary.

I'd weakly prefer **Option C**: it's the most "this is a fact about
this apply run" framing, sidesteps the format-string-pass-through bug,
and gives the grouping property for free.

## Design notes

- Whichever shape ships, document it in the rendering reference (the
  current docs for `expanduser` / `tojson` are the right neighbours).
- Add a strict-mode test: an unknown tag/filter inside a string field
  should fail loudly, not silently pass through (the current behavior
  for `{% now %}` was silent — it would have been caught at write time
  if the renderer errored on unrecognized tags).
- `strftime` format choice: Go's `2006-01-02 15:04:05` ref-time form is
  awful in user-facing config; either Django-style codes or POSIX
  `%Y%m%d` are fine.

## Effort estimate

Small. Option C is the cleanest: hang a `time.Time` off the runtime
context, expose it as a `apply_started_at` variable (and maybe `now` as
a live-time alias), and register a `strftime` filter that wraps Go's
`time.Format` with a POSIX-code translation.

## Won't fix unless

- The engine choice (pongo2 vs alternative) is up for replacement —
  in which case fold this into that decision rather than carrying it on
  pongo2.
