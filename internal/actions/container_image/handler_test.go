package container_image

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

// withFake swaps newRuntime to return the given Fake for the duration of
// the test, restoring the original on cleanup.
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
		Svc: &executor.RunServices{
			Template: r,
			PathUtil: pathutil.NewPathExpander(r),
			Logger:   logger.NewLogger(logger.ErrorLevel),
			Mode:     mode,
			Stats:    executor.NewExecutionStats(),
		},
		Scope:      executor.NewVariableScope(),
		CurrentDir: "/tmp",
	}
}

func TestImage_Present_AlreadyExists_NoOp(t *testing.T) {
	fake := containerruntime.NewFake()
	fake.Images["alpine:3.20"] = true
	withFake(t, fake)

	step := &config.Step{ContainerImage: &config.ContainerImage{Name: "alpine:3.20"}}
	res, err := (&Handler{}).Run(newCtx(t, actions.ModeApply), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if r.Changed {
		t.Errorf("expected Changed=false; got true")
	}
	if !strings.Contains(r.Reason, "already present") {
		t.Errorf("reason should say already present; got %q", r.Reason)
	}
}

func TestImage_Present_Missing_Pulls(t *testing.T) {
	fake := containerruntime.NewFake()
	withFake(t, fake)

	step := &config.Step{ContainerImage: &config.ContainerImage{Name: "alpine:3.20"}}
	res, err := (&Handler{}).Run(newCtx(t, actions.ModeApply), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if !r.Changed {
		t.Errorf("expected Changed=true; got false")
	}
	if !fake.Images["alpine:3.20"] {
		t.Errorf("expected image to be pulled")
	}
	wantCalls := []string{"ImageExists:alpine:3.20", "ImagePull:alpine:3.20"}
	if !equalSlices(fake.Calls, wantCalls) {
		t.Errorf("calls = %v, want %v", fake.Calls, wantCalls)
	}
}

func TestImage_Present_Plan_ReportsWouldChange(t *testing.T) {
	fake := containerruntime.NewFake()
	withFake(t, fake)

	step := &config.Step{ContainerImage: &config.ContainerImage{Name: "alpine:3.20"}}
	res, err := (&Handler{}).Run(newCtx(t, actions.ModePlan), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if !r.WouldChange {
		t.Errorf("plan should report WouldChange; got reason %q", r.Reason)
	}
	if fake.Images["alpine:3.20"] {
		t.Errorf("plan must not mutate state")
	}
}

func TestImage_Present_ForcePull(t *testing.T) {
	fake := containerruntime.NewFake()
	fake.Images["alpine:3.20"] = true
	withFake(t, fake)

	step := &config.Step{ContainerImage: &config.ContainerImage{Name: "alpine:3.20", ForcePull: true}}
	res, err := (&Handler{}).Run(newCtx(t, actions.ModeApply), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if !r.Changed {
		t.Errorf("force_pull on existing image should set Changed=true")
	}
	if !contains(fake.Calls, "ImagePull:alpine:3.20") {
		t.Errorf("expected ImagePull call; got %v", fake.Calls)
	}
}

func TestImage_Absent_Existing_Removes(t *testing.T) {
	fake := containerruntime.NewFake()
	fake.Images["alpine:3.20"] = true
	withFake(t, fake)

	step := &config.Step{ContainerImage: &config.ContainerImage{Name: "alpine:3.20", State: "absent"}}
	res, err := (&Handler{}).Run(newCtx(t, actions.ModeApply), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if !r.Changed {
		t.Errorf("expected Changed=true")
	}
	if fake.Images["alpine:3.20"] {
		t.Errorf("image should be removed")
	}
}

func TestImage_Absent_Missing_NoOp(t *testing.T) {
	fake := containerruntime.NewFake()
	withFake(t, fake)

	step := &config.Step{ContainerImage: &config.ContainerImage{Name: "alpine:3.20", State: "absent"}}
	res, err := (&Handler{}).Run(newCtx(t, actions.ModeApply), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if r.Changed {
		t.Errorf("expected Changed=false; got true")
	}
	if !strings.Contains(r.Reason, "already absent") {
		t.Errorf("reason should say already absent; got %q", r.Reason)
	}
}

func TestImage_Validate(t *testing.T) {
	h := &Handler{}
	cases := []struct {
		name    string
		step    *config.Step
		wantErr string
	}{
		{"nil action", &config.Step{}, "requires configuration"},
		{"missing name", &config.Step{ContainerImage: &config.ContainerImage{}}, "name is required"},
		{"bad state", &config.Step{ContainerImage: &config.ContainerImage{Name: "x", State: "weird"}}, "state must be"},
		{"ok", &config.Step{ContainerImage: &config.ContainerImage{Name: "x"}}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := h.Validate(tc.step)
			if tc.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
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
