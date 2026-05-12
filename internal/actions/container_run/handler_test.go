package container_run

import (
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/containerruntime"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/template"
)

func withFake(t *testing.T, fake *containerruntime.Fake) {
	t.Helper()
	orig := newRuntime
	newRuntime = func(_ string) (containerruntime.Runtime, error) { return fake, nil }
	t.Cleanup(func() { newRuntime = orig })
}

func newCtx(t *testing.T, mode actions.Mode) *executor.ExecutionContext {
	t.Helper()
	r, err := template.NewPongo2Renderer()
	if err != nil {
		t.Fatal(err)
	}
	return &executor.ExecutionContext{
		Variables:   map[string]interface{}{},
		Template:    r,
		PathUtil:    pathutil.NewPathExpander(r),
		Logger:      logger.NewLogger(logger.ErrorLevel),
		CurrentDir:  "/tmp",
		CurrentMode: mode,
		Stats:       executor.NewExecutionStats(),
	}
}

func TestRun_Running_Missing_CreatesAndStarts(t *testing.T) {
	fake := containerruntime.NewFake()
	withFake(t, fake)

	step := &config.Step{Container: &config.Container{Name: "web", Image: "alpine:3.20"}}
	res, err := (&Handler{}).Run(newCtx(t, actions.ModeApply), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if !r.Changed {
		t.Errorf("expected Changed=true")
	}
	c, ok := fake.Containers["web"]
	if !ok || !c.Running || c.Image != "alpine:3.20" {
		t.Errorf("container not created or wrong state: %+v ok=%v", c, ok)
	}
}

func TestRun_Running_AlreadyRunning_NoOp(t *testing.T) {
	fake := containerruntime.NewFake()
	fake.Containers["web"] = &containerruntime.ContainerState{Exists: true, Running: true, Image: "alpine:3.20"}
	withFake(t, fake)

	step := &config.Step{Container: &config.Container{Name: "web", Image: "alpine:3.20"}}
	res, err := (&Handler{}).Run(newCtx(t, actions.ModeApply), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if r.Changed {
		t.Errorf("expected Changed=false; reason=%q", r.Reason)
	}
	if !strings.Contains(r.Reason, "already running") {
		t.Errorf("reason should say already running; got %q", r.Reason)
	}
}

func TestRun_Running_StoppedExisting_Starts(t *testing.T) {
	fake := containerruntime.NewFake()
	fake.Containers["web"] = &containerruntime.ContainerState{Exists: true, Running: false, Image: "alpine:3.20"}
	withFake(t, fake)

	step := &config.Step{Container: &config.Container{Name: "web", Image: "alpine:3.20"}}
	res, err := (&Handler{}).Run(newCtx(t, actions.ModeApply), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if !r.Changed {
		t.Errorf("expected Changed=true")
	}
	if !contains(fake.Calls, "ContainerStart:web") {
		t.Errorf("expected ContainerStart; got %v", fake.Calls)
	}
}

func TestRun_Stopped_Running_Stops(t *testing.T) {
	fake := containerruntime.NewFake()
	fake.Containers["web"] = &containerruntime.ContainerState{Exists: true, Running: true, Image: "alpine:3.20"}
	withFake(t, fake)

	step := &config.Step{Container: &config.Container{Name: "web", Image: "alpine:3.20", State: "stopped"}}
	res, err := (&Handler{}).Run(newCtx(t, actions.ModeApply), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if !r.Changed {
		t.Errorf("expected Changed=true")
	}
	if !contains(fake.Calls, "ContainerStop:web") {
		t.Errorf("expected ContainerStop; got %v", fake.Calls)
	}
}

func TestRun_Absent_Existing_Removes(t *testing.T) {
	fake := containerruntime.NewFake()
	fake.Containers["web"] = &containerruntime.ContainerState{Exists: true, Running: true}
	withFake(t, fake)

	step := &config.Step{Container: &config.Container{Name: "web", State: "absent"}}
	res, err := (&Handler{}).Run(newCtx(t, actions.ModeApply), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if !r.Changed {
		t.Errorf("expected Changed=true")
	}
	if _, exists := fake.Containers["web"]; exists {
		t.Errorf("container should be removed")
	}
}

func TestRun_Absent_Missing_NoOp(t *testing.T) {
	fake := containerruntime.NewFake()
	withFake(t, fake)

	step := &config.Step{Container: &config.Container{Name: "web", State: "absent"}}
	res, err := (&Handler{}).Run(newCtx(t, actions.ModeApply), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if r.Changed {
		t.Errorf("expected Changed=false")
	}
	if !strings.Contains(r.Reason, "already absent") {
		t.Errorf("reason should say already absent; got %q", r.Reason)
	}
}

func TestRun_Recreate_DropsAndCreates(t *testing.T) {
	fake := containerruntime.NewFake()
	fake.Containers["web"] = &containerruntime.ContainerState{Exists: true, Running: true, Image: "alpine:3.20"}
	withFake(t, fake)

	step := &config.Step{Container: &config.Container{Name: "web", Image: "alpine:3.20", Recreate: true}}
	res, err := (&Handler{}).Run(newCtx(t, actions.ModeApply), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if !r.Changed {
		t.Errorf("expected Changed=true")
	}
	// Order matters: remove then create.
	wantSeq := []string{"ContainerInspect:web", "ContainerRemove:web", "ContainerCreate:web"}
	if !equalSlices(fake.Calls, wantSeq) {
		t.Errorf("call sequence wrong:\n got = %v\nwant = %v", fake.Calls, wantSeq)
	}
}

// Image-drift auto-recreate is intentionally NOT supported in the MVP:
// engines resolve short refs to fully-qualified ones on inspect, so a
// string-match heuristic gives false positives. With a container in
// the desired running state, the action is a no-op regardless of the
// image-string the caller passed.
func TestRun_ImageStringMismatch_NoOp(t *testing.T) {
	fake := containerruntime.NewFake()
	fake.Containers["web"] = &containerruntime.ContainerState{Exists: true, Running: true, Image: "docker.io/library/alpine:3.20"}
	withFake(t, fake)

	step := &config.Step{Container: &config.Container{Name: "web", Image: "alpine:3.20"}}
	res, err := (&Handler{}).Run(newCtx(t, actions.ModeApply), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if r.Changed {
		t.Errorf("expected Changed=false (no drift heuristic); got reason=%q", r.Reason)
	}
	if contains(fake.Calls, "ContainerRemove:web") || contains(fake.Calls, "ContainerCreate:web") {
		t.Errorf("must not touch container in MVP; got %v", fake.Calls)
	}
}

func TestRun_Plan_Reports(t *testing.T) {
	fake := containerruntime.NewFake()
	withFake(t, fake)

	step := &config.Step{Container: &config.Container{Name: "web", Image: "alpine:3.20"}}
	res, err := (&Handler{}).Run(newCtx(t, actions.ModePlan), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if !r.WouldChange {
		t.Errorf("plan should report WouldChange")
	}
	if !strings.Contains(r.Reason, "create+start") {
		t.Errorf("plan reason should describe create+start; got %q", r.Reason)
	}
	if _, exists := fake.Containers["web"]; exists {
		t.Errorf("plan must not mutate state")
	}
}

func TestRun_Stopped_Missing_CreatesThenStops(t *testing.T) {
	fake := containerruntime.NewFake()
	withFake(t, fake)

	step := &config.Step{Container: &config.Container{Name: "web", Image: "alpine:3.20", State: "stopped"}}
	res, err := (&Handler{}).Run(newCtx(t, actions.ModeApply), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if !r.Changed {
		t.Errorf("expected Changed=true")
	}
	if fake.Containers["web"] == nil || fake.Containers["web"].Running {
		t.Errorf("container should exist and be stopped; got %+v", fake.Containers["web"])
	}
}

func TestRun_Validate(t *testing.T) {
	h := &Handler{}
	cases := []struct {
		name    string
		step    *config.Step
		wantErr string
	}{
		{"nil", &config.Step{}, "requires configuration"},
		{"no name", &config.Step{Container: &config.Container{Image: "x"}}, "name is required"},
		{"no image for running", &config.Step{Container: &config.Container{Name: "web"}}, "image is required"},
		{"bad state", &config.Step{Container: &config.Container{Name: "web", Image: "x", State: "weird"}}, "state must be"},
		{"absent ok without image", &config.Step{Container: &config.Container{Name: "web", State: "absent"}}, ""},
		{"running ok", &config.Step{Container: &config.Container{Name: "web", Image: "alpine"}}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := h.Validate(tc.step)
			if tc.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("err = %v, want substring %q", err, tc.wantErr)
			}
		})
	}
}

func contains(slice []string, s string) bool {
	for _, e := range slice {
		if e == s {
			return true
		}
	}
	return false
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
