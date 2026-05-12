#!/usr/bin/env python3
"""
mooncake v1 → v2 YAML migrator (spec-21).

Rewrites Mooncake YAML files to the modern dot-namespaced action surface:

  file              → file.write
  template          → file.template
  copy              → file.copy
  download          → file.download
  unarchive         → file.unarchive
  file_replace      → text.replace
  file_insert       → text.insert
  file_delete_range → text.delete_range
  file_patch_apply  → text.patch
  package           → pkg
  service           → os.service
  command           → cmd
  repo_search       → repo.search
  repo_tree         → repo.tree
  repo_apply_patchset → repo.patch
  artifact_capture  → artifact.capture
  artifact_validate → artifact.validate
  print             → log
  preset            → use
  include           → import
  include_vars      → vars.load
  (shell, assert, wait, vars unchanged)

Framework keywords on each Step:

  with_items     → for_each
  with_filetree  → for_each_file
  register       → as
  creates        → unless_exists
  unless         → unless_command
  ignore_errors  → continue_on_error
  become + become_user → as_user (collapsed; become:true → as_user:root)
  retries + retry_delay → retry: {attempts, delay} (collapsed)

Usage:
    python3 migrate.py [--write] [--quiet] PATH [PATH...]

Without --write, runs in dry-run mode and prints which files would change.
With --write, rewrites files in place.

Step detection: a YAML mapping is treated as a Step iff its parent is
either (a) the document root as a sequence, or (b) a value of a key
named "steps". Sub-property mappings (e.g. inside `assert:` or
`file.write:` body) are left untouched.
"""
from __future__ import annotations

import argparse
import io
import sys
from pathlib import Path

from ruamel.yaml import YAML
from ruamel.yaml.comments import CommentedMap, CommentedSeq

# ---------- rename tables ---------------------------------------------------

ACTION_RENAMES = {
    "file": "file.write",
    "template": "file.template",
    "copy": "file.copy",
    "download": "file.download",
    "unarchive": "file.unarchive",
    "file_replace": "text.replace",
    "file_insert": "text.insert",
    "file_delete_range": "text.delete_range",
    "file_patch_apply": "text.patch",
    "package": "pkg",
    "service": "os.service",
    "command": "cmd",
    "repo_search": "repo.search",
    "repo_tree": "repo.tree",
    "repo_apply_patchset": "repo.patch",
    "artifact_capture": "artifact.capture",
    "artifact_validate": "artifact.validate",
    "print": "log",
    "preset": "use",
    "include": "import",
    "include_vars": "vars.load",
}

FRAMEWORK_RENAMES = {
    "with_items": "for_each",
    "with_filetree": "for_each_file",
    "register": "as",
    "creates": "unless_exists",
    "unless": "unless_command",
    "ignore_errors": "continue_on_error",
}

# Action keys whose value is a list of nested steps the migrator must
# recurse into (e.g. artifact_capture has a `steps:` field). After
# renaming the action key, we descend into the nested steps.
ACTIONS_WITH_NESTED_STEPS = {
    "artifact.capture",
    "artifact.validate",
}

# Keys we never rename when seen inside a Step (they're not action/framework
# keys but legitimate Step metadata or Step-level fields we keep as-is).
KEEP_AS_IS = {
    "name", "when", "shell", "assert", "wait", "vars",
    "tags", "timeout", "env", "cwd",
    "changed_when", "failed_when",
    "id", "action_type", "origin", "skipped", "loop_context",
}

# ---------- ruamel.yaml setup -----------------------------------------------

def _new_yaml() -> YAML:
    y = YAML()
    y.preserve_quotes = True
    y.width = 100000  # don't wrap
    return y


def _detect_indent_style(text: str) -> tuple[int, int]:
    """Detect (sequence, offset) indent for `text`.

    The Mooncake project mixes two conventions:

      a) Top-level YAML is a block sequence; `-` at column 0:
             - name: foo
               shell: bar
         → sequence=2, offset=0

      b) YAML root is a map with `steps:`; `-` indented under it:
             steps:
               - name: foo
                 shell: bar
         → sequence=4, offset=2

    Detection: look for `\\nsteps:\\n` followed by `\\n  - ` (b) vs `\\n- `
    starting the file (a). Default to (a).
    """
    if "\nsteps:\n" in text or text.startswith("steps:\n"):
        # Block-style under a map. Sample the lines after `steps:` to see
        # the actual offset.
        after_steps = text.split("steps:", 1)[1].lstrip("\n")
        for line in after_steps.splitlines():
            if line.strip().startswith("-"):
                indent = len(line) - len(line.lstrip(" "))
                if indent >= 2:
                    return 4, 2
                return 2, 0
        return 4, 2
    return 2, 0


# ---------- transformations -------------------------------------------------

def _rebuild_in_order(
    step: CommentedMap,
    pairs: list[tuple[str, object]],
) -> None:
    """Replace step's contents with `pairs` preserving its identity."""
    step.clear()
    for k, v in pairs:
        step[k] = v


def collapse_become(step: CommentedMap) -> None:
    """become + become_user → as_user, emitted at position of first old key."""
    has_become = "become" in step
    has_become_user = "become_user" in step
    if not (has_become or has_become_user):
        return

    become_val = step.get("become", False) if has_become else False
    become_user = step.get("become_user", "") if has_become_user else ""

    if become_user:
        new_val: str | None = become_user
    elif become_val is True:
        new_val = "root"
    else:
        new_val = None  # become: false → emit nothing

    new_pairs: list[tuple[str, object]] = []
    emitted = False
    for k, v in step.items():
        if k == "become" or k == "become_user":
            if not emitted and new_val is not None:
                new_pairs.append(("as_user", new_val))
                emitted = True
            # else: drop the old key entirely
            continue
        new_pairs.append((k, v))
    _rebuild_in_order(step, new_pairs)


def collapse_retry(step: CommentedMap) -> None:
    """retries + retry_delay → retry: {attempts, delay}, emitted at first old key."""
    has_retries = "retries" in step
    has_delay = "retry_delay" in step
    if not (has_retries or has_delay):
        return

    block = CommentedMap()
    if has_retries:
        block["attempts"] = step["retries"]
    if has_delay:
        block["delay"] = step["retry_delay"]

    new_pairs: list[tuple[str, object]] = []
    emitted = False
    for k, v in step.items():
        if k == "retries" or k == "retry_delay":
            if not emitted:
                new_pairs.append(("retry", block))
                emitted = True
            continue
        new_pairs.append((k, v))
    _rebuild_in_order(step, new_pairs)


def rename_step_keys(step: CommentedMap) -> bool:
    """Rename action + framework keys on a Step mapping. Returns True iff
    any change was made (used for stats). Mutates in place."""
    changed = False

    # Mapping iteration order is preserved via CommentedMap. To rename keys
    # without reordering, we walk the keys and replace each in place via a
    # small dance: ruamel.yaml CommentedMap supports `.rename`-like via
    # rebuilding the dict in original order.

    new = CommentedMap()
    for k, v in step.items():
        new_k = k
        if isinstance(k, str):
            if k in ACTION_RENAMES:
                new_k = ACTION_RENAMES[k]
                changed = True
            elif k in FRAMEWORK_RENAMES:
                new_k = FRAMEWORK_RENAMES[k]
                changed = True
        new[new_k] = v

    # Replace contents of `step` with `new` (preserves identity for callers
    # holding the reference, e.g. items in a sequence).
    step.clear()
    for k, v in new.items():
        step[k] = v

    collapse_become(step)
    collapse_retry(step)

    return changed


def walk_steps(seq: CommentedSeq) -> None:
    """Walk a sequence of Step mappings, renaming each, recursing into
    actions that carry nested step arrays."""
    for item in seq:
        if not isinstance(item, CommentedMap):
            continue  # malformed step; skip
        rename_step_keys(item)

        # Recurse into nested step arrays (artifact.capture etc.)
        for action_key in ACTIONS_WITH_NESTED_STEPS:
            if action_key in item and isinstance(item[action_key], CommentedMap):
                inner_steps = item[action_key].get("steps")
                if isinstance(inner_steps, CommentedSeq):
                    walk_steps(inner_steps)


def migrate_doc(doc) -> bool:
    """Migrate a single YAML document. Returns True iff anything changed.

    Recognized shapes:
      - Top-level SequenceNode → list of Steps (apply rename to each).
      - Top-level MappingNode with a `steps:` key → walk its value as Steps.
      - Top-level MappingNode with NO `steps:` → vars file; leave alone.
    """
    if isinstance(doc, CommentedSeq):
        walk_steps(doc)
        return True  # we can't easily detect "no-op" but spot-checks confirm

    if isinstance(doc, CommentedMap):
        if "steps" in doc and isinstance(doc["steps"], CommentedSeq):
            walk_steps(doc["steps"])
            return True

    return False


# ---------- file driver -----------------------------------------------------

def migrate_file(path: Path, write: bool, quiet: bool) -> tuple[bool, str | None]:
    """Returns (changed, error). If changed and write=True, writes back."""
    try:
        text = path.read_text()
    except Exception as e:  # noqa: BLE001
        return False, f"read error: {e}"

    if not text.strip():
        return False, None

    seq_indent, offset = _detect_indent_style(text)
    yaml = _new_yaml()
    yaml.indent(mapping=2, sequence=seq_indent, offset=offset)

    try:
        # Use round_trip mode (default) for comment preservation.
        doc = yaml.load(text)
    except Exception as e:  # noqa: BLE001
        return False, f"yaml parse error: {e}"

    if doc is None:
        return False, None

    try:
        changed = migrate_doc(doc)
    except Exception as e:  # noqa: BLE001
        return False, f"migration error: {e}"

    if not changed:
        return False, None

    buf = io.StringIO()
    yaml.dump(doc, buf)
    new_text = buf.getvalue()

    if new_text == text:
        return False, None

    if write:
        path.write_text(new_text)

    if not quiet:
        rel = path
        marker = "WRITE" if write else "WOULD"
        print(f"{marker} {rel}")

    return True, None


def main() -> int:
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawTextHelpFormatter)
    p.add_argument("paths", nargs="+", type=Path,
                   help="files or directories to migrate")
    p.add_argument("--write", action="store_true",
                   help="rewrite files in place (default: dry run)")
    p.add_argument("--quiet", action="store_true",
                   help="suppress per-file output")
    args = p.parse_args()

    # Gather files
    files: list[Path] = []
    for raw in args.paths:
        if raw.is_file():
            files.append(raw)
        elif raw.is_dir():
            files.extend(raw.rglob("*.yml"))
            files.extend(raw.rglob("*.yaml"))

    changed_count = 0
    error_count = 0
    for f in sorted(files):
        changed, err = migrate_file(f, write=args.write, quiet=args.quiet)
        if err:
            error_count += 1
            print(f"ERROR {f}: {err}", file=sys.stderr)
        if changed:
            changed_count += 1

    mode = "wrote" if args.write else "would change"
    print(f"\n{mode} {changed_count} file(s); {error_count} error(s); {len(files)} scanned",
          file=sys.stderr)

    return 0 if error_count == 0 else 1


if __name__ == "__main__":
    sys.exit(main())
