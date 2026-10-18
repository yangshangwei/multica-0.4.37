//go:build !linux

package collaborationrun

import "os/exec"

func namespaceSupported() bool     { return false }
func configureNamespace(*exec.Cmd) {}
func NamespaceHelperMain() int     { return 125 }
