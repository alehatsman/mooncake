# My mooncake project

Scaffolded by `mooncake init --template dotfiles`.

## Layout

- `mooncake.yml` — main playbook. Edit me.
- `mooncake.vars.yml` — variables consumed by the playbook. Loaded
  automatically; no flag needed.
- `.mooncake/` — local state, plan artifacts. Gitignored.

## Daily use

```bash
mooncake plan          # preview what would change
mooncake apply         # run it
mooncake history       # what did the most recent run do?
mooncake actions list  # browse the action vocabulary
mooncake doctor        # something off? run this
```

## Reusing other people's config

```bash
mooncake mod add github.com/owner/repo@v1.0.0
```

Then reference it from a step:

```yaml
- use: <alias>
  props:
    some_param: value
```

## Template placeholders

Files in this scaffold use mooncake template syntax (`{{ os }}`,
`{{ home }}`, `{{ package_manager }}`, …). The planner resolves them at
apply time against the host you're running on — so the same playbook
behaves differently on Linux vs macOS, Arch vs Debian, etc.

Run `mooncake state facts` to see every variable available to your templates.
