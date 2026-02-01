# Best Practices

## 1. Always Use Dry-Run

Preview changes before applying:
```bash
mooncake run --config config.yml --dry-run
```

## 2. Organize by Purpose

```
project/
├── main.yml
├── tasks/
│   ├── common.yml
│   ├── dev.yml
│   └── prod.yml
└── vars/
    ├── dev.yml
    └── prod.yml
```

## 3. Use Variables

Make configs reusable:
```yaml
- vars:
    app_name: myapp
    version: "1.0.0"
```

## 4. Tag Your Workflows

```yaml
- name: Dev setup
  shell: install-dev-tools
  tags: [dev]

- name: Prod deploy
  shell: deploy-prod
  tags: [prod]
```

## 5. Document Conditions

```yaml
# Ubuntu 20+ only (older versions incompatible)
- name: Install package
  shell: apt install package
  when: distribution == "ubuntu" and distribution_major >= "20"
```

## 6. Use System Facts

```yaml
- shell: "{{package_manager}} install neovim"
  when: os == "linux"
```

## 7. Test Incrementally

1. Start simple
2. Test with `--dry-run`
3. Add complexity gradually
4. Use `--log-level debug`

## 8. Handle Errors

```yaml
- shell: which docker
  register: docker_check

- shell: install-docker
  when: docker_check.rc != 0
```
