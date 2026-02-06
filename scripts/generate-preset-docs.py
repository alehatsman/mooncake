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


def generate_preset_card(preset, preset_name):
    """Generate a markdown card for a single preset."""

    # Build parameters info
    params_info = ""
    if preset.get('parameters'):
        param_count = len(preset['parameters'])
        params_info = f"<strong>Parameters:</strong> {param_count}"

    # Build card using pure HTML
    card = f"""
<div class="grid-card">

<h3>{preset['name']}</h3>

<p class="card-meta">
<span class="badge">v{preset['version']}</span> {params_info}
</p>

<p>{preset['description']}</p>

<pre><code class="language-bash">mooncake presets install {preset_name}</code></pre>

<p class="card-actions">
<a href="../guide/presets.md">Documentation</a>
<span>•</span>
<a href="../../presets/{preset_name}/">Source</a>
</p>

</div>
"""
    return card


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

    # Generate header
    content = """# Available Presets

Browse our collection of ready-to-use presets for common development tools and infrastructure.

<div class="preset-grid">
"""

    # Generate cards
    for preset in presets:
        card = generate_preset_card(preset['data'], preset['name'])
        content += card

    # Close grid
    content += "\n</div>\n"

    # Add footer
    content += f"""
---

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
