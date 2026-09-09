//go:build windows

package agent

import (
	"strings"
	"testing"
)

func TestACPManagedTerminalUsesUTF8CmdCodePage(t *testing.T) {
	shell, args := acpTerminalShellCommand("echo test")
	if shell != "cmd.exe" {
		t.Fatalf("shell = %q, want cmd.exe", shell)
	}
	if len(args) != 4 || args[0] != "/d" || args[1] != "/s" || args[2] != "/c" {
		t.Fatalf("args = %#v, want cmd.exe flags followed by a command", args)
	}
	if !strings.Contains(args[3], "chcp 65001") || !strings.Contains(args[3], "echo test") {
		t.Fatalf("wrapped command = %q, want explicit UTF-8 setup and original command", args[3])
	}
}

func TestNormalizeTerminalOutputDecodesACPBytes(t *testing.T) {
	got := string(decodeWindowsCodePage([]byte{0xD6, 0xD0}, 936))
	if got != "中" {
		t.Fatalf("decoded CP936 output = %q, want 中", got)
	}
}
