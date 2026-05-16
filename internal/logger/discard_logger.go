package logger

// DiscardLogger implements Logger by ignoring every call.
//
// Use it on a production code path that needs to satisfy the Logger
// interface but has no place to surface the output. The MCP server's
// best-effort plan-inspection calls (internal/mcp/tools.go) are the
// motivating case: InspectPlan logs are noise to the LLM client and
// dropping them avoids reaching for NewTestLogger (which retains
// every entry in a Logs slice — see F022).
type DiscardLogger struct{}

// NewDiscardLogger returns a Logger that drops every entry.
func NewDiscardLogger() *DiscardLogger { return &DiscardLogger{} }

func (DiscardLogger) Infof(string, ...interface{})  {}
func (DiscardLogger) Debugf(string, ...interface{}) {}
func (DiscardLogger) Errorf(string, ...interface{}) {}
func (DiscardLogger) Codef(string, ...interface{})  {}
func (DiscardLogger) Textf(string, ...interface{})  {}
func (DiscardLogger) Mooncake()                     {}
func (DiscardLogger) SetLogLevel(int)               {}
func (DiscardLogger) SetLogLevelStr(string) error   { return nil }
func (d DiscardLogger) WithPadLevel(int) Logger     { return d }
func (DiscardLogger) LogStep(StepInfo)              {}
func (DiscardLogger) Complete(ExecutionStats)       {}
func (DiscardLogger) SetRedactor(Redactor)          {}
