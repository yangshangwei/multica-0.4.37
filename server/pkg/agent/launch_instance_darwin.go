//go:build darwin

package agent

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func nativeProcessInstanceID(process *os.Process) (string, error) {
	pid := process.Pid
	info, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("darwin:%d:%d:%d", pid, info.Proc.P_starttime.Sec, info.Proc.P_starttime.Usec), nil
}
