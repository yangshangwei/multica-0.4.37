package main

import (
	"encoding/json"
	"errors"
	"github.com/multica-ai/multica/server/internal/cli"
	"github.com/spf13/cobra"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"
)

func TestManagedShutdownRequiresAcceptedIntent(t *testing.T) {
	token := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// A peer lacking response proof must never be treated as a shutdown ack.
		w.WriteHeader(202)
		json.NewEncoder(w).Encode(map[string]bool{"accepted": false})
	}))
	defer server.Close()
	target, _ := url.Parse(server.URL)
	port, _ := strconv.Atoi(target.Port())
	if err := postDaemonShutdown(port, token, nil); err == nil {
		t.Fatal("forged accepted response treated as shutdown")
	}
}

func TestManagedShutdownPeerReceivesSignedIntentBody(t *testing.T) {
	token := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	var received map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(202)
		w.Write([]byte(`{"accepted":false}`))
	}))
	defer server.Close()
	target, _ := url.Parse(server.URL)
	port, _ := strconv.Atoi(target.Port())
	_ = postDaemonShutdown(port, token, nil)
	if _, ok := received["intent_id"].(string); !ok {
		t.Fatal("managed shutdown did not send intent")
	}
	if _, ok := received["expected_intent_id"]; !ok {
		t.Fatal("managed shutdown omitted expected intent CAS")
	}
}

func TestManagedBackgroundStartRequiresProvenProcessExit(t *testing.T) {
	for _, pid := range []string{"invalid", strconv.Itoa(os.Getpid())} {
		t.Run(pid, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			const profile = "managed-start-guard"
			if err := cli.SaveCLIConfigForProfile(cli.CLIConfig{Token: "mul_fixture", ManagementDeploymentID: "11111111-1111-4111-8111-111111111111"}, profile); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(daemonPIDPathForProfile(profile), []byte(pid), 0600); err != nil {
				t.Fatal(err)
			}
			original := daemonExecutable
			defer func() { daemonExecutable = original }()
			resolved := false
			daemonExecutable = func() (string, error) { resolved = true; return "", errors.New("test stops before any process spawn") }
			command := &cobra.Command{}
			command.Flags().String("profile", profile, "")
			command.Flags().Bool("managed-handoff-stdin", false, "")
			err := runDaemonBackground(command)
			if err == nil || resolved {
				t.Fatalf("unproven managed PID reached process resolution: %v", err)
			}
		})
	}
}

func TestManagedPIDHolder(t *testing.T) {
	if os.Getenv("MULTICA_MANAGED_PID_HOLDER") == "1" {
		time.Sleep(time.Minute)
	}
}

func TestManagedForegroundDuplicateCannotErasePredecessorPID(t *testing.T) {
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	child := exec.Command(binary, "-test.run=^TestManagedPIDHolder$")
	child.Env = append(os.Environ(), "MULTICA_MANAGED_PID_HOLDER=1")
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = child.Process.Kill(); _ = child.Wait() }()
	for _, pid := range []string{"invalid", strconv.Itoa(child.Process.Pid)} {
		t.Run(pid, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			const profile = "managed-foreground-guard"
			if err := cli.SaveCLIConfigForProfile(cli.CLIConfig{Token: "mul_fixture", ManagementDeploymentID: "11111111-1111-4111-8111-111111111111"}, profile); err != nil {
				t.Fatal(err)
			}
			path := daemonPIDPathForProfile(profile)
			if err := os.WriteFile(path, []byte(pid), 0600); err != nil {
				t.Fatal(err)
			}
			command := &cobra.Command{}
			command.Flags().String("profile", profile, "")
			command.Flags().Bool("managed-handoff-stdin", false, "")
			if err := runDaemonForeground(command); err == nil {
				t.Fatal("duplicate managed foreground accepted")
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != pid {
				t.Fatal("failed duplicate destroyed predecessor evidence")
			}
		})
	}
}
