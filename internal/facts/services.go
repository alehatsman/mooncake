package facts

import (
	"os/exec"
	"strings"
)

// keyServices lists service names worth reporting when stopped on Linux.
var keyLinuxServices = []string{
	"sshd", "ssh", "docker", "containerd", "nginx", "apache2", "httpd",
	"mysql", "postgresql", "redis", "mongodb", "ollama",
}

// detectLinuxServices returns failed and noteworthy stopped services via systemctl.
func detectLinuxServices() (failed, stopped []string) {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return nil, nil
	}

	// Failed services
	out, err := probeOutput("systemctl", "list-units", "--state=failed", "--no-legend", "--no-pager")
	if err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) > 0 {
				name := strings.TrimSuffix(fields[0], ".service")
				failed = append(failed, name)
			}
		}
	}

	// Key stopped services
	for _, svc := range keyLinuxServices {
		// #nosec G204 -- svc is from a trusted static list of known service names
		out, err := probeOutput("systemctl", "is-active", "--quiet", svc)
		_ = out
		if err != nil {
			// Non-zero exit = not active; check if it exists at all
			// #nosec G204 -- svc is from a trusted static list of known service names
			out2, err2 := probeOutput("systemctl", "status", svc)
			_ = err2
			if strings.Contains(string(out2), "inactive") || strings.Contains(string(out2), "dead") {
				stopped = append(stopped, svc)
			}
		}
	}

	return failed, stopped
}

// detectDarwinServices returns failed and stopped services on macOS.
// On macOS this is lightweight: check a small set of known services.
func detectDarwinServices() (failed, stopped []string) {
	// macOS service detection via launchctl is complex and noisy — skip for v1
	return nil, nil
}
