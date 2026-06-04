package cron

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/urfave/cli/v2"
)

// TestMain disables cli's default os.Exit-on-ExitCoder so that tests can
// inspect non-zero exit errors returned from app.Run without killing the process.
func TestMain(m *testing.M) {
	cli.OsExiter = func(_ int) {}
	os.Exit(m.Run())
}

func newTestApp(w io.Writer) *cli.App {
	app := &cli.App{Writer: w}
	app.Commands = []*cli.Command{Command()}
	return app
}

func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	app := newTestApp(&buf)
	err := app.Run(append([]string{"mooncake"}, args...))
	return buf.String(), err
}

func writeCronFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// ---- parseCronFile ----

func TestParseCronFile_Managed(t *testing.T) {
	content := "# Managed by mooncake os.cron\n0 2 * * *  root  /usr/bin/backup.sh\n"
	entries := parseCronFile("backup", true, content)
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Schedule != "0 2 * * *" {
		t.Errorf("schedule: got %q", e.Schedule)
	}
	if e.User != "root" {
		t.Errorf("user: got %q", e.User)
	}
	if e.Command != "/usr/bin/backup.sh" {
		t.Errorf("command: got %q", e.Command)
	}
	if !e.Managed {
		t.Error("managed should be true")
	}
}

func TestParseCronFile_WithEnvVars(t *testing.T) {
	content := "# Managed by mooncake os.cron\nMAILTO=root\nSHELL=/bin/sh\n*/5 * * * *  deploy  /app/run.sh --arg\n"
	entries := parseCronFile("myapp", true, content)
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d: %+v", len(entries), entries)
	}
	e := entries[0]
	if e.Schedule != "*/5 * * * *" {
		t.Errorf("schedule: got %q", e.Schedule)
	}
	if e.Command != "/app/run.sh --arg" {
		t.Errorf("command: got %q", e.Command)
	}
}

func TestParseCronFile_AtReboot(t *testing.T) {
	content := "# Managed by mooncake os.cron\n@reboot root /usr/bin/start-agent\n"
	entries := parseCronFile("agent", true, content)
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	if entries[0].Schedule != "@reboot" {
		t.Errorf("schedule: got %q", entries[0].Schedule)
	}
}

func TestParseCronFile_MultiEntry(t *testing.T) {
	content := "# system cron\n0 1 * * * www-data /usr/bin/job1\n0 2 * * 0 www-data /usr/bin/job2\n"
	entries := parseCronFile("system", false, content)
	if len(entries) != 2 {
		t.Fatalf("want 2 entries, got %d", len(entries))
	}
}

func TestParseCronFile_SkipsShortLines(t *testing.T) {
	content := "not a valid cron line\n"
	entries := parseCronFile("bad", false, content)
	if len(entries) != 0 {
		t.Errorf("want 0 entries, got %d", len(entries))
	}
}

// ---- readCronDir ----

func TestReadCronDir_Empty(t *testing.T) {
	dir := t.TempDir()
	entries, err := readCronDir(dir, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty, got %d", len(entries))
	}
}

func TestReadCronDir_MissingDir(t *testing.T) {
	entries, err := readCronDir("/nonexistent/path/cron.d", false)
	if err != nil {
		t.Fatalf("missing dir should not error, got: %v", err)
	}
	if entries != nil {
		t.Errorf("expected nil, got %v", entries)
	}
}

func TestReadCronDir_ManagedOnly(t *testing.T) {
	dir := t.TempDir()
	writeCronFile(t, dir, "managed-job", "# Managed by mooncake os.cron\n0 * * * * root /bin/managed\n")
	writeCronFile(t, dir, "other-job", "# not managed\n0 * * * * root /bin/other\n")

	entries, err := readCronDir(dir, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 managed entry, got %d", len(entries))
	}
	if entries[0].Name != "managed-job" {
		t.Errorf("name: got %q", entries[0].Name)
	}
}

func TestReadCronDir_All(t *testing.T) {
	dir := t.TempDir()
	writeCronFile(t, dir, "managed-job", "# Managed by mooncake os.cron\n0 * * * * root /bin/managed\n")
	writeCronFile(t, dir, "other-job", "# not managed\n0 * * * * root /bin/other\n")

	entries, err := readCronDir(dir, true)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 entries, got %d", len(entries))
	}
}

func TestReadCronDir_Sorted(t *testing.T) {
	dir := t.TempDir()
	writeCronFile(t, dir, "zzz-job", "# Managed by mooncake os.cron\n0 * * * * root /bin/z\n")
	writeCronFile(t, dir, "aaa-job", "# Managed by mooncake os.cron\n0 * * * * root /bin/a\n")

	entries, err := readCronDir(dir, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 entries, got %d", len(entries))
	}
	if entries[0].Name != "aaa-job" || entries[1].Name != "zzz-job" {
		t.Errorf("not sorted: %v %v", entries[0].Name, entries[1].Name)
	}
}

// ---- CLI integration ----

func TestCronList_Empty(t *testing.T) {
	dir := t.TempDir()
	out, err := run(t, "cron", "list", "--dir", dir)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(out, "no cron entries") {
		t.Errorf("expected empty message, got: %q", out)
	}
}

func TestCronList_Table(t *testing.T) {
	dir := t.TempDir()
	writeCronFile(t, dir, "backup", "# Managed by mooncake os.cron\n0 2 * * * root /usr/bin/backup.sh\n")

	out, err := run(t, "cron", "list", "--dir", dir)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(out, "backup") {
		t.Errorf("expected 'backup' in output, got: %q", out)
	}
	if !strings.Contains(out, "0 2 * * *") {
		t.Errorf("expected schedule in output, got: %q", out)
	}
	if !strings.Contains(out, "root") {
		t.Errorf("expected user in output, got: %q", out)
	}
}

func TestCronList_JSON(t *testing.T) {
	dir := t.TempDir()
	writeCronFile(t, dir, "backup", "# Managed by mooncake os.cron\n0 2 * * * root /usr/bin/backup.sh\n")

	out, err := run(t, "cron", "list", "--dir", dir, "--json")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	var entries []cronEntry
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		t.Fatalf("json parse: %v (output: %q)", err, out)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	if entries[0].Name != "backup" {
		t.Errorf("name: got %q", entries[0].Name)
	}
}

func TestCronList_JSONEmpty(t *testing.T) {
	dir := t.TempDir()
	out, err := run(t, "cron", "list", "--dir", dir, "--json")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	var entries []cronEntry
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		t.Fatalf("json parse: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("want empty array, got %d", len(entries))
	}
}

func TestCronList_AllFlag(t *testing.T) {
	dir := t.TempDir()
	writeCronFile(t, dir, "managed", "# Managed by mooncake os.cron\n0 * * * * root /bin/m\n")
	writeCronFile(t, dir, "unmanaged", "# system\n0 * * * * root /bin/u\n")

	out, err := run(t, "cron", "list", "--dir", dir, "--all")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(out, "managed") || !strings.Contains(out, "unmanaged") {
		t.Errorf("expected both entries, got: %q", out)
	}
}

func TestCronList_ManagedColumn(t *testing.T) {
	dir := t.TempDir()
	writeCronFile(t, dir, "managed", "# Managed by mooncake os.cron\n0 * * * * root /bin/m\n")
	writeCronFile(t, dir, "system", "# system\n0 * * * * root /bin/s\n")

	out, err := run(t, "cron", "list", "--dir", dir, "--all")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	var managedLine, systemLine string
	for _, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "managed") {
			managedLine = l
		}
		if strings.HasPrefix(strings.TrimSpace(l), "system") {
			systemLine = l
		}
	}
	if !strings.Contains(managedLine, "yes") {
		t.Errorf("managed line should contain 'yes': %q", managedLine)
	}
	if !strings.Contains(systemLine, "no") {
		t.Errorf("system line should contain 'no': %q", systemLine)
	}
}

// ---- cron remove ----

func TestCronRemove_Managed(t *testing.T) {
	dir := t.TempDir()
	writeCronFile(t, dir, "backup", "# Managed by mooncake os.cron\n0 2 * * * root /bin/backup\n")

	out, err := run(t, "cron", "remove", "--dir", dir, "backup")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(out, "removed") {
		t.Errorf("expected 'removed' in output, got: %q", out)
	}
	if _, err := os.Stat(filepath.Join(dir, "backup")); !os.IsNotExist(err) {
		t.Error("file should be gone")
	}
}

func TestCronRemove_NotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := run(t, "cron", "remove", "--dir", dir, "ghost")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	exitErr, ok := err.(cli.ExitCoder)
	if !ok || exitErr.ExitCode() != 1 {
		t.Errorf("expected exit code 1, got: %v", err)
	}
}

func TestCronRemove_NotManagedBlocked(t *testing.T) {
	dir := t.TempDir()
	writeCronFile(t, dir, "system-job", "# system cron\n0 * * * * root /bin/x\n")

	_, err := run(t, "cron", "remove", "--dir", dir, "system-job")
	if err == nil {
		t.Fatal("expected error for unmanaged file without --force")
	}
	exitErr, ok := err.(cli.ExitCoder)
	if !ok || exitErr.ExitCode() != 2 {
		t.Errorf("expected exit code 2, got: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "system-job")); statErr != nil {
		t.Error("file should still exist")
	}
}

func TestCronRemove_Force(t *testing.T) {
	dir := t.TempDir()
	writeCronFile(t, dir, "system-job", "# system cron\n0 * * * * root /bin/x\n")

	_, err := run(t, "cron", "remove", "--dir", dir, "--force", "system-job")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "system-job")); !os.IsNotExist(statErr) {
		t.Error("file should be gone after --force remove")
	}
}

func TestCronRemove_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	_, err := run(t, "cron", "remove", "--dir", dir, "../etc/passwd")
	if err == nil {
		t.Fatal("expected error for path traversal")
	}
}

func TestCronRemove_MissingArg(t *testing.T) {
	dir := t.TempDir()
	_, err := run(t, "cron", "remove", "--dir", dir)
	if err == nil {
		t.Fatal("expected error for missing arg")
	}
}
