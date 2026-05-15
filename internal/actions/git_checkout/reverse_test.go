package git_checkout //nolint:revive // package name follows action convention

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

func TestReverse_BuildsCheckoutToPriorSHA(t *testing.T) {
	h := Handler{}
	result := executor.NewResult()
	result.ReverseData = &GitCheckoutReverseInfo{
		Dest:     "/srv/app",
		PriorSHA: "abc1234deadbeef",
	}
	step := &config.Step{GitCheckout: &config.GitCheckout{Dest: "/srv/app", Ref: "v2"}}

	rev, err := h.Reverse(nil, step, result)
	if err != nil {
		t.Fatalf("Reverse: %v", err)
	}
	if rev == nil || rev.GitCheckout == nil {
		t.Fatal("Reverse must return a git.checkout step")
	}
	if rev.GitCheckout.Dest != "/srv/app" {
		t.Errorf("rev.Dest = %s, want /srv/app", rev.GitCheckout.Dest)
	}
	if rev.GitCheckout.Ref != "abc1234deadbeef" {
		t.Errorf("rev.Ref = %s, want abc1234deadbeef (the prior sha)", rev.GitCheckout.Ref)
	}
	if !rev.GitCheckout.Force {
		t.Error("rev.Force must be true on reverse (working tree may have drifted)")
	}
}

func TestReverse_NoReverseDataIsNoop(t *testing.T) {
	h := Handler{}
	result := executor.NewResult()
	// No ReverseData → apply was a noop (HEAD already at requested
	// ref). Reverse must return (nil, nil) per the contract.
	step, err := h.Reverse(nil, &config.Step{GitCheckout: &config.GitCheckout{Dest: "/srv/app", Ref: "v2"}}, result)
	if err != nil {
		t.Fatalf("Reverse on no-capture must not error; got: %v", err)
	}
	if step != nil {
		t.Errorf("Reverse on no-capture must return nil step; got %+v", step)
	}
}

func TestReverse_NilStep(t *testing.T) {
	h := Handler{}
	_, err := h.Reverse(nil, nil, nil)
	if err == nil {
		t.Fatal("Reverse must error on nil step")
	}
}

func TestReverse_WrongResultType(t *testing.T) {
	h := Handler{}
	_, err := h.Reverse(nil, &config.Step{GitCheckout: &config.GitCheckout{Dest: "/srv/app", Ref: "v2"}}, nil)
	if err == nil {
		t.Fatal("Reverse must error when result is nil/wrong type")
	}
}

func TestReverse_WrongReverseDataType(t *testing.T) {
	h := Handler{}
	result := executor.NewResult()
	result.ReverseData = "not the right type"
	_, err := h.Reverse(nil, &config.Step{GitCheckout: &config.GitCheckout{Dest: "/srv/app", Ref: "v2"}}, result)
	if err == nil {
		t.Fatal("Reverse must error when ReverseData has wrong type")
	}
}

func TestReverse_IncompleteReverseData(t *testing.T) {
	h := Handler{}
	result := executor.NewResult()
	result.ReverseData = &GitCheckoutReverseInfo{Dest: "/srv/app"} // missing PriorSHA
	_, err := h.Reverse(nil, &config.Step{GitCheckout: &config.GitCheckout{Dest: "/srv/app", Ref: "v2"}}, result)
	if err == nil {
		t.Fatal("Reverse must error on incomplete ReverseData")
	}
}

func TestGitCheckoutHandler_ImplementsReverser(t *testing.T) {
	var _ actions.Reverser = (*Handler)(nil)
}
