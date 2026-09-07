package executor

import (
	"path/filepath"
	"testing"

	"github.com/alehatsman/mooncake/internal/plan"
)

// RootDir anchors module-lockfile discovery. An empty result means "no
// lockfile, verify nothing", which is the correct behavior for inline plans —
// but a real playbook must yield its own directory.
func TestPlanRootDir(t *testing.T) {
	tests := []struct {
		name string
		p    *plan.Plan
		want string
	}{
		{"nil plan", nil, ""},
		{"no root file", &plan.Plan{}, ""},
		{
			"playbook path yields its dir",
			&plan.Plan{RootFile: filepath.Join("/srv", "infra", "mooncake.yml")},
			filepath.Join("/srv", "infra"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := planRootDir(tt.p); got != tt.want {
				t.Errorf("planRootDir() = %q, want %q", got, tt.want)
			}
		})
	}
}
