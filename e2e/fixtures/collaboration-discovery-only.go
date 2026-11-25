// Build explicitly for the native acceptance host. This is not a task provider.
package main

import (
	"fmt"
	"os"
	"regexp"
)

var sourceCommit string

func main() {
	if !regexp.MustCompile(`^[a-f0-9]{40}$`).MatchString(sourceCommit) {
		os.Exit(78)
	}
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Println("2.1.5 (Claude Code)")
		return
	}
	executable, err := os.Executable()
	if err != nil {
		os.Exit(98)
	}
	marker, err := os.OpenFile(executable+".unexpected", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		os.Exit(98)
	}
	_, writeErr := marker.WriteString("unexpected agent execution\n")
	closeErr := marker.Close()
	if writeErr != nil || closeErr != nil {
		os.Exit(98)
	}
	os.Exit(97)
}
