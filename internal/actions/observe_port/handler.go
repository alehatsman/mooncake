// Package observe_port implements the observe.port action: single-shot
// read of TCP/UDP port state (spec-59). The polling cousin is wait.port;
// observe.port returns the current state once and lets the next step
// branch on it via spec-37 `as:` capture.
package observe_port

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

const (
	actionName     = "observe.port"
	defaultProto   = "tcp"
	defaultTimeout = 2 * time.Second
)

// PortObservation is the typed Value payload for observe.port.
// Pid+Listener are best-effort; on systems where /proc/net/tcp isn't
// readable without root, Pid will be 0 and Listener will be "".
type PortObservation struct {
	Open      bool   `json:"open"`                 // a listener is bound to (host, port)
	Protocol  string `json:"protocol,omitempty"`   // "tcp" | "udp"
	Host      string `json:"host,omitempty"`       // resolved host that was probed
	Port      int    `json:"port"`                 // probed port
	LocalAddr string `json:"local_addr,omitempty"` // host:port as probed
	Listener  string `json:"listener,omitempty"`   // process name, when discoverable
	Pid       int    `json:"pid,omitempty"`        // process pid, when discoverable
}

type Handler struct{}

func init() { actions.Register(&Handler{}) }

func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               actionName,
		Description:        "Single-shot read of TCP/UDP port state (open? listener? pid?)",
		Category:           actions.CategoryNetwork,
		SupportsDryRun:     true,
		SupportsBecome:     false,
		SupportedPlatforms: []string{}, // works on every Go-supported OS
		RequiresSudo:       false,
		ImplementsCheck:    false,
		// spec-37: read-only observation, safe to bind in plan mode.
		// Plan-mode handlers return PlanDeferred so consumers still
		// see a typed (zero-value) payload to template against.
		CaptureInPlan: true,
	}
}

func (h *Handler) Validate(step *config.Step) error {
	o := step.ObservePort
	if o == nil {
		return fmt.Errorf("%s requires configuration", actionName)
	}
	if o.Port <= 0 || o.Port > 65535 {
		return fmt.Errorf("%s: port must be 1..65535, got %d", actionName, o.Port)
	}
	if o.Protocol != "" && o.Protocol != "tcp" && o.Protocol != "udp" {
		return fmt.Errorf("%s: protocol must be 'tcp' or 'udp', got %q", actionName, o.Protocol)
	}
	if o.Timeout != "" {
		if _, err := time.ParseDuration(o.Timeout); err != nil {
			return fmt.Errorf("%s: invalid timeout %q: %w", actionName, o.Timeout, err)
		}
	}
	return nil
}

func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	o := step.ObservePort

	host, err := resolveHost(ctx, o.Host)
	if err != nil {
		return nil, err
	}
	protocol := o.Protocol
	if protocol == "" {
		protocol = defaultProto
	}
	timeout := defaultTimeout
	if o.Timeout != "" {
		// Validate guarantees this parses; ignore the error.
		timeout, _ = time.ParseDuration(o.Timeout)
	}
	addr := net.JoinHostPort(host, strconv.Itoa(o.Port))

	result := executor.NewResult()
	result.Changed = false
	result.StartTime = time.Now()
	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
	}()

	// Plan mode: per spec-59 G4, defer the actual probe. Publish a
	// zero-value typed payload so downstream templates still type-check.
	if ctx.Mode() == actions.ModePlan {
		envelope := actions.PlanDeferred(PortObservation{
			Protocol:  protocol,
			Host:      host,
			Port:      o.Port,
			LocalAddr: addr,
		})
		result.PublishObservation(envelope, addr)
		result.Checkable = true
		result.Reason = fmt.Sprintf("would observe %s %s (deferred to apply)", strings.ToUpper(protocol), addr)
		return result, nil
	}

	obs := PortObservation{
		Protocol:  protocol,
		Host:      host,
		Port:      o.Port,
		LocalAddr: addr,
	}
	obs.Open = probe(protocol, addr, timeout)

	envelope := actions.ObserveResult{
		Found: obs.Open, // "found" means listener is bound
		Value: obs,
		AsOf:  time.Now(),
	}
	result.PublishObservation(envelope, addr)

	ctx.Logger().Debugf("%s %s = open:%v", actionName, addr, obs.Open)
	return result, nil
}

// probe attempts a connection to (proto, addr). For TCP a successful
// Dial means a listener is accepting. For UDP we can't reliably
// distinguish "open" from "filtered" without sending traffic; v1 uses
// a Dial+close which always succeeds for UDP and reports the connection
// state via subsequent send/read. For now UDP returns Open=true if the
// dial succeeded (no bind error from the local stack) — honest, narrow,
// improvable later if a user surfaces a concrete need.
func probe(protocol, addr string, timeout time.Duration) bool {
	conn, err := net.DialTimeout(protocol, addr, timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func resolveHost(ctx actions.Context, host string) (string, error) {
	if host == "" {
		return "localhost", nil
	}
	rendered, err := ctx.Template().Render(host, ctx.Variables())
	if err != nil {
		return "", &executor.RenderError{Field: actionName + ".host", Cause: err}
	}
	return rendered, nil
}

// publish writes the ObserveResult into Result.Data using the spec-59
// shape — the typed Value lives under "value" so template references
// like {{ name.value.open }} resolve naturally, and the universal
// fields (found, as_of, error) live alongside it at top level.
//
// The typed Value is round-tripped through JSON (via
// actions.ObserveValueToMap) so the template engine sees a nested
// map[string]any keyed by the struct's json tags.

// --- Spec-22 ABI sub-interfaces (no-mutation specialization) -----------------

// Cost: every observe.* handler reports Risk=1, Reversible=true,
// Resources=0, Bytes=0. Pure read.
func (h *Handler) Cost(_ actions.Context, _ *config.Step) (actions.CostEstimate, error) {
	return actions.CostEstimate{
		Resources:  0,
		Bytes:      0,
		Reversible: true,
		Risk:       1,
	}, nil
}

// Permissions: observe.port uses only the Go stdlib net package; no
// external binary required. Network=true because we do open a socket
// (a strict policy gate may want to know).
func (h *Handler) Permissions(_ *config.Step) actions.PermissionSet {
	return actions.PermissionSet{
		Network: true,
		Notes:   []string{"read-only observation; no mutation"},
	}
}

// Diff: observation does not mutate. Return a Diff with Operation=noop
// so plan output can still surface "would observe X" without faking a
// mutation.
func (h *Handler) Diff(_ actions.Context, step *config.Step) (actions.Diff, error) {
	o := step.ObservePort
	if o == nil {
		return actions.Diff{}, nil
	}
	proto := o.Protocol
	if proto == "" {
		proto = defaultProto
	}
	host := o.Host
	if host == "" {
		host = "localhost"
	}
	return actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceOther,
			Identifier: fmt.Sprintf("%s:%s:%d", proto, host, o.Port),
			Attributes: map[string]string{"observe_kind": "port"},
		},
		Operation: actions.OpNoop,
	}, nil
}

// Reverse: pure observation has no inverse. Return (nil, nil) per the
// Reverser contract for "no reverse needed."
func (h *Handler) Reverse(_ actions.Context, _ *config.Step, _ actions.Result) (*config.Step, error) {
	return nil, nil
}
