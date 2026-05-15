package git_config

import (
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

func TestReverse_RefusesPendingCapture(t *testing.T) {
	h := Handler{}
	step, err := h.Reverse(nil, &config.Step{GitConfig: &config.GitConfig{Scope: "global", Set: map[string]string{"a": "b"}}}, nil)
	if step != nil {
		t.Errorf("Reverse must return nil step; got %+v", step)
	}
	if err == nil {
		t.Fatal("Reverse must return a refusal error")
	}
	if !strings.Contains(err.Error(), "not yet implemented") {
		t.Errorf("error message must mention 'not yet implemented'; got: %s", err)
	}
}

func TestReverse_NilStep(t *testing.T) {
	h := Handler{}
	_, err := h.Reverse(nil, nil, nil)
	if err == nil {
		t.Fatal("Reverse must error on nil step")
	}
}

func TestGitConfigHandler_ImplementsReverser(t *testing.T) {
	var _ actions.Reverser = (*Handler)(nil)
}
