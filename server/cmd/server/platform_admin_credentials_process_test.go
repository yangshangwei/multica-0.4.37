package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const credentialPublicationChildFlag = "--multica-test-credential-publication-child"
const credentialPublicationExitCode = 86

func init() {
	if len(os.Args) != 3 || os.Args[1] != credentialPublicationChildFlag {
		return
	}
	// Exit before TestMain: this filesystem-only child must never initialize or
	// clean up the integration test database shared with the parent process.
	if err := interruptCredentialPublication(os.Args[2]); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(1)
}

func interruptCredentialPublication(path string) error {
	data, err := io.ReadAll(io.LimitReader(os.Stdin, 4097))
	if err != nil {
		return err
	}
	credentials, err := decodePlatformAdminCredentials(data)
	if err != nil {
		return err
	}
	file, err := openPlatformAdminCredentialFile(path)
	if err != nil {
		return err
	}
	defer file.close()
	file.publish = func(directory *os.File, temp, name string) error {
		if err := publishInitializationFile(directory, temp, name); err != nil {
			return err
		}
		// Terminate after the real syscall without running staging-file cleanup.
		os.Exit(credentialPublicationExitCode)
		return nil
	}
	return file.durable(credentials)
}

func TestPlatformAdminCredentialFileInterruptedPublication(t *testing.T) {
	file, credentials := credentialFileFixture(t)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(credentials)
	if err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(t.Context(), executable, credentialPublicationChildFlag, file.path)
	command.Stdin = bytes.NewReader(data)
	output, err := command.CombinedOutput()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != credentialPublicationExitCode {
		t.Fatalf("publication child did not reach the interruption point: %v\n%s", err, output)
	}
	file.close()

	reopened, err := openPlatformAdminCredentialFile(file.path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(reopened.close)
	got, exists, err := reopened.read()
	if err != nil || !exists {
		t.Fatalf("reading interrupted publication: exists=%t error=%v", exists, err)
	}
	if got != credentials {
		t.Fatal("interrupted publication changed the credentials")
	}
	before, err := os.ReadFile(file.path)
	if err != nil {
		t.Fatal(err)
	}
	reopened.publish = func(*os.File, string, string) error {
		t.Fatal("retry must reuse the published credentials")
		return nil
	}
	if err := reopened.durable(credentials); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(file.path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("retry changed the credential file")
	}
	entries, err := os.ReadDir(filepath.Dir(file.path))
	if err != nil || len(entries) != 1 || entries[0].Name() != reopened.name {
		t.Fatal("interrupted publication left a staging file")
	}
}
