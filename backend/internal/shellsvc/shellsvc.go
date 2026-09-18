// Package shellsvc runs persistent PTY-backed shell sessions on the host
// machine, keyed by ID, that outlive any single websocket connection so a
// browser tab can disconnect and reattach without losing the session.
package shellsvc

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"os/exec"
	"sort"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/creack/pty"
)

// ringBufferBytes caps how much output each session remembers for replay
// when a client (re)attaches, so a session left open indefinitely can't grow
// without bound.
const ringBufferBytes = 256 * 1024

type ringBuffer struct {
	data []byte
}

func (r *ringBuffer) Write(p []byte) {
	r.data = append(r.data, p...)
	if len(r.data) > ringBufferBytes {
		r.data = r.data[len(r.data)-ringBufferBytes:]
	}
}

func (r *ringBuffer) Bytes() []byte {
	out := make([]byte, len(r.data))
	copy(out, r.data)
	return out
}

// Session is one persistent shell process. It keeps running after its
// websocket disconnects; only Manager.Close ends it early.
type Session struct {
	ID        string
	CreatedAt time.Time

	cmd    *exec.Cmd
	ptmx   *os.File
	ring   ringBuffer
	remove func()

	mu   sync.Mutex
	conn *websocket.Conn
}

// Attach makes conn the session's live output target, replaying any
// buffered output first. A previously attached connection is closed, since
// this app doesn't support two live viewers of the same session.
func (s *Session) Attach(ctx context.Context, conn *websocket.Conn) {
	s.mu.Lock()
	old := s.conn
	s.conn = conn
	buf := s.ring.Bytes()
	s.mu.Unlock()

	if old != nil {
		old.Close(websocket.StatusNormalClosure, "replaced by new connection")
	}
	if len(buf) > 0 {
		_ = conn.Write(ctx, websocket.MessageBinary, buf)
	}
}

// Detach clears conn as the live target, but only if it's still the current
// one (a newer Attach may have already replaced it).
func (s *Session) Detach(conn *websocket.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == conn {
		s.conn = nil
	}
}

func (s *Session) Write(p []byte) {
	_, _ = s.ptmx.Write(p)
}

func (s *Session) Resize(rows, cols int) {
	if rows <= 0 || cols <= 0 {
		return
	}
	_ = pty.Setsize(s.ptmx, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
}

func (s *Session) kill() {
	_ = s.cmd.Process.Kill()
	_ = s.ptmx.Close()
}

func (s *Session) readLoop() {
	buf := make([]byte, 4096)
	for {
		n, err := s.ptmx.Read(buf)
		if n > 0 {
			chunk := append([]byte(nil), buf[:n]...)
			s.mu.Lock()
			s.ring.Write(chunk)
			conn := s.conn
			s.mu.Unlock()
			if conn != nil {
				_ = conn.Write(context.Background(), websocket.MessageBinary, chunk)
			}
		}
		if err != nil {
			s.mu.Lock()
			conn := s.conn
			s.conn = nil
			s.mu.Unlock()
			if conn != nil {
				conn.Close(websocket.StatusNormalClosure, "shell exited")
			}
			s.remove()
			return
		}
	}
}

// Manager tracks every live session. There is no idle timeout or cap:
// sessions live until a client explicitly closes them or the process exits
// on its own, per podtainer's design for the Shell page.
type Manager struct {
	mu       sync.Mutex
	sessions map[string]*Session
}

func NewManager() *Manager {
	return &Manager{sessions: map[string]*Session{}}
}

func newID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return hex.EncodeToString(buf)
}

func (m *Manager) Create() (*Session, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}
	cmd := exec.Command(shell)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	return m.start(cmd)
}

// CreateExec starts a PTY-backed `podman exec` session inside the given
// container, falling back to sh when bash isn't present in its image.
func (m *Manager) CreateExec(containerID string) (*Session, error) {
	cmd := exec.Command("podman", "exec", "-it", containerID, "sh", "-c",
		`if command -v bash >/dev/null 2>&1; then exec bash; else exec sh; fi`)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	return m.start(cmd)
}

func (m *Manager) start(cmd *exec.Cmd) (*Session, error) {
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}

	id := newID()
	s := &Session{
		ID:        id,
		CreatedAt: time.Now(),
		cmd:       cmd,
		ptmx:      ptmx,
	}

	m.mu.Lock()
	m.sessions[id] = s
	m.mu.Unlock()
	s.remove = func() {
		m.mu.Lock()
		delete(m.sessions, id)
		m.mu.Unlock()
	}

	go s.readLoop()
	return s, nil
}

func (m *Manager) Get(id string) (*Session, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	return s, ok
}

// List returns every live session, oldest first.
func (m *Manager) List() []*Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}

// Close ends a session immediately, killing its shell process.
func (m *Manager) Close(id string) bool {
	m.mu.Lock()
	s, ok := m.sessions[id]
	if ok {
		delete(m.sessions, id)
	}
	m.mu.Unlock()
	if ok {
		s.kill()
	}
	return ok
}
