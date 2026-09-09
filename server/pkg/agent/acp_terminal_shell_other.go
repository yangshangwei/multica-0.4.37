//go:build !windows

package agent

func acpTerminalShellCommand(command string) (string, []string) {
	return "/bin/sh", []string{"-c", command}
}

func normalizeTerminalOutput(raw []byte) []byte {
	return raw
}
