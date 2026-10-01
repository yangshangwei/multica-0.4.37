package main

import (
	"bytes"
	"io"
	"os/exec"
	"reflect"
	"testing"

	"github.com/spf13/cobra"
)

func TestManagedHandoffForwardingDoesNotUseArgsOrEnvironment(t *testing.T) {
	child := exec.Command("test-created-fake-daemon", "daemon", "start", "--foreground", "--managed-handoff-stdin")
	raw := []byte(`{"proof":"test-only-pipe-payload"}`)
	forwardManagedHandoff(child, raw)
	received, err := io.ReadAll(child.Stdin)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(received, raw) {
		t.Fatal("supervisor did not forward exact one-shot bytes")
	}
	if len(child.Env) != 0 {
		t.Fatal("handoff modified environment")
	}
	if !reflect.DeepEqual(child.Args, []string{"test-created-fake-daemon", "daemon", "start", "--foreground", "--managed-handoff-stdin"}) {
		t.Fatal("handoff modified process arguments")
	}
	want := []string{"daemon", "start", "--foreground", "--profile", "desktop-test"}
	got := withoutManagedHandoffFlag([]string{"daemon", "start", "--foreground", "--managed-handoff-stdin", "--profile", "desktop-test"})
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("restart arguments: %v", got)
	}
}

func TestManagedHandoffRequiresExplicitProfile(t *testing.T) {
	command := &cobra.Command{}
	command.Flags().Bool("managed-handoff-stdin", true, "")
	command.Flags().String("profile", "", "")
	command.SetIn(bytes.NewReader([]byte("must not be read")))
	if _, _, err := readManagedHandoffForCommand(command); err == nil {
		t.Fatal("default user profile accepted a management handoff")
	}
}
