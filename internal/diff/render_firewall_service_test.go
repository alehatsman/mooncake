package diff

import (
	"bytes"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
)

// ------ helpers --------------------------------------------------------------

func firewallDiff(op actions.Operation, payload *actions.FirewallDiff) *actions.Diff {
	return &actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceOther,
			Identifier: "ufw:rules",
			Attributes: map[string]string{"kind": "os.firewall", "backend": payload.Backend},
		},
		Operation: op,
		After:     payload,
	}
}

func serviceDiff(op actions.Operation, payload *actions.ServiceDiff) *actions.Diff {
	return &actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceService,
			Identifier: payload.Name,
		},
		Operation: op,
		After:     payload,
	}
}

func boolp(b bool) *bool { return &b }

// ------ firewall -------------------------------------------------------------

func TestLookup_FirewallDiff_AddSingleRule(t *testing.T) {
	r := Lookup(firewallDiff(actions.OpCreate, &actions.FirewallDiff{
		Backend:   "ufw",
		State:     "present",
		RuleCount: 1,
	}))
	if r == nil {
		t.Fatal("Lookup returned nil")
	}
	if r.Kind() != "firewall" {
		t.Errorf("Kind = %q, want firewall", r.Kind())
	}
	var buf bytes.Buffer
	if err := r.Render(&buf, FormatText); err != nil {
		t.Fatalf("Render: %v", err)
	}
	want := "  firewall add ufw: 1 rule\n"
	if got := buf.String(); got != want {
		t.Errorf("Render mismatch:\n got: %q\nwant: %q", got, want)
	}
}

func TestLookup_FirewallDiff_AddMultiRule(t *testing.T) {
	r := Lookup(firewallDiff(actions.OpCreate, &actions.FirewallDiff{
		Backend:   "ufw",
		State:     "present",
		RuleCount: 3,
	}))
	var buf bytes.Buffer
	if err := r.Render(&buf, FormatText); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got := buf.String(); got != "  firewall add ufw: 3 rules\n" {
		t.Errorf("Render = %q", got)
	}
}

func TestLookup_FirewallDiff_Remove(t *testing.T) {
	r := Lookup(firewallDiff(actions.OpDelete, &actions.FirewallDiff{
		Backend:   "ufw",
		State:     "absent",
		RuleCount: 2,
	}))
	var buf bytes.Buffer
	if err := r.Render(&buf, FormatText); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got := buf.String(); got != "  firewall remove ufw: 2 rules\n" {
		t.Errorf("Render = %q", got)
	}
}

func TestLookup_FirewallDiff_AutoBackend(t *testing.T) {
	// When the step doesn't pin a backend the handler defaults to
	// "auto". The renderer surfaces it as-is.
	d := firewallDiff(actions.OpCreate, &actions.FirewallDiff{Backend: "auto", RuleCount: 1})
	r := Lookup(d)
	var buf bytes.Buffer
	_ = r.Render(&buf, FormatText)
	if !strings.Contains(buf.String(), "auto") {
		t.Errorf("expected auto backend in output: %q", buf.String())
	}
}

func TestLookup_FirewallDiff_JSONDecoded(t *testing.T) {
	d := &actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceOther,
			Attributes: map[string]string{"kind": "os.firewall"},
		},
		Operation: actions.OpCreate,
		After: map[string]interface{}{
			"backend":    "ufw",
			"state":      "present",
			"rule_count": float64(1),
		},
	}
	r := Lookup(d)
	if r == nil {
		t.Fatal("Lookup returned nil for json-decoded firewall diff")
	}
	var buf bytes.Buffer
	if err := r.Render(&buf, FormatText); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got := buf.String(); got != "  firewall add ufw: 1 rule\n" {
		t.Errorf("Render = %q", got)
	}
}

func TestLookup_FirewallDiff_WrongAttribute(t *testing.T) {
	d := firewallDiff(actions.OpCreate, &actions.FirewallDiff{Backend: "ufw", RuleCount: 1})
	d.Resource.Attributes["kind"] = "os.user"
	r := Lookup(d)
	if r != nil && r.Kind() == "firewall" {
		t.Errorf("firewall renderer matched on kind=os.user: %+v", r)
	}
}

func TestLookup_FirewallDiff_EmptyAfter(t *testing.T) {
	d := &actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceOther,
			Attributes: map[string]string{"kind": "os.firewall"},
		},
		After: map[string]interface{}{},
	}
	if r := Lookup(d); r != nil {
		t.Errorf("Lookup matched on empty After: %+v", r)
	}
}

// ------ service (os.systemd) -------------------------------------------------

func TestLookup_ServiceDiff_InstallWithLifecycle(t *testing.T) {
	r := Lookup(serviceDiff(actions.OpCreate, &actions.ServiceDiff{
		Name:     "myapp.service",
		State:    "present",
		Sections: []string{"Service", "Install"},
		Enabled:  boolp(true),
		Started:  boolp(true),
	}))
	if r == nil {
		t.Fatal("Lookup returned nil")
	}
	if r.Kind() != "service" {
		t.Errorf("Kind = %q, want service", r.Kind())
	}
	var buf bytes.Buffer
	if err := r.Render(&buf, FormatText); err != nil {
		t.Fatalf("Render: %v", err)
	}
	want := "  service install myapp.service (Service Install) enabled started\n"
	if got := buf.String(); got != want {
		t.Errorf("Render mismatch:\n got: %q\nwant: %q", got, want)
	}
}

func TestLookup_ServiceDiff_RemoveBare(t *testing.T) {
	r := Lookup(serviceDiff(actions.OpDelete, &actions.ServiceDiff{
		Name:  "old.service",
		State: "absent",
	}))
	var buf bytes.Buffer
	if err := r.Render(&buf, FormatText); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got := buf.String(); got != "  service remove old.service\n" {
		t.Errorf("Render = %q", got)
	}
}

func TestLookup_ServiceDiff_DisabledStopped(t *testing.T) {
	r := Lookup(serviceDiff(actions.OpUpdate, &actions.ServiceDiff{
		Name:    "x.service",
		Path:    "/etc/systemd/system",
		Enabled: boolp(false),
		Started: boolp(false),
	}))
	var buf bytes.Buffer
	if err := r.Render(&buf, FormatText); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got := buf.String(); got != "  service update x.service disabled stopped\n" {
		t.Errorf("Render = %q", got)
	}
}

func TestLookup_ServiceDiff_JSONDecoded(t *testing.T) {
	// JSON-decoded path must carry a distinctive ServiceDiff field
	// (sections / started / path) so the renderer doesn't steal the
	// dispatch from a future legacy-os.service JSON shape.
	d := &actions.Diff{
		Resource:  actions.ResourceRef{Kind: actions.ResourceService, Identifier: "x.service"},
		Operation: actions.OpCreate,
		After: map[string]interface{}{
			"name":     "x.service",
			"state":    "present",
			"sections": []interface{}{"Service"},
			"enabled":  true,
		},
	}
	r := Lookup(d)
	if r == nil {
		t.Fatal("Lookup returned nil for json-decoded service diff")
	}
	var buf bytes.Buffer
	if err := r.Render(&buf, FormatText); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got := buf.String(); got != "  service install x.service (Service) enabled\n" {
		t.Errorf("Render = %q", got)
	}
}

// JSON-decoded map without any of {sections, started, path} should
// NOT trigger the service renderer — that guard keeps it from
// stealing dispatch from the legacy os.service shape.
func TestLookup_ServiceDiff_JSONDecoded_NoDistinctiveFields(t *testing.T) {
	d := &actions.Diff{
		Resource:  actions.ResourceRef{Kind: actions.ResourceService, Identifier: "x"},
		Operation: actions.OpUpdate,
		After: map[string]interface{}{
			"name":  "x",
			"state": "running",
		},
	}
	if r := Lookup(d); r != nil && r.Kind() == "service" {
		t.Errorf("service renderer matched on legacy-shape JSON map: %+v", r)
	}
}

func TestLookup_ServiceDiff_NilAfter(t *testing.T) {
	d := &actions.Diff{
		Resource:  actions.ResourceRef{Kind: actions.ResourceService},
		Operation: actions.OpCreate,
		After:     nil,
	}
	if r := Lookup(d); r != nil {
		t.Errorf("Lookup matched on nil After: %+v", r)
	}
}

func TestServiceOperationVerb(t *testing.T) {
	cases := []struct {
		op   actions.Operation
		want string
	}{
		{actions.OpCreate, "install"},
		{actions.OpUpdate, "update"},
		{actions.OpDelete, "remove"},
		{actions.OpNoop, "noop"},
		{actions.Operation("weird"), "change"},
	}
	for _, tc := range cases {
		if got := serviceOperationVerb(tc.op); got != tc.want {
			t.Errorf("serviceOperationVerb(%q) = %q, want %q", tc.op, got, tc.want)
		}
	}
}

func TestFirewallOperationVerb(t *testing.T) {
	cases := []struct {
		op   actions.Operation
		want string
	}{
		{actions.OpCreate, "add"},
		{actions.OpUpdate, "update"},
		{actions.OpDelete, "remove"},
		{actions.OpNoop, "noop"},
		{actions.Operation("weird"), "change"},
	}
	for _, tc := range cases {
		if got := firewallOperationVerb(tc.op); got != tc.want {
			t.Errorf("firewallOperationVerb(%q) = %q, want %q", tc.op, got, tc.want)
		}
	}
}
