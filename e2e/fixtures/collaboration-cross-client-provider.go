// The AC33 fixture builds this stdlib-only executable for Linux. It never
// discovers a provider, reads an account, or manufactures termination proof.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != 3 || (os.Args[1] != "A" && os.Args[1] != "B") {
		return fmt.Errorf("an explicit fixture lane and callback address are required")
	}
	var input struct {
		Identity struct {
			RunID string `json:"run_id"`
		} `json:"identity"`
		Inputs []struct {
			CommandID string `json:"command_id"`
		} `json:"inputs"`
	}
	var raw json.RawMessage
	if err := json.NewDecoder(io.LimitReader(os.Stdin, 1<<20)).Decode(&raw); err != nil {
		return err
	}
	if err := json.Unmarshal(raw, &input); err != nil || input.Identity.RunID == "" || len(input.Inputs) == 0 {
		return fmt.Errorf("missing actual provider input")
	}
	namespace, err := os.Readlink("/proc/self/ns/pid")
	if err != nil {
		return err
	}
	connection, err := net.DialTimeout("tcp4", os.Args[2], 10*time.Second)
	if err != nil {
		return err
	}
	defer connection.Close()
	if err := connection.SetDeadline(time.Now().Add(150 * time.Second)); err != nil {
		return err
	}
	if err := json.NewEncoder(connection).Encode(struct {
		Lane        string          `json:"lane"`
		NamespaceID string          `json:"namespace_id"`
		PID         int             `json:"pid"`
		Input       json.RawMessage `json:"input"`
	}{os.Args[1], namespace, os.Getpid(), raw}); err != nil {
		return err
	}
	var release [1]byte
	if _, err := io.ReadFull(connection, release[:]); err != nil || release[0] != 'r' {
		return fmt.Errorf("fixture release was not received")
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]string{
		"outcome": "success",
		"body":    "AC33 synthetic " + os.Args[1] + " result for " + input.Identity.RunID,
	})
}
