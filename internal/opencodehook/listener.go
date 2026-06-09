package opencodehook

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/deLiseLINO/codex-quota/internal/config"
)

const (
	stateFileName = "opencode-hook.json"
	maxBodyBytes  = 64 * 1024
)

type Signal struct {
	Source       string    `json:"source"`
	EventType    string    `json:"event_type"`
	SessionID    string    `json:"session_id"`
	ProviderID   string    `json:"provider_id"`
	ModelID      string    `json:"model_id"`
	ErrorName    string    `json:"error_name"`
	StatusCode   int       `json:"status_code"`
	Message      string    `json:"message"`
	ResponseBody string    `json:"response_body"`
	ReceivedAt   time.Time `json:"received_at"`
}

type State struct {
	Version   int       `json:"version"`
	URL       string    `json:"url"`
	Token     string    `json:"token"`
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"started_at"`
}

type Listener struct {
	statePath string
	state     State
	server    *http.Server
	listener  net.Listener
}

func StatePath() (string, error) {
	dir, err := config.ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, stateFileName), nil
}

func Start(onSignal func(Signal)) (*Listener, error) {
	if onSignal == nil {
		return nil, fmt.Errorf("signal handler is nil")
	}

	token, err := randomToken()
	if err != nil {
		return nil, err
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	statePath, err := StatePath()
	if err != nil {
		_ = ln.Close()
		return nil, err
	}

	state := State{
		Version:   1,
		URL:       "http://" + ln.Addr().String(),
		Token:     token,
		PID:       os.Getpid(),
		StartedAt: time.Now().UTC(),
	}

	mux := http.NewServeMux()
	listener := &Listener{statePath: statePath, state: state, listener: ln}
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/v1/opencode/quota-signal", listener.handleSignal(onSignal))
	listener.server = &http.Server{Handler: mux, ReadHeaderTimeout: 2 * time.Second}

	if err := writeState(statePath, state); err != nil {
		_ = ln.Close()
		return nil, err
	}

	go func() {
		if err := listener.server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			_ = listener.Close(context.Background())
		}
	}()

	return listener, nil
}

func (l *Listener) State() State {
	if l == nil {
		return State{}
	}
	return l.state
}

func (l *Listener) Close(ctx context.Context) error {
	if l == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var err error
	if l.server != nil {
		err = l.server.Shutdown(ctx)
	}
	removeStateIfCurrent(l.statePath, l.state.Token)
	return err
}

func (l *Listener) handleSignal(onSignal func(Signal)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !authorized(r.Header.Get("Authorization"), l.state.Token) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		defer r.Body.Close()

		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
		if err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}

		var signal Signal
		if err := json.Unmarshal(body, &signal); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if signal.ReceivedAt.IsZero() {
			signal.ReceivedAt = time.Now().UTC()
		}
		onSignal(signal)
		w.WriteHeader(http.StatusNoContent)
	}
}

func authorized(header, token string) bool {
	header = strings.TrimSpace(header)
	token = strings.TrimSpace(token)
	return token != "" && header == "Bearer "+token
}

func randomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate hook token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func writeState(path string, state State) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}

func removeStateIfCurrent(path, token string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return
	}
	if state.Token == token {
		_ = os.Remove(path)
	}
}
