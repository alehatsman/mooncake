package agentd

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
)

func TestSelfMAC_ReturnsSomethingOrConsistent404(t *testing.T) {
	_, client, stop := startTestServer(t)
	defer stop()

	resp, err := client.Get("http://unix/v1/self/mac")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	// On hosts with no usable non-loopback NIC (rare; some CI
	// sandboxes), the handler is allowed to 404 with a no_mac code.
	// Either outcome is acceptable; what we verify is the shape.
	switch resp.StatusCode {
	case http.StatusOK:
		var out macResponse
		if err := json.Unmarshal(body, &out); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if out.MAC == "" {
			t.Errorf("MAC is empty in 200 response")
		}
		// Returned MAC must lowercase and parseable.
		if out.MAC != strings.ToLower(out.MAC) {
			t.Errorf("MAC %q not lowercase", out.MAC)
		}
		if _, err := net.ParseMAC(out.MAC); err != nil {
			t.Errorf("returned MAC %q not parseable: %v", out.MAC, err)
		}
		if out.Interface == "" {
			t.Errorf("interface name is empty in 200 response")
		}
	case http.StatusNotFound:
		if !strings.Contains(string(body), "no_mac") {
			t.Errorf("404 body lacks no_mac code: %s", body)
		}
	default:
		t.Errorf("status = %d (body: %s), want 200 or 404", resp.StatusCode, body)
	}
}

func TestPickPeerMAC_FallsBackToInterfaceWalk(t *testing.T) {
	// Build a request with no LocalAddrContextKey. The function must
	// still find a MAC by walking interfaces (unless this CI box has
	// truly nothing — in which case it returns an error and we skip).
	req, err := http.NewRequest(http.MethodGet, "http://x/v1/self/mac", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	mac, ifname, err := pickPeerMAC(req)
	if err != nil {
		t.Skipf("no usable interface on this host (CI sandbox?): %v", err)
	}
	if mac == "" || ifname == "" {
		t.Errorf("mac=%q ifname=%q; both should be non-empty", mac, ifname)
	}
}
