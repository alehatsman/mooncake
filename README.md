# Mooncake [![build](https://github.com/alehatsman/mooncake/actions/workflows/build.yml/badge.svg?branch=master)](https://github.com/alehatsman/mooncake/actions/workflows/build.yml)

Space fighters provisioning tool, **Chookity!**

## Progress

- [x]  Cli commands and help via ([https://github.com/spf13/cobra](https://github.com/spf13/cobra))
- [x]  Include
- [x]  Dir
- [x]  Template via (https://github.com/flosch/pongo2)
- [x]  Shell
- [x]  Conditioning via (https://github.com/Knetic/govaluate)
- [x]  Facts (OS)
- [x]  Render configuration values (https://github.com/flosch/pongo2)
- [x]  Relative paths
- [ ]  apt
- [ ]  brew
- [ ]  git (via https://github.com/go-git/go-git or command-line)
- [ ]  pip
- [ ]  pipenv
- [ ]  cargo
- [ ]  sudo
- [ ]  1password
- [ ]  openssh keypair
- [ ]  make
- [ ]  http
- [ ]  with_items
- [ ]  with_filetree
- [ ]  Watch (https://github.com/fsnotify/fsnotify)

## Configuration

### Example

```yaml
- variables:
    config_dir: ~/.config
    nvim_config_dir: "{{config_dir}}/nvim"

- name: Make sure neovim config folder exists
  file:
    path: "{{nvim_config_dir}}"
    state: directory

- name: Render init.lua template
  template:
    src: ./init.lua
    dest: "{{nvim_config_dir}}/init.lua"
```

### Global variables

```yaml
os: linux | windows | darwin
```

### Conditioning

```yaml
- name: Include file if linux
  include: ./linux.yml
  when: os == "linux"
```

### Variables

Make sure to wrap the value into `"value"` for variable expansion to work.

```yaml
- variables:
    config_dir: ~/.config
    nvim_dir: "{{config_dir}}/nvim"

- template:
    src: ./init.lua.j2
    dest: "{{nvim_dir}}/init.lua"
```

### Include

```yaml
- name: Include provisioning file
  include: ./someprov.yml
```

### File

```yaml
- name: Make sure file or directory exists
  file:
    path: <Path>
    state: directory | file
```

### Template

```yaml
- name: Make sure template rendered into file
  template:
    src: ./template.j2
    dest: ~/.config/nvim/init.lua
    vars:
      port: 8080
```

### Shell

```yaml
- name: Make sure c ommands are executed
  shell:
    command: |
      echo "Hello World"
```
