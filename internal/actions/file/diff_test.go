package file

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// diffTestCtx builds a minimal ExecutionContext suitable for calling
// Diff(). Reuses mockExecutionContext from handler_test.go but flips
// the mode to Plan, which is the realistic caller (Diff is a planning
// concept) and matches what runtime callers will pass.
func diffTestCtx(t *testing.T) actions.Context {
	t.Helper()
	ec := mockExecutionContext()
	ec.Svc.Mode = actions.ModePlan
	return ec
}

// writeFile is a small helper for setting up Before state in tests.
func writeFile(t *testing.T, path string, content []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, content, mode); err != nil {
		t.Fatalf("seed write %s: %v", path, err)
	}
}

// ----- state=file --------------------------------------------------------

// TestDiff_File_CreateWhenAbsent locks in the basic Operation: writing
// to a missing path is an OpCreate, Before reports Exists=false, After
// carries the desired sha256 + mode.
func TestDiff_File_CreateWhenAbsent(t *testing.T) {
	h := &Handler{}
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")

	step := &config.Step{FileWrite: &config.File{
		Path:    path,
		Content: "hello\n",
		Mode:    "0644",
	}}
	d, err := h.Diff(diffTestCtx(t), step)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpCreate {
		t.Errorf("Operation = %q, want create", d.Operation)
	}
	if d.Resource.Kind != actions.ResourceFile || d.Resource.Identifier != path {
		t.Errorf("Resource = %+v, want kind=file, ident=%q", d.Resource, path)
	}
	before := d.Before.(*FileSnapshot)
	if before.Exists {
		t.Error("Before.Exists = true, want false (path is absent)")
	}
	after := d.After.(*FileSnapshot)
	if !after.Exists || after.Kind != "file" {
		t.Errorf("After = %+v, want Exists=true, Kind=file", after)
	}
	if after.Size != int64(len("hello\n")) {
		t.Errorf("After.Size = %d, want %d", after.Size, len("hello\n"))
	}
	if after.Sha256 == "" {
		t.Error("After.Sha256 empty; want populated for regular file")
	}
}

// TestDiff_File_NoopWhenContentMatches — content + mode identical →
// OpNoop. Lock this in because the cheap Sha256 + Mode comparison is
// the whole point of the contract: a re-plan against a converged
// system should report nothing changed.
func TestDiff_File_NoopWhenContentMatches(t *testing.T) {
	h := &Handler{}
	dir := t.TempDir()
	path := filepath.Join(dir, "x.txt")
	writeFile(t, path, []byte("same\n"), 0o644)

	step := &config.Step{FileWrite: &config.File{
		Path:    path,
		Content: "same\n",
		Mode:    "0644",
	}}
	d, err := h.Diff(diffTestCtx(t), step)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpNoop {
		t.Errorf("Operation = %q, want noop (content + mode match)", d.Operation)
	}
}

// TestDiff_File_UpdateWhenContentDiffers — different content → update.
// Sha256 in Before and After must differ; that's how a downstream
// consumer knows the file changed without re-reading both versions.
func TestDiff_File_UpdateWhenContentDiffers(t *testing.T) {
	h := &Handler{}
	dir := t.TempDir()
	path := filepath.Join(dir, "x.txt")
	writeFile(t, path, []byte("old\n"), 0o644)

	step := &config.Step{FileWrite: &config.File{
		Path:    path,
		Content: "new\n",
		Mode:    "0644",
	}}
	d, err := h.Diff(diffTestCtx(t), step)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpUpdate {
		t.Errorf("Operation = %q, want update", d.Operation)
	}
	before := d.Before.(*FileSnapshot)
	after := d.After.(*FileSnapshot)
	if before.Sha256 == "" || after.Sha256 == "" {
		t.Errorf("both snapshots should carry Sha256; got before=%q after=%q", before.Sha256, after.Sha256)
	}
	if before.Sha256 == after.Sha256 {
		t.Errorf("Sha256 identical across update; want different. before=%q after=%q", before.Sha256, after.Sha256)
	}
}

// TestDiff_File_UpdateWhenModeDiffers — content same, mode different.
// Confirms mode is part of the equality check; otherwise chmod-only
// steps would be silently classified as noop.
func TestDiff_File_UpdateWhenModeDiffers(t *testing.T) {
	h := &Handler{}
	dir := t.TempDir()
	path := filepath.Join(dir, "x.txt")
	writeFile(t, path, []byte("same\n"), 0o600)

	step := &config.Step{FileWrite: &config.File{
		Path:    path,
		Content: "same\n",
		Mode:    "0644",
	}}
	d, err := h.Diff(diffTestCtx(t), step)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpUpdate {
		t.Errorf("Operation = %q, want update (mode differs)", d.Operation)
	}
}

// ----- state=absent ------------------------------------------------------

func TestDiff_Absent_DeleteWhenPresent(t *testing.T) {
	h := &Handler{}
	dir := t.TempDir()
	path := filepath.Join(dir, "delete-me.txt")
	writeFile(t, path, []byte("doomed"), 0o644)

	step := &config.Step{FileWrite: &config.File{Path: path, State: "absent"}}
	d, err := h.Diff(diffTestCtx(t), step)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpDelete {
		t.Errorf("Operation = %q, want delete", d.Operation)
	}
	if d.After != nil {
		t.Errorf("After = %+v, want nil for delete", d.After)
	}
	if !d.Before.(*FileSnapshot).Exists {
		t.Error("Before.Exists = false, want true")
	}
}

func TestDiff_Absent_NoopWhenMissing(t *testing.T) {
	h := &Handler{}
	dir := t.TempDir()
	path := filepath.Join(dir, "not-there")

	step := &config.Step{FileWrite: &config.File{Path: path, State: "absent"}}
	d, err := h.Diff(diffTestCtx(t), step)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpNoop {
		t.Errorf("Operation = %q, want noop", d.Operation)
	}
}

// ----- state=directory ---------------------------------------------------

func TestDiff_Directory_CreateWhenAbsent(t *testing.T) {
	h := &Handler{}
	dir := t.TempDir()
	target := filepath.Join(dir, "newdir")

	step := &config.Step{FileWrite: &config.File{Path: target, State: "directory", Mode: "0755"}}
	d, err := h.Diff(diffTestCtx(t), step)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpCreate {
		t.Errorf("Operation = %q, want create", d.Operation)
	}
	if d.After.(*FileSnapshot).Kind != "directory" {
		t.Errorf("After.Kind = %q, want directory", d.After.(*FileSnapshot).Kind)
	}
}

func TestDiff_Directory_NoopWhenMatchingMode(t *testing.T) {
	h := &Handler{}
	dir := t.TempDir()
	target := filepath.Join(dir, "existing")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	step := &config.Step{FileWrite: &config.File{Path: target, State: "directory", Mode: "0755"}}
	d, err := h.Diff(diffTestCtx(t), step)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpNoop {
		t.Errorf("Operation = %q, want noop", d.Operation)
	}
}

// ----- state=touch -------------------------------------------------------

// TestDiff_Touch_NeverNoop — touch always bumps mtime, so even an
// already-existing file with matching mode reports OpUpdate. Locked in
// because a future optimisation might be tempted to dedupe it.
func TestDiff_Touch_NeverNoop(t *testing.T) {
	h := &Handler{}
	dir := t.TempDir()
	path := filepath.Join(dir, "exists.txt")
	writeFile(t, path, []byte(""), 0o644)

	step := &config.Step{FileWrite: &config.File{Path: path, State: "touch", Mode: "0644"}}
	d, err := h.Diff(diffTestCtx(t), step)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpUpdate {
		t.Errorf("Operation = %q, want update (touch always changes mtime)", d.Operation)
	}
}

func TestDiff_Touch_CreateWhenAbsent(t *testing.T) {
	h := &Handler{}
	dir := t.TempDir()
	path := filepath.Join(dir, "fresh.touch")

	step := &config.Step{FileWrite: &config.File{Path: path, State: "touch"}}
	d, err := h.Diff(diffTestCtx(t), step)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpCreate {
		t.Errorf("Operation = %q, want create", d.Operation)
	}
}

// ----- state=link --------------------------------------------------------

func TestDiff_Link_CreateWhenAbsent(t *testing.T) {
	h := &Handler{}
	dir := t.TempDir()
	src := filepath.Join(dir, "target")
	writeFile(t, src, []byte("real"), 0o644)
	linkPath := filepath.Join(dir, "link")

	step := &config.Step{FileWrite: &config.File{Path: linkPath, State: "link", Src: src}}
	d, err := h.Diff(diffTestCtx(t), step)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpCreate {
		t.Errorf("Operation = %q, want create", d.Operation)
	}
	if d.After.(*FileSnapshot).Kind != "symlink" {
		t.Errorf("After.Kind = %q, want symlink", d.After.(*FileSnapshot).Kind)
	}
	if d.After.(*FileSnapshot).Target != src {
		t.Errorf("After.Target = %q, want %q", d.After.(*FileSnapshot).Target, src)
	}
}

func TestDiff_Link_NoopWhenTargetMatches(t *testing.T) {
	h := &Handler{}
	dir := t.TempDir()
	src := filepath.Join(dir, "target")
	writeFile(t, src, []byte("real"), 0o644)
	linkPath := filepath.Join(dir, "link")
	if err := os.Symlink(src, linkPath); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	step := &config.Step{FileWrite: &config.File{Path: linkPath, State: "link", Src: src}}
	d, err := h.Diff(diffTestCtx(t), step)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpNoop {
		t.Errorf("Operation = %q, want noop (target already matches)", d.Operation)
	}
}

// ----- state=perms -------------------------------------------------------

func TestDiff_Perms_NoopWhenMatching(t *testing.T) {
	h := &Handler{}
	dir := t.TempDir()
	path := filepath.Join(dir, "x.txt")
	writeFile(t, path, []byte(""), 0o600)

	step := &config.Step{FileWrite: &config.File{Path: path, State: "perms", Mode: "0600"}}
	d, err := h.Diff(diffTestCtx(t), step)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpNoop {
		t.Errorf("Operation = %q, want noop (mode matches)", d.Operation)
	}
}

func TestDiff_Perms_UpdateWhenDiffers(t *testing.T) {
	h := &Handler{}
	dir := t.TempDir()
	path := filepath.Join(dir, "x.txt")
	writeFile(t, path, []byte(""), 0o600)

	step := &config.Step{FileWrite: &config.File{Path: path, State: "perms", Mode: "0644"}}
	d, err := h.Diff(diffTestCtx(t), step)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpUpdate {
		t.Errorf("Operation = %q, want update", d.Operation)
	}
}

// ----- defensive edge cases ---------------------------------------------

// TestDiff_NilStep — Diff must error gracefully on a nil/empty step
// rather than panicking. The contract is "give me a useful answer or
// an error"; nil step gets an error.
func TestDiff_NilStep(t *testing.T) {
	h := &Handler{}
	if _, err := h.Diff(diffTestCtx(t), nil); err == nil {
		t.Error("Diff(nil) should return an error")
	}
	if _, err := h.Diff(diffTestCtx(t), &config.Step{}); err == nil {
		t.Error("Diff(empty step) should return an error")
	}
}

// TestDiff_HandlerIsRegisteredAsDiffer locks in interface satisfaction.
// Spec-22 registers handlers via actions.IsDiffer for capability
// reporting; if a future refactor narrows the receiver type and
// silently breaks satisfaction, this catches it.
func TestDiff_HandlerIsRegisteredAsDiffer(t *testing.T) {
	if !actions.IsDiffer(&Handler{}) {
		t.Error("*Handler should satisfy actions.Differ")
	}
}

// TestDiff_TemplatedContentExpanded confirms the content gets rendered
// through the Pongo2 template engine the same way Run does. Without
// this, a step that uses {{ var }} in Content would compute the wrong
// After Sha256 and miscompute the Operation.
func TestDiff_TemplatedContentExpanded(t *testing.T) {
	h := &Handler{}
	dir := t.TempDir()
	path := filepath.Join(dir, "templated.txt")

	step := &config.Step{FileWrite: &config.File{
		Path:    path,
		Content: "value={{ x }}\n",
		Mode:    "0644",
	}}
	ec := mockExecutionContext()
	ec.Svc.Mode = actions.ModePlan
	ec.Scope.User["x"] = "42"

	d, err := h.Diff(ec, step)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	after := d.After.(*FileSnapshot)
	want := int64(len("value=42\n"))
	if after.Size != want {
		t.Errorf("After.Size = %d, want %d (template should have rendered '42')", after.Size, want)
	}
}
