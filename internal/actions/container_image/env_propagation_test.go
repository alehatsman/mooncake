package container_image

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/containerruntime"
)

// Step-level env must flow into the runtime via Runtime.WithEnv so
// users can set DOCKER_CONFIG / DOCKER_HOST per action. Without this,
// the WSL/Docker-Desktop credsStore workaround that motivated the
// feature (anonymous pulls failing because credsStore=desktop.exe is
// unreachable from WSL) cannot be expressed in YAML — and any other
// per-action engine knob is also unreachable.
func TestEnvPropagation_RendersAndCallsWithEnv(t *testing.T) {
	fake := containerruntime.NewFake()
	withFake(t, fake)

	step := &config.Step{
		ContainerImage: &config.ContainerImage{Name: "alpine:3.20"},
		Env: map[string]string{
			"DOCKER_CONFIG": "{{ env.HOME }}/.docker-anon",
			"PLAIN":         "literal",
		},
	}
	ctx := newCtx(t, actions.ModeApply)
	// Seed env.HOME so the template renderer has something to expand.
	ctx.Scope.Env = map[string]string{"HOME": "/home/test"}

	if _, err := (&Handler{}).Run(ctx, step); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if fake.Env == nil {
		t.Fatalf("expected runtime to receive env via WithEnv, got nil")
	}
	if got := fake.Env["DOCKER_CONFIG"]; got != "/home/test/.docker-anon" {
		t.Errorf("DOCKER_CONFIG = %q, want %q", got, "/home/test/.docker-anon")
	}
	if got := fake.Env["PLAIN"]; got != "literal" {
		t.Errorf("PLAIN = %q, want %q", got, "literal")
	}
}

// When step.Env is empty (the common case), WithEnv must not mutate
// the fake's Env tracker — i.e., the no-env path is a true no-op.
func TestEnvPropagation_EmptyEnvIsNoop(t *testing.T) {
	fake := containerruntime.NewFake()
	withFake(t, fake)

	step := &config.Step{ContainerImage: &config.ContainerImage{Name: "alpine:3.20"}}
	if _, err := (&Handler{}).Run(newCtx(t, actions.ModeApply), step); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if fake.Env != nil {
		t.Errorf("expected Env to remain nil with no step.Env; got %v", fake.Env)
	}
}
