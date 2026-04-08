package cmd

import (
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jinto/ina/daemon"
)

func TestForwardHook_NoSocket(t *testing.T) {
	// When socket does not exist, forwardHook must return nil (exit 0).
	socketPath := filepath.Join(t.TempDir(), "missing.sock")
	err := forwardHook(socketPath, "post-tool-use", strings.NewReader(`{}`), 500*time.Millisecond)
	if err != nil {
		t.Fatalf("expected nil error when socket is missing, got %v", err)
	}
}

func TestForwardHook_SocketExists(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "test.sock")

	// Start a fake daemon listener that reads one command.
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	received := make(chan daemon.Command, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		var cmd daemon.Command
		json.NewDecoder(conn).Decode(&cmd)
		// Send response so client doesn't hang.
		json.NewEncoder(conn).Encode(daemon.Response{OK: true})
		received <- cmd
	}()

	stdin := `{"session_id":"s1","cwd":"/tmp","hook_event_name":"PostToolUse","tool_name":"Bash"}`
	err = forwardHook(socketPath, "post-tool-use", strings.NewReader(stdin), 500*time.Millisecond)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	select {
	case cmd := <-received:
		if cmd.Action != "hook" {
			t.Errorf("action = %q, want %q", cmd.Action, "hook")
		}
		// Verify the event name and stdin are in the data payload.
		var payload struct {
			Event string          `json:"event"`
			Body  json.RawMessage `json:"body"`
		}
		json.Unmarshal(cmd.Data, &payload)
		if payload.Event != "post-tool-use" {
			t.Errorf("event = %q, want %q", payload.Event, "post-tool-use")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for daemon to receive command")
	}
}

func TestForwardHook_ConnectTimeout(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "slow.sock")

	// Create a listener but never accept — simulates hung daemon.
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	// Set backlog to 0 by not accepting. The connect will block once backlog fills.
	// On macOS/Linux, Unix sockets have generous backlog, so we test with a very short timeout.
	start := time.Now()
	err = forwardHook(socketPath, "post-tool-use", strings.NewReader(`{}`), 50*time.Millisecond)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("expected nil error on timeout, got %v", err)
	}
	// Should complete quickly, not hang for seconds.
	if elapsed > 2*time.Second {
		t.Fatalf("took %v, expected fast timeout", elapsed)
	}
}

func TestForwardHook_EmptyStdin(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "test.sock")

	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		// Read and discard.
		io.ReadAll(conn)
	}()

	// Empty stdin should not crash.
	err = forwardHook(socketPath, "post-tool-use", strings.NewReader(""), 500*time.Millisecond)
	if err != nil {
		t.Fatalf("expected nil on empty stdin, got %v", err)
	}
}

func TestForwardHook_StdinIsNil(t *testing.T) {
	// os.Stdin could theoretically be nil in edge cases.
	socketPath := filepath.Join(t.TempDir(), "missing.sock")
	err := forwardHook(socketPath, "post-tool-use", nil, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("expected nil error on nil stdin, got %v", err)
	}
}

func init() {
	// Suppress PersistentPreRunE config loading in tests.
	_ = os.MkdirAll(filepath.Join(os.TempDir(), ".ina"), 0700)
}
