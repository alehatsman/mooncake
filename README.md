# Mooncake [![build](https://github.com/alehatsman/mooncake/actions/workflows/build_test.yml/badge.svg?branch=master)](https://github.com/alehatsman/mooncake/actions/workflows/build_test.yml)

Space fighters provisioning tool, **Chookity!**

## In progress

- [ ] Nice output, info, log, error

## Progress

- [x] Cli commands and help via ([https://github.com/spf13/cobra](https://github.com/spf13/cobra))
- [x] Include another file
- [x] Dir - create directive
- [x] Template - create a file by rendering a template via (https://github.com/flosch/pongo2)
- [x] Shell - execute a shell command, passing the script with templating
- [x] When - conditions, for example step only for specific os (https://github.com/Knetic/govaluate)
- [x] Facts - global variables with facts about system (OS)
- [x] Templating for config values - variables are available from facts and defined by user. (https://github.com/flosch/pongo2)
- [x] Supports relatives paths - node like file resolution.
- [x] sudo argument or prompt by default 

- [ ] OpenSsh - generate keys
- [ ] Http - make http calls, download files etc.
- [ ] with_items - pass list and it will execute the command with each item
- [ ] with_filetree - point to a folder with files or tree of files, and it will execute the task with each file/dir.
- [ ] Apt - yml directive
- [ ] Brew - yml directive
- [ ] Git - clone repos (via https://github.com/go-git/go-git or command-line)
- [ ] pip
- [ ] pipenv
- [ ] cargo
- [ ] 1password
- [ ] make
- [ ] watch (https://github.com/fsnotify/fsnotify)

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
