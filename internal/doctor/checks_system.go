package doctor

import (
	"fmt"

	"github.com/alehatsman/mooncake/internal/facts"
)

// checkFacts confirms facts collection works and surfaces a one-line
// summary of the detected host. facts.Collect is cached (sync.Once) so
// this call is a hash-lookup after the first invocation.
type checkFacts struct{}

func (checkFacts) Section() string { return "system" }
func (checkFacts) Name() string    { return "facts" }
func (checkFacts) Run(_ Context) Result {
	r := Result{Section: "system", Name: "facts"}
	f := facts.Collect()
	if f == nil {
		r.Status = StatusError
		r.Message = "facts collection returned nil"
		r.Fix = "report a bug: https://github.com/alehatsman/mooncake/issues"
		return r
	}
	r.Status = StatusOK
	r.Message = fmt.Sprintf("os=%s arch=%s distribution=%s package_manager=%s",
		valueOr(f.OS, "?"), valueOr(f.Arch, "?"),
		valueOr(f.Distribution, "-"), valueOr(f.PackageManager, "-"))
	r.Detail = "Use `mooncake facts` for the full list"
	return r
}

func valueOr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
