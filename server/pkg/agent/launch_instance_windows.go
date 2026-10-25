//go:build windows

package agent

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

func nativeProcessInstanceID(process *os.Process) (string, error) {
	var instanceID string
	var queryErr error
	err := process.WithHandle(func(handle uintptr) {
		var created, exited, kernel, user windows.Filetime
		queryErr = windows.GetProcessTimes(windows.Handle(handle), &created, &exited, &kernel, &user)
		if queryErr == nil {
			instanceID = fmt.Sprintf("windows:%d:%d:%d", process.Pid, created.HighDateTime, created.LowDateTime)
		}
	})
	if err != nil {
		return "", err
	}
	return instanceID, queryErr
}
