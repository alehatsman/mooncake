# Hyprland Preset

Installs the Hyprland Wayland compositor stack for Arch Linux.

## What's included

| Component | Package | Role |
|-----------|---------|------|
| Compositor | hyprland | Wayland tiling WM |
| Status bar | waybar | Top bar |
| Launcher | wofi | App launcher |
| Notifications | dunst | Notification daemon |
| Wallpaper | hyprpaper | Wallpaper daemon |
| Lock screen | hyprlock | Screen locker |
| Idle | hypridle | Idle / sleep manager |
| Terminal | alacritty | GPU-accelerated terminal |
| File manager | yazi | Terminal file manager |
| Screenshots | grim + slurp | Wayland screenshot tools |

## Usage

```yaml
- name: Install Hyprland desktop
  preset: hyprland
  tags: [gui, hyprland]
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `state` | string | `present` | `present` or `absent` |
| `terminal` | string | `alacritty` | Default terminal emulator |
