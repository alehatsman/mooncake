package transport

// In-process SSH test server. Lets us exercise Session.Run / Upload /
// WriteFile against a real SSH protocol implementation without needing
// Docker, sshd, or a network. The handler dispatches incoming commands
// against a small in-memory table the test populates per case.
//
// Implementation pattern is the standard one for testing `golang.org/x/
// crypto/ssh`: NewServerConn over a net.Listener, accept one client, spawn
// a goroutine per channel, handle "session" channels by reading exec
// requests and responding with canned stdout/stderr/exit-status, handle
// "subsystem=sftp" requests by handing off to pkg/sftp.NewServer.

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"
	"testing"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// testSSHServer is a minimal in-process SSH server suitable for transport
// tests. Start it with newTestSSHServer; close with t.Cleanup.
type testSSHServer struct {
	t        *testing.T
	listener net.Listener
	addr     string
	port     int

	// commands maps exec request → canned response. When a command isn't
	// found, the server replies with empty output and exit code 0.
	commands map[string]commandResponse

	mu       sync.Mutex
	requests []string // log of received commands; for test assertion
}

type commandResponse struct {
	Stdout string
	Stderr string
	Exit   uint32
}

// newTestSSHServer accepts a single client, authenticates against an
// ephemeral keypair, and serves until t.Cleanup closes it. Returns the
// server handle plus the AuthMethod the client side should use.
func newTestSSHServer(t *testing.T) (*testSSHServer, ssh.AuthMethod) {
	t.Helper()

	// Client keypair (ed25519): smaller and faster than RSA for tests, and
	// matches the spec-44 §279 preferred order.
	_, clientPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("gen client key: %v", err)
	}
	clientSigner, err := ssh.NewSignerFromKey(clientPriv)
	if err != nil {
		t.Fatalf("client signer: %v", err)
	}

	// Server host key (RSA — supported widely; ed25519 also works).
	serverPriv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("gen server key: %v", err)
	}
	serverHostKey, err := ssh.NewSignerFromKey(serverPriv)
	if err != nil {
		t.Fatalf("server signer: %v", err)
	}

	srvCfg := &ssh.ServerConfig{
		PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if bytes.Equal(key.Marshal(), clientSigner.PublicKey().Marshal()) {
				return &ssh.Permissions{}, nil
			}
			return nil, errors.New("unknown client key")
		},
	}
	srvCfg.AddHostKey(serverHostKey)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := listener.Addr().(*net.TCPAddr)

	srv := &testSSHServer{
		t:        t,
		listener: listener,
		addr:     addr.IP.String(),
		port:     addr.Port,
		commands: make(map[string]commandResponse),
	}
	go srv.serve(srvCfg)
	t.Cleanup(func() { _ = listener.Close() })

	return srv, ssh.PublicKeys(clientSigner)
}

// expect registers cmd → response. Any command not registered returns
// empty output + exit 0.
func (s *testSSHServer) expect(cmd string, resp commandResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.commands[cmd] = resp
}

// received returns the (ordered) list of commands the server has been
// asked to run.
func (s *testSSHServer) received() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.requests))
	copy(out, s.requests)
	return out
}

// serve accepts connections in a loop. golang.org/x/crypto/ssh's handshake
// + channel/request dispatch is single-connection-friendly; the test
// scenarios fit one client per test.
func (s *testSSHServer) serve(cfg *ssh.ServerConfig) {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handleConn(conn, cfg)
	}
}

func (s *testSSHServer) handleConn(conn net.Conn, cfg *ssh.ServerConfig) {
	defer func() { _ = conn.Close() }()
	sshConn, chans, reqs, err := ssh.NewServerConn(conn, cfg)
	if err != nil {
		return
	}
	defer func() { _ = sshConn.Close() }()

	go ssh.DiscardRequests(reqs)
	for ch := range chans {
		switch ch.ChannelType() {
		case "session":
			go s.handleSession(ch)
		default:
			_ = ch.Reject(ssh.UnknownChannelType, ch.ChannelType())
		}
	}
}

// handleSession services one "session" channel: dispatches exec + subsystem
// requests. exec commands look up the canned response; subsystem=sftp
// hands off to pkg/sftp's server.
func (s *testSSHServer) handleSession(newCh ssh.NewChannel) {
	ch, reqs, err := newCh.Accept()
	if err != nil {
		return
	}

	for req := range reqs {
		switch req.Type {
		case "exec":
			cmd := decodeStringPayload(req.Payload)
			s.mu.Lock()
			s.requests = append(s.requests, cmd)
			resp := s.commands[cmd]
			s.mu.Unlock()
			_ = req.Reply(true, nil)

			if resp.Stdout != "" {
				_, _ = ch.Write([]byte(resp.Stdout))
			}
			if resp.Stderr != "" {
				_, _ = ch.Stderr().Write([]byte(resp.Stderr))
			}
			// Send the exit-status request before closing — the client
			// derives the exit code from it.
			_, _ = ch.SendRequest("exit-status", false, encodeUint32(resp.Exit))
			_ = ch.Close()
			return
		case "subsystem":
			name := decodeStringPayload(req.Payload)
			if name != "sftp" {
				_ = req.Reply(false, nil)
				continue
			}
			_ = req.Reply(true, nil)
			srv, err := sftp.NewServer(ch)
			if err != nil {
				_ = ch.Close()
				return
			}
			_ = srv.Serve()
			_ = ch.Close()
			return
		default:
			_ = req.Reply(false, nil)
		}
	}
	_ = ch.Close()
}

// decodeStringPayload pulls the leading length-prefixed string out of an
// SSH request payload. Both "exec" and "subsystem" payloads have this
// shape (command line / subsystem name).
func decodeStringPayload(payload []byte) string {
	if len(payload) < 4 {
		return ""
	}
	n := binary.BigEndian.Uint32(payload[:4])
	if 4+int(n) > len(payload) {
		return ""
	}
	return string(payload[4 : 4+n])
}

// encodeUint32 produces the wire shape for exit-status request payload.
func encodeUint32(v uint32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, v)
	return b
}

// connectClient returns a Session connected to srv via the new native
// transport. Used by the actual transport tests.
func (s *testSSHServer) connectClient(t *testing.T, authMethod ssh.AuthMethod) *Session {
	t.Helper()
	target := SSHTarget{User: "tester", Host: s.addr, Port: s.port}
	// Build a one-shot ClientConfig that uses the test AuthMethod directly,
	// bypassing buildAuthMethods (which expects ssh-agent / key files on
	// disk). Reaches Connect() via the same dial path it would use in prod.
	cfg := &ssh.ClientConfig{
		User:            "tester",
		Auth:            []ssh.AuthMethod{authMethod},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // test only
	}
	addr := s.addr + ":" + intToString(s.port)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial test server: %v", err)
	}
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, cfg)
	if err != nil {
		t.Fatalf("ssh handshake: %v", err)
	}
	return &Session{client: ssh.NewClient(sshConn, chans, reqs), target: target}
}

func intToString(v int) string {
	if v == 0 {
		return "0"
	}
	var buf [10]byte
	pos := len(buf)
	for v > 0 {
		pos--
		buf[pos] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[pos:])
}

// Make the imports of io / bytes deliberate — both are used by the SFTP
// server-side path that pkg/sftp builds on. Leaving them used here keeps
// the import block honest if pkg/sftp's exported API ever stops pulling
// them in.
var _ = io.Discard
var _ = bytes.NewBuffer