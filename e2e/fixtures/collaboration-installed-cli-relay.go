// This test-only relay lets the exact cached CLI make real HTTP requests in a
// network-none container. Credentials travel on private stdin, never argv.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

type bootstrap struct {
	Token     string   `json:"token"`
	Workspace string   `json:"workspace_id"`
	Args      []string `json:"args"`
}

type frame struct {
	Kind     string              `json:"kind"`
	ID       uint64              `json:"id,omitempty"`
	Method   string              `json:"method,omitempty"`
	Path     string              `json:"path,omitempty"`
	Headers  map[string][]string `json:"headers,omitempty"`
	Body     []byte              `json:"body,omitempty"`
	Status   int                 `json:"status,omitempty"`
	ExitCode int                 `json:"exit_code"`
	Stdout   string              `json:"stdout,omitempty"`
	Stderr   string              `json:"stderr,omitempty"`
}

func main() {
	if err := run(); err != nil {
		// Errors must never dump input, credentials or the CLI environment.
		fmt.Fprintln(os.Stderr, "installed CLI relay failed")
		os.Exit(1)
	}
}

func run() error {
	decoder := json.NewDecoder(io.LimitReader(os.Stdin, 4<<20))
	var input bootstrap
	if err := decoder.Decode(&input); err != nil {
		return err
	}
	if len(input.Args) < 2 || input.Args[0] != "issue" || input.Token == "" || input.Workspace == "" {
		return fmt.Errorf("invalid bootstrap")
	}
	root, err := os.MkdirTemp("/tmp", "installed-cli-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	config := filepath.Join(root, "multica")
	if err := os.Mkdir(config, 0700); err != nil {
		return err
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return err
	}
	var writes, pendingMu sync.Mutex
	encoder := json.NewEncoder(os.Stdout)
	emit := func(value frame) error {
		writes.Lock()
		defer writes.Unlock()
		return encoder.Encode(value)
	}
	pending := make(map[uint64]chan frame)
	var sequence uint64
	go func() {
		for {
			var response frame
			if decoder.Decode(&response) != nil {
				return
			}
			pendingMu.Lock()
			reply := pending[response.ID]
			pendingMu.Unlock()
			if reply != nil && response.Kind == "response" {
				reply <- response
			}
		}
	}()
	server := &http.Server{ReadHeaderTimeout: 5 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			http.Error(w, "fixture body unavailable", http.StatusBadGateway)
			return
		}
		pendingMu.Lock()
		sequence++
		id := sequence
		reply := make(chan frame, 1)
		pending[id] = reply
		pendingMu.Unlock()
		defer func() {
			pendingMu.Lock()
			delete(pending, id)
			pendingMu.Unlock()
		}()
		if emit(frame{Kind: "request", ID: id, Method: r.Method, Path: r.URL.RequestURI(), Headers: r.Header, Body: body}) != nil {
			http.Error(w, "fixture transport unavailable", http.StatusBadGateway)
			return
		}
		select {
		case response := <-reply:
			for key, values := range response.Headers {
				for _, value := range values {
					w.Header().Add(key, value)
				}
			}
			w.WriteHeader(response.Status)
			_, _ = w.Write(response.Body)
		case <-time.After(15 * time.Second):
			http.Error(w, "fixture response unavailable", http.StatusGatewayTimeout)
		case <-r.Context().Done():
		}
	})}
	defer server.Close()
	go func() { _ = server.Serve(listener) }()
	cmd := exec.Command("/app/multica", input.Args...)
	cmd.Env = append(os.Environ(), "MULTICA_TASK_CONFIG_ROOT="+config,
		"MULTICA_SERVER_URL=http://"+listener.Addr().String(), "MULTICA_TOKEN="+input.Token,
		"MULTICA_WORKSPACE_ID="+input.Workspace, "MULTICA_HTTP_TIMEOUT=10s",
		"MULTICA_DAEMON_AUTO_UPDATE=false", "MULTICA_DAEMON_AUTO_RELOAD=false")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	exit := 0
	if err := cmd.Run(); err != nil {
		if failure, ok := err.(*exec.ExitError); ok {
			exit = failure.ExitCode()
		} else {
			return err
		}
	}
	return emit(frame{Kind: "result", ExitCode: exit, Stdout: stdout.String(), Stderr: stderr.String()})
}
