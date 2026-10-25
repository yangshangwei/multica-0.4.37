//go:build !linux && !darwin && !windows

package agent

import "os"

func nativeProcessInstanceID(*os.Process) (string, error) { return "", ErrLaunchUnsupported }
