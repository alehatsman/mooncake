# Proposal: High-Level `os.disk` Action for Partitioning and Filesystems

**Status**: Draft — exploration
**Date**: 2026-05-13
**Related**: `os.service` action (system-level provisioning), facts system (device discovery), spec-21 (modern action surface)

## Summary

Introduce a single declarative `os.disk` action that describes a whole-disk
layout — partition table, partitions, filesystems, and mounts — and reconciles
it against the live state of a block device. The action targets the case
where Mooncake is doing bare-metal or cloud-image provisioning and needs to
shape storage before anything else can be installed onto it.

The `os.*` namespace is chosen to parallel `os.service`: both describe
OS-level resources whose state Mooncake reconciles.

## Motivation

Partitioning is currently outside Mooncake's vocabulary. Users who need it
today drop to `shell:` with `parted` / `sgdisk` / `mkfs.*` / `mount`
invocations. That works, but:

- It is not idempotent without hand-rolled `unless:` guards.
- It is not visible to plan mode — the user cannot preview what will happen
  to their disks before applying.
- There is no built-in safety against targeting the wrong device. `/dev/sdb`
  on one boot may be the boot disk on the next.
- A complete layout (table + N partitions + N filesystems + N mounts)
  expands to a dozen or more `shell:` steps with order coupling between
  them.

A first-class action collapses all of that into one declaration with
idempotent reconciliation and plan-mode preview.

## Non-Goals (v1)

- **LUKS encryption.** Deferred — solvable as a follow-up `os.disk.crypt`
  action or as a `crypto:` block inside this one.
- **LVM volume groups / logical volumes.** Deferred — same shape as LUKS.
- **RAID / mdadm.** Deferred.
- **In-place partition resizing.** v1 refuses to mutate existing partitions;
  it can create on an empty disk or verify a matching layout. Resize is a
  separate, much harder problem.
- **macOS / Windows.** Linux only in v1. Other platforms get a clear
  "unsupported" error, matching the project's existing platform pattern.

## Design

### YAML Shape

```yaml
- name: lay out data disk
  as_user: root
  os.disk:
    device: /dev/disk/by-id/ata-WDC_WD5000AAKS-00V1A0_WD-WCAS12345678
    table: gpt

    # Safety guards — at least one expected_* is required.
    expected_size: 500GiB
    expected_serial: "WD-WCAS12345678"
    expected_model: "WDC WD5000AAKS-00V1A0"

    # Default false. Set true to allow destruction of existing data.
    wipe: false

    partitions:
      - name: efi
        size: 512MiB
        type: esp           # alias for the EFI System Partition GUID
        filesystem: vfat
        label: EFI
        mount:
          path: /boot/efi
          options: [umask=0077]
      - name: swap
        size: 8GiB
        filesystem: swap
      - name: root
        size: 100%          # consume remainder
        filesystem: ext4
        label: ROOT
        mount:
          path: /
          options: [defaults, noatime]
```

### Action Surface (`config.OsDisk`)

| Field             | Type          | Notes |
|-------------------|---------------|-------|
| `device`          | string        | Required. Prefer `/dev/disk/by-id/...` over `/dev/sdX`. |
| `table`           | enum          | `gpt` (default), `mbr`. |
| `expected_size`   | size string   | At least one `expected_*` required. |
| `expected_serial` | string        | |
| `expected_model`  | string        | |
| `wipe`            | bool          | Default false. Required to destroy existing data. |
| `partitions`      | list          | See below. |

Each partition:

| Field         | Type        | Notes |
|---------------|-------------|-------|
| `name`        | string      | Required, used as the GPT partition name. |
| `size`        | size string | `512MiB`, `8GiB`, `100%`. Exactly one `100%` allowed. |
| `type`        | string      | GPT type alias (`esp`, `linux`, `swap`) or raw GUID. |
| `filesystem`  | enum        | `ext4`, `xfs`, `btrfs`, `vfat`, `swap`, or `none`. |
| `label`       | string      | Filesystem label. |
| `mount`       | object      | Optional. `path`, `options`, `fstab` (default true). |

### Reconciliation Logic

Per-invocation flow:

1. **Resolve and verify device.** Read symlink, `lsblk -J`, `udevadm info`.
   Verify all provided `expected_*` fields. Mismatch → fail before touching
   anything. This is the single most important safety property.
2. **Read current layout.** `sfdisk -d` for the table, `blkid` for
   filesystems, `findmnt` for mounts.
3. **Compare to desired layout.**
   - **No table / blank disk** → create fresh. Allowed without `wipe`.
   - **Layout matches** (same table type, same partition count, sizes within
     a small tolerance, same filesystems and labels) → no-op,
     `changed=false`.
   - **Layout differs** → fail unless `wipe: true`. With `wipe: true`,
     destroy and recreate.
4. **Mount and fstab.** Write `/etc/fstab` entries (idempotent — keyed on
     filesystem UUID, not device path) and mount.

Plan mode performs steps 1–3 read-only and prints the planned table:

```
[PLAN] os.disk /dev/disk/by-id/ata-WDC_…WCAS12345678
       size=500GiB serial=WD-WCAS12345678 → matches
       current: blank
       planned: gpt
                p1 efi  512MiB  vfat  EFI    → /boot/efi
                p2 swap   8GiB  swap
                p3 root  rest   ext4  ROOT   → /
```

### Tooling

- **Linux**: `sfdisk` for the partition table (scriptable, deterministic),
  `mkfs.ext4` / `mkfs.xfs` / `mkfs.btrfs` / `mkfs.vfat` / `mkswap` for
  filesystems, `mount` + direct `/etc/fstab` edits for mounts.
- `parted` is intentionally avoided — `sfdisk -d` round-trips cleanly and
  is the lower-friction tool for declarative use.

### Safety Properties

1. **Required device fingerprinting.** No `os.disk` action runs without at
   least one `expected_*` field. Catches `/dev/sdb`-renumbered-overnight.
2. **No destruction without `wipe: true`.** Default is reconcile-or-fail,
   never reconcile-by-destroying.
3. **Refuse mounted devices** unless the existing mount is part of the
   desired layout.
4. **Plan mode never writes.** Same code path for plan and apply, gated by
   `ctx.Mode()`.

## Open Questions

1. **Does fstab management belong inside `os.disk`, or as a separate
   `os.mount` action?** Argument for inside: a disk layout without mounts
   is half a feature and forces step ordering on the user. Argument for
   outside: smaller action surface, composes with non-disk mounts (NFS,
   tmpfs). Lean: inside, with a separate `os.mount` action later for
   non-disk cases.
2. **Size tolerance for "matches".** A 100% partition will not byte-match
   across disks. Need a tolerance (e.g. ±1 MiB) and a clear rule for the
   tail partition.
3. **What does `state: absent` mean?** Wipe the partition table? Refuse?
   Probably refuse in v1 and add an explicit `os.disk.wipe` action later if
   demand exists.
4. **Re-running after a successful apply** must be a clean no-op. Need to
   verify that `sfdisk -d` output is stable across kernel versions.

## Alternatives Considered

### Composable primitives (`os.partition_table`, `os.partition`, `os.filesystem`, `os.mount`)

Four actions instead of one. Pros: each is small and orthogonal. Cons:
users must coordinate ordering, idempotency checks live in the user's
config rather than the action, and the natural unit of intent — "I want
this disk to look like X" — is split across four steps. Rejected for v1;
nothing prevents extracting primitives later if needed.

### Wrapping an external tool (e.g. Disko, sgdisk script generation)

Disko is NixOS-specific. Generating an sgdisk script and shelling out hides
the logic from plan mode and makes idempotency harder. Rejected.

## v1 Scope Summary

- Linux only.
- GPT only (MBR can be a follow-up).
- Filesystems: ext4, xfs, btrfs, vfat, swap.
- Mandatory `expected_size` or `expected_serial` or `expected_model`.
- `wipe: false` default; non-destructive reconcile or fail.
- Mount + fstab managed inside the action.
- LUKS / LVM / RAID / resize / macOS / Windows out of scope.

## Implementation Sketch

- `internal/config/config.go` — add `OsDisk` struct and
  `Step.OsDisk *OsDisk` field with tag `yaml:"os.disk"`. Follows the
  spec-21 dot-namespaced convention.
- `internal/actions/os_disk/handler.go` — implement `Handler` interface
  with `Metadata().Name = "os.disk"`. Package name mirrors the dotted
  action name (compare `internal/actions/service/` for `os.service`).
- `internal/actions/os_disk/inspect.go` — read current state via
  `sfdisk -d`, `blkid`, `findmnt`.
- `internal/actions/os_disk/plan.go` — diff desired vs. current, produce
  a reconciliation plan struct shared by apply and plan modes (spec-16
  unified Run path).
- `internal/actions/os_disk/apply.go` — execute the reconciliation plan.
- `internal/config/schema.json` — regenerated via `mooncake schema
  generate` once `OsDisk` has the right struct tags.
- `examples/actions/os-disk.yml` — end-to-end example.
- Integration tests against a loopback file-backed device (`losetup`) so
  CI does not need real disks.
