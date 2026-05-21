package container_run

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/containerruntime"
)

// Step-level env must flow into the engine via Runtime.WithEnv. The
// container action specifically must distinguish step.Env (engine
// subprocess env, e.g. DOCKER_CONFIG) from container.env (`-e VAR=val`
// inside the container) — both can be set independently and neither
// should clobber the other.
func TestEnvPropagation_StepEnvVsContainerEnv(t *testing.T) {
	fake := containerruntime.NewFake()
	withFake(t, fake)

	step := &config.Step{
		Container: &config.Container{
			Name:  "web",
			Image: "alpine:3.20",
			Env:   map[string]string{"IN_CONTAINER": "yes"},
		},
		Env: map[string]string{"DOCKER_CONFIG": "{{ env.HOME }}/.docker-anon"},
	}
	ctx := newCtx(t, actions.ModeApply)
	ctx.Scope.Env = map[string]string{"HOME": "/home/test"}

	if _, err := (&Handler{}).Run(ctx, step); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// step.Env → engine subprocess env (recorded on the fake runtime).
	if fake.Env == nil || fake.Env["DOCKER_CONFIG"] != "/home/test/.docker-anon" {
		t.Errorf("runtime env not set as expected; got %v", fake.Env)
	}
	// container.env → ContainerSpec.Env (passed to ContainerCreate).
	spec, ok := fake.Specs["web"]
	if !ok {
		t.Fatalf("container not created")
	}
	if got := spec.Env["IN_CONTAINER"]; got != "yes" {
		t.Errorf("container env IN_CONTAINER = %q, want %q", got, "yes")
	}
	// Engine knob must NOT leak into container env.
	if _, leaked := spec.Env["DOCKER_CONFIG"]; leaked {
		t.Errorf("step.Env leaked into container env: %v", spec.Env)
	}
}

func TestEnvPropagation_EmptyEnvIsNoop(t *testing.T) {
	fake := containerruntime.NewFake()
	withFake(t, fake)

	step := &config.Step{Container: &config.Container{Name: "web", Image: "alpine:3.20"}}
	if _, err := (&Handler{}).Run(newCtx(t, actions.ModeApply), step); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if fake.Env != nil {
		t.Errorf("expected Env to remain nil with no step.Env; got %v", fake.Env)
	}
}
