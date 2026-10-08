package marksman

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io"
	"os/exec"
	"sync"
)

const maxSessions = 8

// A Marksman session belongs to the provider extension, never to a single RPC.
type lspSession struct {
	mu     sync.Mutex
	stopMu sync.Mutex
	closed bool
	cmd    *exec.Cmd
	input  io.WriteCloser
	client *stdioClient
	opened map[string]openedFile
}
type openedFile struct {
	version int
	sha     [32]byte
}

func (s *lspSession) stop() {
	s.stopMu.Lock()
	defer s.stopMu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	if s.input != nil {
		_ = s.input.Close()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
		_ = s.cmd.Wait()
	}
}
func (s *lspSession) start(binary, workspace string) error {
	s.stopMu.Lock()
	defer s.stopMu.Unlock()
	if s.closed {
		return fmt.Errorf("Marksman session already closed")
	}
	cmd := exec.Command(binary, "server")
	cmd.Dir = workspace
	cmd.Stderr = io.Discard
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return err
	}
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return err
	}
	s.cmd, s.input = cmd, stdin
	s.client = &stdioClient{stdin: stdin, out: bufio.NewReader(stdout)}
	s.opened = make(map[string]openedFile)
	return nil
}
func (s *lspSession) syncDocument(uri string, source []byte) error {
	sha := sha256.Sum256(source)
	old, ok := s.opened[uri]
	if ok && old.sha == sha {
		return nil
	}
	if !ok {
		if err := s.client.notify("textDocument/didOpen", map[string]any{"textDocument": map[string]any{
			"uri": uri, "languageId": "markdown", "version": 1, "text": string(source)}}); err != nil {
			return err
		}
		s.opened[uri] = openedFile{version: 1, sha: sha}
		return nil
	}
	version := old.version + 1
	if err := s.client.notify("textDocument/didChange", map[string]any{
		"textDocument":   map[string]any{"uri": uri, "version": version},
		"contentChanges": []map[string]string{{"text": string(source)}},
	}); err != nil {
		return err
	}
	s.opened[uri] = openedFile{version: version, sha: sha}
	return nil
}
func (e *Extension) getSession(workspace string) (*lspSession, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.sessions == nil {
		e.sessions = map[string]*lspSession{}
	}
	if s := e.sessions[workspace]; s != nil {
		return s, false
	}
	s := &lspSession{}
	if len(e.sessions) >= maxSessions {
		return s, true
	}
	e.sessions[workspace] = s
	return s, false
}
func (e *Extension) dropSession(workspace string, s *lspSession) {
	e.mu.Lock()
	if e.sessions[workspace] == s {
		delete(e.sessions, workspace)
	}
	e.mu.Unlock()
	s.stop()
}
func (e *Extension) Close() error {
	e.mu.Lock()
	sessions := e.sessions
	e.sessions = nil
	e.mu.Unlock()
	for _, s := range sessions {
		s.stop()
	}
	return nil
}

// Requests on a session are serialized; initialize uses id 1.
func (s *lspSession) requestID() int { return 2 }
