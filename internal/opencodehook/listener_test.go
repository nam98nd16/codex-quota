package opencodehook

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestListenerAcceptsAuthorizedSignal(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CQ_CONFIG_HOME", dir)

	received := make(chan Signal, 1)
	listener, err := Start(func(signal Signal) { received <- signal })
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer listener.Close(context.Background())

	state := listener.State()
	req, err := http.NewRequest(http.MethodPost, state.URL+"/v1/opencode/quota-signal", bytes.NewBufferString(`{"source":"opencode","status_code":429,"message":"quota exhausted"}`))
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+state.Token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	select {
	case signal := <-received:
		if signal.StatusCode != 429 || signal.Message != "quota exhausted" {
			t.Fatalf("unexpected signal: %#v", signal)
		}
		if signal.ReceivedAt.IsZero() {
			t.Fatalf("expected ReceivedAt to be populated")
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for signal")
	}
}

func TestListenerRejectsUnauthorizedSignal(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CQ_CONFIG_HOME", dir)

	received := make(chan Signal, 1)
	listener, err := Start(func(signal Signal) { received <- signal })
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer listener.Close(context.Background())

	req, err := http.NewRequest(http.MethodPost, listener.State().URL+"/v1/opencode/quota-signal", bytes.NewBufferString(`{}`))
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	req.Header.Set("Authorization", "Bearer wrong")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}

	select {
	case signal := <-received:
		t.Fatalf("unexpected signal: %#v", signal)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestListenerRemovesCurrentStateOnClose(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CQ_CONFIG_HOME", dir)

	listener, err := Start(func(Signal) {})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	path := filepath.Join(dir, "codex-quota", stateFileName)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected state file: %v", err)
	}
	if err := listener.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("state file still exists after close: %v", err)
	}
}
