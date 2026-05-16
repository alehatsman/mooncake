# Bug — `git.clone` silently reports ✓ when `update:false` and `ref:` differs from the cloned ref

**Tracking:** [#26](https://github.com/alehatsman/mooncake/issues/26)
**Surfaced:** 2026-05-15 during tick-5 of the autonomous test loop —
file.* / git.* idempotency investigation.

## Repro

Two consecutive applies against the same dest but different requested
refs:

```yaml
# gc1.yml — clone at ref=master
- git.clone:
    repo: https://github.com/octocat/Hello-World.git
    dest: /tmp/gctest
    ref: master

# gc2.yml — request ref=test, leave update:false
- git.clone:
    repo: https://github.com/octocat/Hello-World.git
    dest: /tmp/gctest
    ref: test
```

```
$ mooncake apply -c gc1.yml
~ clone-ref-master
$ git -C /tmp/gctest rev-parse --abbrev-ref HEAD
master

$ mooncake apply -c gc2.yml
✓ clone-ref-test       ← green check ✓
RECAP  ok=1  changed=0  skipped=0  failed=0
$ git -C /tmp/gctest rev-parse --abbrev-ref HEAD
master                 ← actual state: NOT what the plan asked for
```

The plan said "I want this repo at ref=test". Mooncake reports ✓
(no change needed, all good). But the repo is at ref=master, not
ref=test. The operator reading the recap has no signal that the
declared state isn't reality.

## Why this looks like a bug

`git.clone`'s struct comment explicitly describes the design:

```go
// Idempotent: if dest is already a git repo at the requested ref, no change.
Update bool `yaml:"update"` // If dest exists: fetch + checkout ref (default false → noop)
```

So "do nothing if dest exists and update:false" *is* the documented
default — but the **reporting is wrong**:

- The action's contract is "idempotent if at the requested ref".
- The action checks "is the dest a git repo at all" and then
  returns "no change" without verifying the ref matches.
- The user-facing rendering says ✓ (success/no-change), which
  operators read as "state matches the declaration."

The right outcome is one of these three:

### Option A — Warn loudly, return skipped

```
~ clone-ref-test  [skipped: dest is at 'master', plan wants 'test', and update:false]
```

Skipped runs are visually distinct from ok / changed; the operator
knows the declaration isn't satisfied.

### Option B — Default `update:` to `true`

If the operator declares a ref, the natural intent is "make the
repo be at that ref." The current default (don't touch) is the
opposite. Changing the default is a behavior break for plans that
relied on the no-update default, but matches Ansible's `git:`
module which defaults to update.

### Option C — Hard fail at plan time

If `update:false` and the existing dest is at a different ref,
fail at plan-time with "won't reconcile dest=… ref=master to
ref=test without update:true. Set update:true to apply or change
the declared ref to match the current state."

Recommend **Option A** — the smallest behavior change that
restores honest reporting. Operators who *want* the silent-noop
behavior have an explicit signal (skipped vs ok).

## This is one of a family

In the silent-success-bugs findings doc
(`docs-working/analysis/findings-2026-05-15/silent-success-bugs.md`)
the dominant pattern is "recap reports ✓ but the action didn't
achieve the declared state." This `git.clone` case fits the
pattern:

- recap shows `failed=0`, `changed=0`, `ok=1`
- the actual on-disk state doesn't match the plan
- CI sees green, operator sees green, real state diverges

The fix philosophy from that finding doc applies here: any action
that consciously chose not to converge state should mark itself
`skipped` with a reason — never plain `ok`.

## Test gap

`internal/actions/git_clone/handler_test.go` probably tests:
1. clone fresh — ✓
2. re-apply same plan — idempotent ✓

What's missing:
3. clone fresh at ref=A; re-apply at ref=B with update:false →
   should be marked `skipped` with a reason, not `ok`.

## Workaround

Always set `update: true` if you care about reconciling the ref:

```yaml
- git.clone:
    repo: ...
    dest: ...
    ref: ...
    update: true        # ← explicit "yes please reconcile"
```

This works (I verified — switched from master → test cleanly).
But operators who follow the typical "set the field you care
about, leave others default" pattern will hit this trap.
