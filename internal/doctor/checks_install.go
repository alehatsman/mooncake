package doctor

import (
	"fmt"
	"os"
	"runtime"
)

// checkBinary reports the mooncake binary path and version. Version is
// resolved from the runtime "version" var via a package-level hook the
// CLI installs at startup (so we don't pull cmd-layer state into a
// library package).
type checkBinary struct{}

// BinaryVersion is set by cmd/doctor.go from cmd/mooncake.go's `version`.
// Default keeps tests and direct library callers from panicking.
var BinaryVersion = "unknown"

func (checkBinary) Section() string { return "install" }
func (checkBinary) Name() string    { return "binary" }
func (checkBinary) Run(_ Context) Result {
	r := Result{Section: "install", Name: "binary"}
	exe, err := os.Executable()
	if err != nil {
		r.Status = StatusError
		r.Message = "cannot resolve mooncake binary path"
		r.Fix = "rerun mooncake from a directory where it has read access to its own executable"
		return r
	}
	r.Status = StatusOK
	r.Message = fmt.Sprintf("mooncake %s", BinaryVersion)
	r.Detail = exe
	return r
}

type checkGoRuntime struct{}

func (checkGoRuntime) Section() string { return "install" }
func (checkGoRuntime) Name() string    { return "go-runtime" }
func (checkGoRuntime) Run(_ Context) Result {
	return Result{
		Section: "install", Name: "go-runtime",
		Status:  StatusInfo,
		Message: fmt.Sprintf("Go runtime: %s %s/%s", runtime.Version(), runtime.GOOS, runtime.GOARCH),
	}
}
