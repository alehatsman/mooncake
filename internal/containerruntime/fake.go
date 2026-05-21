package containerruntime

import (
	"context"
	"fmt"
)

// Fake is a Runtime implementation backed by in-memory state. It is the
// canonical test double used by container action tests. Behavior:
//
//   - Images: tracked as a set in Images. Pull adds; Remove deletes.
//   - Containers: tracked in Containers, keyed by name. Create stores
//     the spec; Start/Stop flips Running; Remove deletes.
//
// Calls records every operation in order so tests can assert on the
// exact sequence ("pull X, then create Y, then start Y").
type Fake struct {
	Images     map[string]bool
	Containers map[string]*ContainerState
	Specs      map[string]ContainerSpec
	Calls      []string

	// Env records the merged engine env from the latest WithEnv call,
	// so tests can assert that handlers plumb step.Env through to the
	// runtime. Nil until WithEnv is called.
	Env map[string]string

	// FailOn lets a test simulate engine failures. The key is the call
	// name (e.g. "ImagePull"); when set, the next call of that kind
	// returns the stored error and is NOT recorded in Calls.
	FailOn map[string]error
}

// NewFake returns an empty Fake ready for use.
func NewFake() *Fake {
	return &Fake{
		Images:     make(map[string]bool),
		Containers: make(map[string]*ContainerState),
		Specs:      make(map[string]ContainerSpec),
		FailOn:     make(map[string]error),
	}
}

func (f *Fake) Name() string { return "fake" }

// WithEnv records the env on the receiver and returns it. Returning
// the receiver (rather than a copy) keeps existing test code working:
// handlers can call rt = rt.WithEnv(env) without losing the reference
// the test holds for post-call assertions.
func (f *Fake) WithEnv(env map[string]string) Runtime {
	if len(env) == 0 {
		return f
	}
	merged := make(map[string]string, len(f.Env)+len(env))
	for k, v := range f.Env {
		merged[k] = v
	}
	for k, v := range env {
		merged[k] = v
	}
	f.Env = merged
	return f
}

func (f *Fake) record(op string) error {
	if err, ok := f.FailOn[op]; ok && err != nil {
		delete(f.FailOn, op)
		return err
	}
	f.Calls = append(f.Calls, op)
	return nil
}

func (f *Fake) ImageExists(_ context.Context, ref string) (bool, error) {
	if err := f.record("ImageExists:" + ref); err != nil {
		return false, err
	}
	return f.Images[ref], nil
}

func (f *Fake) ImagePull(_ context.Context, ref string) error {
	if err := f.record("ImagePull:" + ref); err != nil {
		return err
	}
	f.Images[ref] = true
	return nil
}

func (f *Fake) ImageRemove(_ context.Context, ref string) error {
	if err := f.record("ImageRemove:" + ref); err != nil {
		return err
	}
	delete(f.Images, ref)
	return nil
}

func (f *Fake) ContainerInspect(_ context.Context, name string) (ContainerState, error) {
	if err := f.record("ContainerInspect:" + name); err != nil {
		return ContainerState{}, err
	}
	c, ok := f.Containers[name]
	if !ok {
		return ContainerState{Exists: false}, nil
	}
	return *c, nil
}

func (f *Fake) ContainerCreate(_ context.Context, spec ContainerSpec) error {
	if err := f.record("ContainerCreate:" + spec.Name); err != nil {
		return err
	}
	if _, exists := f.Containers[spec.Name]; exists {
		return fmt.Errorf("fake: container %s already exists", spec.Name)
	}
	f.Containers[spec.Name] = &ContainerState{
		Exists:  true,
		Running: spec.Detach, // create-and-run starts it
		Image:   spec.Image,
		ID:      "fake-" + spec.Name,
	}
	f.Specs[spec.Name] = spec
	return nil
}

func (f *Fake) ContainerStart(_ context.Context, name string) error {
	if err := f.record("ContainerStart:" + name); err != nil {
		return err
	}
	c, ok := f.Containers[name]
	if !ok {
		return fmt.Errorf("fake: container %s does not exist", name)
	}
	c.Running = true
	return nil
}

func (f *Fake) ContainerStop(_ context.Context, name string) error {
	if err := f.record("ContainerStop:" + name); err != nil {
		return err
	}
	c, ok := f.Containers[name]
	if !ok {
		return fmt.Errorf("fake: container %s does not exist", name)
	}
	c.Running = false
	return nil
}

func (f *Fake) ContainerRemove(_ context.Context, name string, _ bool) error {
	if err := f.record("ContainerRemove:" + name); err != nil {
		return err
	}
	delete(f.Containers, name)
	delete(f.Specs, name)
	return nil
}
