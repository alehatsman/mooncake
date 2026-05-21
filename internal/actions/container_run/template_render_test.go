package container_run

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/containerruntime"
)

// TestRun_RendersSpecFields is a regression test: previously only
// container.name and container.image were template-rendered, while
// volumes/ports/env/command/extra/network/restart were passed to the
// engine verbatim. A `volumes: ["{{ env.HOME }}/.cache:/app/.cache"]`
// reached docker as the literal `{{ env.HOME }}/...` string and docker
// rejected the volume name. Every user-controllable spec field must
// now flow through the templater.
func TestRun_RendersSpecFields(t *testing.T) {
	fake := containerruntime.NewFake()
	withFake(t, fake)

	ctx := newCtx(t, actions.ModeApply)
	ctx.Scope.Env = map[string]string{"HOME": "/home/tester"}
	ctx.Scope.User["port"] = "7997"
	ctx.Scope.User["net"] = "host"
	ctx.Scope.User["restart_policy"] = "unless-stopped"
	ctx.Scope.User["cache_dir"] = "/var/cache/x"

	step := &config.Step{Container: &config.Container{
		Name:    "rerank",
		Image:   "infinity:latest",
		Command: []string{"--data-dir", "{{ env.HOME }}/data"},
		Env:     map[string]string{"HF_HOME": "{{ cache_dir }}"},
		Ports:   []string{"127.0.0.1:{{ port }}:7997"},
		Volumes: []string{"{{ env.HOME }}/.cache:/app/.cache"},
		Network: "{{ net }}",
		Restart: "{{ restart_policy }}",
		Extra:   []string{"--gpus", "all"},
	}}

	if _, err := (&Handler{}).Run(ctx, step); err != nil {
		t.Fatalf("Run: %v", err)
	}

	got, ok := fake.Specs["rerank"]
	if !ok {
		t.Fatalf("ContainerCreate never received a spec: calls=%v", fake.Calls)
	}

	wantCommand := []string{"--data-dir", "/home/tester/data"}
	if !equalSlices(got.Command, wantCommand) {
		t.Errorf("command not rendered:\n got = %v\nwant = %v", got.Command, wantCommand)
	}
	if got.Env["HF_HOME"] != "/var/cache/x" {
		t.Errorf("env[HF_HOME] = %q, want /var/cache/x", got.Env["HF_HOME"])
	}
	wantPorts := []string{"127.0.0.1:7997:7997"}
	if !equalSlices(got.Ports, wantPorts) {
		t.Errorf("ports not rendered:\n got = %v\nwant = %v", got.Ports, wantPorts)
	}
	wantVolumes := []string{"/home/tester/.cache:/app/.cache"}
	if !equalSlices(got.Volumes, wantVolumes) {
		t.Errorf("volumes not rendered:\n got = %v\nwant = %v", got.Volumes, wantVolumes)
	}
	if got.Network != "host" {
		t.Errorf("network = %q, want host", got.Network)
	}
	if got.Restart != "unless-stopped" {
		t.Errorf("restart = %q, want unless-stopped", got.Restart)
	}
	// Extra has no templated values here; it must still survive intact.
	wantExtra := []string{"--gpus", "all"}
	if !equalSlices(got.Extra, wantExtra) {
		t.Errorf("extra mangled:\n got = %v\nwant = %v", got.Extra, wantExtra)
	}
}

// TestRun_RenderError_VolumeFieldNamedInError ensures the error path
// names the field and index so users can locate the offending entry
// in playbooks with many volumes.
func TestRun_RenderError_VolumeFieldNamedInError(t *testing.T) {
	fake := containerruntime.NewFake()
	withFake(t, fake)

	step := &config.Step{Container: &config.Container{
		Name:    "broken",
		Image:   "alpine:3.20",
		Volumes: []string{"/ok:/ok", "{{ unclosed "},
	}}

	_, err := (&Handler{}).Run(newCtx(t, actions.ModeApply), step)
	if err == nil {
		t.Fatal("expected render error, got nil")
	}
	msg := err.Error()
	if !containsAny(msg, "container.volumes[1]") {
		t.Errorf("error should locate the bad field: %q", msg)
	}
}

func containsAny(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
