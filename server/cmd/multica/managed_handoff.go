package main

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"

	"github.com/multica-ai/multica/server/internal/daemon"
	"github.com/spf13/cobra"
)

func readManagedHandoffForCommand(cmd *cobra.Command) ([]byte, *daemon.ManagedStartupHandoff, error) {
	enabled, _ := cmd.Flags().GetBool("managed-handoff-stdin")
	if !enabled {
		return nil, nil, nil
	}
	if resolveProfile(cmd) == "" {
		return nil, nil, errors.New("managed handoff requires an explicit daemon profile")
	}
	return daemon.ReadManagedStartupHandoff(cmd.InOrStdin())
}

func forwardManagedHandoff(child *exec.Cmd, raw []byte) {
	if len(raw) > 0 {
		child.Stdin = bytes.NewReader(raw)
	}
}

func withoutManagedHandoffFlag(args []string) []string {
	result := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--managed-handoff-stdin" || strings.HasPrefix(arg, "--managed-handoff-stdin=") {
			continue
		}
		result = append(result, arg)
	}
	return result
}
