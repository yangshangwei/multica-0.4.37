//go:build linux

package agent

import (
	"fmt"
	"os"
	"strings"
)

func nativeProcessInstanceID(process *os.Process) (string, error) {
	pid := process.Pid
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return "", err
	}
	end := strings.LastIndexByte(string(data), ')')
	if end < 0 {
		return "", fmt.Errorf("invalid process instance record")
	}
	fields := strings.Fields(string(data)[end+1:])
	if len(fields) < 20 {
		return "", fmt.Errorf("incomplete process instance record")
	}
	boot, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("linux:%s:%d:%s", strings.TrimSpace(string(boot)), pid, fields[19]), nil
}
