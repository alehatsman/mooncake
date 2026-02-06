#!/usr/bin/env python3
"""
Generate preset documentation with cards for MkDocs.

This script scans the presets directory and generates a markdown file
with cards for each preset, including name, description, version, and parameters.
"""

import yaml
import os
from pathlib import Path
import sys


def generate_preset_row(preset, preset_name):
    """Generate a markdown table row for a single preset."""

    # Escape pipe characters in description if any
    description = preset['description'].replace('|', '\\|')

    # Create source link
    source_link = f"[{preset_name}](https://github.com/alehatsman/mooncake/tree/master/presets/{preset_name})"

    # Create install command
    install_cmd = f"`mooncake presets install {preset_name}`"

    return f"| {source_link} | {description} | {install_cmd} |\n"


def scan_presets(presets_dir="presets"):
    """Scan presets directory and return list of preset info."""
    presets = []
    presets_path = Path(presets_dir)

    if not presets_path.exists():
        print(f"Warning: Presets directory '{presets_dir}' not found", file=sys.stderr)
        return presets

    # Scan for directory-based presets (name/preset.yml)
    for preset_dir in sorted(presets_path.iterdir()):
        if not preset_dir.is_dir():
            continue

        preset_file = preset_dir / "preset.yml"
        if not preset_file.exists():
            continue

        try:
            with open(preset_file) as f:
                preset_data = yaml.safe_load(f)

            presets.append({
                'name': preset_dir.name,
                'data': preset_data,
                'path': preset_file
            })
        except Exception as e:
            print(f"Warning: Failed to parse {preset_file}: {e}", file=sys.stderr)

    # Scan for flat presets (name.yml)
    for preset_file in sorted(presets_path.glob("*.yml")):
        try:
            with open(preset_file) as f:
                preset_data = yaml.safe_load(f)

            preset_name = preset_file.stem
            presets.append({
                'name': preset_name,
                'data': preset_data,
                'path': preset_file
            })
        except Exception as e:
            print(f"Warning: Failed to parse {preset_file}: {e}", file=sys.stderr)

    return presets


def generate_preset_docs(output_file="docs/presets/available.md"):
    """Generate the preset documentation page."""

    presets = scan_presets()

    if not presets:
        print("Warning: No presets found", file=sys.stderr)
        return

    # Sort by name
    presets.sort(key=lambda p: p['name'])

    # Generate header with usage instructions at top
    content = """# Available Presets

Browse our collection of ready-to-use presets for common development tools and infrastructure.

## Using Presets

Install a preset interactively:

```bash
mooncake presets -K
```

Or install a specific preset:

```bash
mooncake presets install -K <preset-name>
```

For more information, see the [Preset Guide](../guide/presets.md).

---

## All Presets

| Preset | Description | Install Command |
|--------|-------------|-----------------|
"""

    # Generate table rows
    for preset in presets:
        row = generate_preset_row(preset['data'], preset['name'])
        content += row

    # Add footer
    content += f"""
---

*Found {len(presets)} presets*
"""

    # Write output
    output_path = Path(output_file)
    output_path.parent.mkdir(parents=True, exist_ok=True)

    with open(output_path, 'w') as f:
        f.write(content)

    print(f"Generated {output_file} with {len(presets)} presets")


def main():
    """Main entry point."""
    # Check if we're in the right directory
    if not Path("presets").exists():
        print("Error: Must run from project root (presets/ directory not found)", file=sys.stderr)
        sys.exit(1)

    generate_preset_docs()
    print("✓ Preset documentation generated successfully")


if __name__ == "__main__":
    main()
