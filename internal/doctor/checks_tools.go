package doctor

import (
	"os/exec"
	"runtime"
)

// checkTool resolves an external CLI on PATH and reports whether it's
// available. Missing tools degrade mooncake features rather than break
// it, so the worst case is StatusWarning. UsedBy populates the JSON
// payload so consumers know which mooncake feature the tool affects.
type checkTool struct {
	name     string
	usedBy   []string
	unixOnly bool // skip the check on Windows when true
}

func (c checkTool) Section() string { return "tools" }
func (c checkTool) Name() string    { return c.name }

func (c checkTool) Run(_ Context) Result {
	r := Result{Section: "tools", Name: c.name, UsedBy: c.usedBy}
	if c.unixOnly && runtime.GOOS == "windows" {
		r.Status = StatusInfo
		r.Message = c.name + " not applicable on Windows"
		return r
	}
	path, err := exec.LookPath(c.name)
	if err != nil {
		r.Status = StatusWarning
		r.Message = c.name + " not on PATH"
		r.Fix = installHint(c.name)
		return r
	}
	r.Status = StatusOK
	r.Message = c.name + " present"
	r.Detail = path
	return r
}

// installHint maps a tool name to a one-line "how do I get this" string.
// Generic enough to avoid going stale; users who want exact instructions
// can search.
func installHint(tool string) string {
	switch tool {
	case "fzf":
		return "install fzf — https://github.com/junegunn/fzf"
	case "git":
		return "install git — https://git-scm.com/downloads"
	case "sudo":
		return "install sudo from your distribution's package manager"
	}
	return "install " + tool
}
