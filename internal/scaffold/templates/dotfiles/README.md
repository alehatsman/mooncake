# My mooncake project

Scaffolded by `mooncake init --template dotfiles`.

## Layout

- `mooncake.yml` — main playbook. Edit me.
- `mooncake.vars.yml` — variables consumed by the playbook.
- `.mooncake/` — local state, plan artifacts. Gitignored.

## Daily use

```bash
mooncake plan          # preview what would change
mooncake apply         # run it
mooncake apply --dry-run   # same as `mooncake plan`
mooncake last          # what did the most recent run do?
mooncake presets list  # browse 330+ ready-made workflows
mooncake doctor        # something off? run this
```

## Template placeholders

Files in this scaffold use mooncake template syntax (`{{ os }}`,
`{{ home }}`, `{{ package_manager }}`, …). The planner resolves them at
apply time against the host you're running on — so the same playbook
behaves differently on Linux vs macOS, Arch vs Debian, etc.

Run `mooncake facts` to see every variable available to your templates.
