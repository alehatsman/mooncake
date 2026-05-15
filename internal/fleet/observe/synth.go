// Package observe implements `mooncake fleet observe` (spec-64).
// Synthesizes a one-step plan with a spec-59 observe.* action,
// uploads to each peer's synced root, submits a run, and captures
// the typed observation envelope (found/value/as_of/error) from the
// step.completed event for tabular or JSON rendering.
//
// Mirrors internal/fleet/exec — same plan-upload + submit + stream
// machinery, different per-step capture (Result map instead of
// stdout/stderr).
package observe

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/alehatsman/mooncake/internal/config"
)

// SynthOptions describes the observation to dispatch. The Kind field
// selects which observe.* handler runs on the peer; per-kind fields
// (Port, ProcessName, URL, ...) carry the handler's parameters.
//
// Only one (Kind, fields-for-that-kind) combination is meaningful per
// call; other fields are ignored. Validation enforces required fields
// per kind.
type SynthOptions struct {
	// Kind is one of "port", "process", "http", "service", "cpu",
	// "memory", "disk", "gpu" — matches the spec-59 / spec-60 / spec-62
	// handlers shipped on the peer.
	Kind string

	// Common
	Host string

	// observe.port
	Port     int
	Protocol string
	Timeout  string

	// observe.process
	ProcessName    string
	ProcessPattern string

	// observe.http
	URL            string
	Method         string
	ExpectStatus   int
	CaptureHeaders []string
	SkipTLSVerify  bool

	// observe.service
	ServiceName    string
	ServiceManager string

	// observe.disk
	DiskPath string

	// observe.gpu
	GPUIndex *int
}

// Synthesize emits the YAML bytes of a one-step plan that runs the
// requested observe.* action on a peer. The synthesized step is
// `as: result` so consumers can rely on a stable capture name when
// reading the run's Step.Completed event.
func Synthesize(opts SynthOptions) ([]byte, error) {
	step := config.Step{
		Name: fmt.Sprintf("observe.%s", opts.Kind),
		As:   "result",
	}
	switch opts.Kind {
	case "port":
		if opts.Port <= 0 || opts.Port > 65535 {
			return nil, fmt.Errorf("observe: port must be 1..65535, got %d", opts.Port)
		}
		step.ObservePort = &config.ObservePort{
			Host:     opts.Host,
			Port:     opts.Port,
			Protocol: opts.Protocol,
			Timeout:  opts.Timeout,
		}
	case "process":
		if opts.ProcessName == "" && opts.ProcessPattern == "" {
			return nil, fmt.Errorf("observe.process: name or pattern is required")
		}
		step.ObserveProcess = &config.ObserveProcess{
			Name:    opts.ProcessName,
			Pattern: opts.ProcessPattern,
		}
	case "http":
		if opts.URL == "" {
			return nil, fmt.Errorf("observe.http: url is required")
		}
		step.ObserveHTTP = &config.ObserveHTTP{
			URL:            opts.URL,
			Method:         opts.Method,
			Timeout:        opts.Timeout,
			ExpectStatus:   opts.ExpectStatus,
			CaptureHeaders: opts.CaptureHeaders,
			SkipTLSVerify:  opts.SkipTLSVerify,
		}
	case "service":
		if opts.ServiceName == "" {
			return nil, fmt.Errorf("observe.service: name is required")
		}
		step.ObserveService = &config.ObserveService{
			Name:    opts.ServiceName,
			Manager: opts.ServiceManager,
		}
	case "cpu":
		step.ObserveCPU = &config.ObserveCPU{}
	case "memory":
		step.ObserveMemory = &config.ObserveMemory{}
	case "disk":
		step.ObserveDisk = &config.ObserveDisk{Path: opts.DiskPath}
	case "gpu":
		step.ObserveGPU = &config.ObserveGPU{Index: opts.GPUIndex}
	default:
		return nil, fmt.Errorf("observe: unknown kind %q (supported: port, process, http, service, cpu, memory, disk, gpu)", opts.Kind)
	}

	root := struct {
		Version string        `yaml:"version"`
		Steps   []config.Step `yaml:"steps"`
	}{
		Version: "1.0",
		Steps:   []config.Step{step},
	}
	return yaml.Marshal(&root)
}

// ParsePortShorthand accepts ":80", "80", "tcp:80", "udp:53" and
// returns (port, protocol). Returns 0/"" on parse failure; caller
// decides whether to error.
func ParsePortShorthand(s string) (int, string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, ""
	}
	proto := ""
	if strings.HasPrefix(s, "tcp:") || strings.HasPrefix(s, "udp:") {
		proto = s[:3]
		s = s[4:]
	} else if strings.HasPrefix(s, ":") {
		s = s[1:]
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 || n > 65535 {
		return 0, ""
	}
	return n, proto
}
