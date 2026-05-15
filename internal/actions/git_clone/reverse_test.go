package git_clone

import (
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

func TestReverse_RefusesByDesign(t *testing.T) {
	h := Handler{}
	step, err := h.Reverse(nil, &config.Step{GitClone: &config.GitClone{Repo: "x", Dest: "/tmp/x"}}, nil)
	if step != nil {
		t.Errorf("Reverse must return nil step; got %+v", step)
	}
	if err == nil {
		t.Fatal("Reverse must return a refusal error")
	}
	if !strings.Contains(err.Error(), "irreversible") {
		t.Errorf("error message must mention 'irreversible'; got: %s", err)
	}
}

func TestReverse_NilStep(t *testing.T) {
	h := Handler{}
	_, err := h.Reverse(nil, nil, nil)
	if err == nil {
		t.Fatal("Reverse must error on nil step")
	}
}

func TestGitCloneHandler_ImplementsReverser(t *testing.T) {
	var _ actions.Reverser = (*Handler)(nil)
}
