package executor_test

import (
	"encoding/json"
	"testing"

	"github.com/alehatsman/mooncake/internal/executor"

	// Side-effect imports: each handler package registers its
	// ReverseData type at init() via executor.RegisterReverseDataType.
	// Pull all 16 in so the round-trip table below can exercise every
	// concrete payload type without per-handler test scaffolding.
	"github.com/alehatsman/mooncake/internal/actions/file"
	_ "github.com/alehatsman/mooncake/internal/actions/file"
	"github.com/alehatsman/mooncake/internal/actions/git_checkout"
	_ "github.com/alehatsman/mooncake/internal/actions/git_checkout"
	"github.com/alehatsman/mooncake/internal/actions/git_config"
	_ "github.com/alehatsman/mooncake/internal/actions/git_config"
	_ "github.com/alehatsman/mooncake/internal/actions/os_cron"
	_ "github.com/alehatsman/mooncake/internal/actions/os_firewall"
	_ "github.com/alehatsman/mooncake/internal/actions/os_group"
	_ "github.com/alehatsman/mooncake/internal/actions/os_mount"
	_ "github.com/alehatsman/mooncake/internal/actions/os_ssh_key"
	_ "github.com/alehatsman/mooncake/internal/actions/os_sysctl"
	_ "github.com/alehatsman/mooncake/internal/actions/os_systemd"
	_ "github.com/alehatsman/mooncake/internal/actions/os_user"
	pkghandler "github.com/alehatsman/mooncake/internal/actions/package"
	"github.com/alehatsman/mooncake/internal/actions/pkg_hold"
	"github.com/alehatsman/mooncake/internal/actions/pkg_repo"
	"github.com/alehatsman/mooncake/internal/actions/service"
)

// TestResult_ReverseData_WireRoundTrip — R2.1c phase 2: every
// registered ReverseInfo type must round-trip cleanly through
// json.Marshal → json.Unmarshal so per-peer Reverse() composes
// after a fleet wire hop. Pre-fix the field was `json:"-"` and the
// post-unmarshal type assertion in every handler's Reverse failed
// (ReverseData=nil → refusal).
//
// The assertions are deliberately structural: discriminator name
// matches, the decoded payload is a *T pointer (not a generic
// map[string]any), and the type-assertion the production Reverse
// code uses still succeeds. Field-by-field content checks live in
// the per-handler tests — this test guards the envelope contract.
func TestResult_ReverseData_WireRoundTrip(t *testing.T) {
	cases := []struct {
		name    string
		payload any
		assert  func(t *testing.T, got any)
	}{
		{
			name:    "FileReverseInfo",
			payload: &file.FileReverseInfo{State: "file", Existed: true, Kind: "file", Mode: 0o644, Content: []byte("hello")},
			assert: func(t *testing.T, got any) {
				v, ok := got.(*file.FileReverseInfo)
				if !ok {
					t.Fatalf("got %T, want *file.FileReverseInfo", got)
				}
				if v.State != "file" || !v.Existed || string(v.Content) != "hello" {
					t.Errorf("payload not restored: %+v", v)
				}
			},
		},
		{
			name:    "GitCheckoutReverseInfo",
			payload: &git_checkout.GitCheckoutReverseInfo{},
			assert: func(t *testing.T, got any) {
				if _, ok := got.(*git_checkout.GitCheckoutReverseInfo); !ok {
					t.Errorf("got %T, want *git_checkout.GitCheckoutReverseInfo", got)
				}
			},
		},
		{
			name:    "GitConfigReverseInfo",
			payload: &git_config.GitConfigReverseInfo{},
			assert: func(t *testing.T, got any) {
				if _, ok := got.(*git_config.GitConfigReverseInfo); !ok {
					t.Errorf("got %T, want *git_config.GitConfigReverseInfo", got)
				}
			},
		},
		{
			name:    "PkgReverseInfo",
			payload: &pkghandler.PkgReverseInfo{Manager: "apt", AppliedState: "present", Mutated: []string{"jq"}},
			assert: func(t *testing.T, got any) {
				v, ok := got.(*pkghandler.PkgReverseInfo)
				if !ok {
					t.Fatalf("got %T, want *pkghandler.PkgReverseInfo", got)
				}
				if v.Manager != "apt" || len(v.Mutated) != 1 || v.Mutated[0] != "jq" {
					t.Errorf("payload not restored: %+v", v)
				}
			},
		},
		{
			name:    "PkgHoldReverseInfo",
			payload: &pkg_hold.PkgHoldReverseInfo{},
			assert: func(t *testing.T, got any) {
				if _, ok := got.(*pkg_hold.PkgHoldReverseInfo); !ok {
					t.Errorf("got %T, want *pkg_hold.PkgHoldReverseInfo", got)
				}
			},
		},
		{
			name:    "PkgRepoReverseInfo",
			payload: &pkg_repo.PkgRepoReverseInfo{Name: "nodesource", PriorExisted: true},
			assert: func(t *testing.T, got any) {
				v, ok := got.(*pkg_repo.PkgRepoReverseInfo)
				if !ok {
					t.Fatalf("got %T, want *pkg_repo.PkgRepoReverseInfo", got)
				}
				if v.Name != "nodesource" || !v.PriorExisted {
					t.Errorf("payload not restored: %+v", v)
				}
			},
		},
		{
			name:    "PkgRepoBrewReverseInfo",
			payload: &pkg_repo.PkgRepoBrewReverseInfo{Name: "cask-fonts", Tap: "homebrew/cask-fonts", WasTapped: false},
			assert: func(t *testing.T, got any) {
				v, ok := got.(*pkg_repo.PkgRepoBrewReverseInfo)
				if !ok {
					t.Fatalf("got %T, want *pkg_repo.PkgRepoBrewReverseInfo", got)
				}
				if v.Tap != "homebrew/cask-fonts" || v.WasTapped {
					t.Errorf("payload not restored: %+v", v)
				}
			},
		},
		{
			name:    "OsServiceReverseInfo",
			payload: &service.OsServiceReverseInfo{},
			assert: func(t *testing.T, got any) {
				if _, ok := got.(*service.OsServiceReverseInfo); !ok {
					t.Errorf("got %T, want *service.OsServiceReverseInfo", got)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := executor.NewResult()
			src.Changed = true
			src.Reason = "test"
			src.ReverseData = tc.payload

			raw, err := json.Marshal(src)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}

			// Envelope sanity: the discriminator name appears in the
			// wire bytes, so a corrupted MarshalJSON that drops the
			// type field fails loudly here rather than via "unknown
			// type" silent-skip on decode.
			if want := `"type":"` + tc.name + `"`; !contains(raw, []byte(want)) {
				t.Errorf("encoded JSON missing discriminator %q:\n%s", want, raw)
			}

			dst := executor.NewResult()
			if err := json.Unmarshal(raw, dst); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			if dst.ReverseData == nil {
				t.Fatal("ReverseData lost across the wire (envelope decoded as nil)")
			}
			tc.assert(t, dst.ReverseData)
		})
	}
}

// TestResult_ReverseData_NilSkipsField — Result without ReverseData
// must encode the pre-phase-2 shape (no `reverse_data` key) so a
// pre-phase-2 client decoding a new daemon's response doesn't choke
// on an unexpected envelope. Symmetric forward-compat to the
// unknown-discriminator-skip path on the decode side.
func TestResult_ReverseData_NilSkipsField(t *testing.T) {
	src := executor.NewResult()
	src.Changed = true

	raw, err := json.Marshal(src)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if contains(raw, []byte(`"reverse_data"`)) {
		t.Errorf("nil ReverseData produced reverse_data key:\n%s", raw)
	}

	dst := executor.NewResult()
	if err := json.Unmarshal(raw, dst); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if dst.ReverseData != nil {
		t.Errorf("nil round-trip produced ReverseData=%v", dst.ReverseData)
	}
}

// TestResult_ReverseData_UnknownTypeIsForwardCompat — a newer daemon
// could send back a discriminator this binary doesn't have a
// factory for (plugin handler, future version). The unmarshal must
// keep ReverseData nil rather than error — the existing handler-side
// "no ReverseData captured" refusal already does the right thing on
// nil. Spec: forward-compat for mixed-version fleets.
func TestResult_ReverseData_UnknownTypeIsForwardCompat(t *testing.T) {
	wire := []byte(`{"stdout":"","stderr":"","rc":0,"failed":false,"changed":false,"skipped":false,"reverse_data":{"type":"FutureReverseInfo","data":{"foo":"bar"}}}`)
	dst := executor.NewResult()
	if err := json.Unmarshal(wire, dst); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if dst.ReverseData != nil {
		t.Errorf("unknown discriminator decoded to %T; want nil (forward-compat skip)", dst.ReverseData)
	}
}

// TestResult_OtherFieldsRoundTrip — guard against the MarshalJSON
// alias trick accidentally dropping a non-ReverseData field. If
// someone adds a future field to Result, this test catches the
// missed wire registration via the embedded alias.
func TestResult_OtherFieldsRoundTrip(t *testing.T) {
	src := executor.NewResult()
	src.Stdout = "out"
	src.Stderr = "err"
	src.Rc = 7
	src.Failed = true
	src.Changed = true
	src.WouldChange = true
	src.Reason = "because"
	src.Checkable = true
	src.Skipped = false
	src.Data = map[string]any{"k": "v"}

	raw, err := json.Marshal(src)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	dst := executor.NewResult()
	if err := json.Unmarshal(raw, dst); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if dst.Stdout != "out" || dst.Stderr != "err" || dst.Rc != 7 || !dst.Failed ||
		!dst.Changed || !dst.WouldChange || dst.Reason != "because" || !dst.Checkable {
		t.Errorf("scalar field round-trip lost a value: %+v", dst)
	}
	if v, _ := dst.Data["k"]; v != "v" {
		t.Errorf("Data round-trip lost: %v", dst.Data)
	}
}

func contains(haystack, needle []byte) bool {
	if len(needle) == 0 {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		match := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
