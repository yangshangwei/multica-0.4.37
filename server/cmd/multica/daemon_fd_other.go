//go:build !linux

package main

// Other platforms use their native process-isolation rules. The Linux
// implementation is deliberately kept out of those builds because /proc and
// Linux fcntl constants are not portable.
func prepareDaemonChildFds() error { return nil }
