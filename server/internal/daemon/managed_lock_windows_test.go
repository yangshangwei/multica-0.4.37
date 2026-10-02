//go:build windows

package daemon

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Atomic replacement/removal acquires DELETE access. A reader must share that
// access, even when the other handle permits concurrent reads and writes.
func TestManagedLockWindowsReadsDuringDeleteAccess(t *testing.T) {
	directory := t.TempDir()
	current, err := managementProcessIdentity()
	if err != nil {
		t.Fatal(err)
	}
	nonce := uuid.NewString()
	path := filepath.Join(directory, nonce+".json")
	ticket := "1"
	owner := managementLockTicket{Version: 1, PID: os.Getpid(), OwnerNonce: nonce, HostID: current.hostID, BootID: current.bootID, Ticket: &ticket}
	if err := writeManagementJSON(path, owner); err != nil {
		t.Fatal(err)
	}
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	const deleteAccess = 0x00010000 // DELETE standard access right.
	handle, err := syscall.CreateFile(name, deleteAccess, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.CloseHandle(handle)
	// Verify the native setup reproduces the old os.ReadFile failure, so a
	// passing scan cannot accidentally rely on an uncontended handle.
	if _, err := os.ReadFile(path); !errors.Is(err, syscall.Errno(32)) {
		t.Fatalf("expected sharing violation from non-delete-sharing reader, got %v", err)
	}
	records, err := managementTickets(directory)
	if err != nil {
		t.Fatalf("read alongside peer deletion handle: %v", err)
	}
	if len(records) != 1 || records[0].OwnerNonce != nonce {
		t.Fatalf("live owner was omitted: %+v", records)
	}
}

func TestManagedLockWindowsPublicationWaitsForReaderAndReleases(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ticket.json")
	if err := os.WriteFile(path, []byte("previous"), 0600); err != nil {
		t.Fatal(err)
	}
	reader, err := openManagementLockFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()

	published := make(chan error, 1)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		published <- writeManagementJSONWithRename(path, "next", renameManagementLockFile)
	}()
	defer func() {
		reader.Close()
		<-finished
	}()
	select {
	case err := <-published:
		t.Fatalf("publication returned while destination reader remained open: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	raw, err := io.ReadAll(reader)
	if err != nil || string(raw) != "previous" {
		t.Fatalf("blocked publication changed old ticket: %q, %v", raw, err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-published:
		if err != nil {
			t.Fatalf("publication failed after reader closed: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("publication did not finish after reader closed")
	}
	raw, err = os.ReadFile(path)
	if err != nil || string(raw) != "\"next\"\n" {
		t.Fatalf("new ticket not published: %q, %v", raw, err)
	}
	next, err := openManagementLockFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	if err := os.Remove(path); err != nil {
		t.Fatalf("reader blocked ticket release: %v", err)
	}
	// Windows may retain a delete-pending name until its last reader closes.
	if err := next.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("released ticket still visible: %v", err)
	}
}

func TestManagedLockWindowsLongPaths(t *testing.T) {
	base := t.TempDir()
	directory := filepath.Join(base, strings.Repeat("a", 100), strings.Repeat("b", 100), strings.Repeat("c", 100))
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "ticket.json")
	if err := os.WriteFile(path, []byte("ticket"), 0600); err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(base, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(base)
	for _, name := range []string{path, relative, extendedLengthPrefix + path} {
		normalized, err := managementLockWindowsPath(name)
		if err != nil || normalized != extendedLengthPrefix+path {
			t.Fatalf("long path normalization: %q, %v", normalized, err)
		}
		raw, err := readManagementLockFile(name)
		if err != nil || string(raw) != "ticket" {
			t.Fatalf("long ticket path %q: %q, %v", name, raw, err)
		}
	}
	// A network share is not required to check the UNC prefix conversion.
	unc := `\\server\share\` + strings.Repeat("segment\\", 40) + "ticket.json"
	normalized, err := managementLockWindowsPath(unc)
	if err != nil || normalized != extendedLengthUNCPrefix+unc[2:] {
		t.Fatalf("UNC normalization: %q, %v", normalized, err)
	}
}

func TestManagedLockWindowsPublicationTimeoutPreservesTicket(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "ticket.json")
	if err := os.WriteFile(path, []byte("previous"), 0600); err != nil {
		t.Fatal(err)
	}
	reader, err := openManagementLockFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	started := time.Now()
	err = writeManagementJSONWithRename(path, "next", func(from, to string) error {
		return renameManagementLockFileUntil(from, to, time.Now().Add(50*time.Millisecond))
	})
	if !errors.Is(err, syscall.ERROR_ACCESS_DENIED) && !errors.Is(err, syscall.Errno(32)) {
		t.Fatalf("expected preserved rename cause, got %v", err)
	}
	if elapsed := time.Since(started); elapsed < 50*time.Millisecond || elapsed > 2*time.Second {
		t.Fatalf("unbounded or missing publication retry: %v", elapsed)
	}
	raw, readErr := os.ReadFile(path)
	if readErr != nil || string(raw) != "previous" {
		t.Fatalf("failed publication changed old ticket: %q, %v", raw, readErr)
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 1 || entries[0].Name() != "ticket.json" {
		t.Fatalf("temporary publication leaked: %+v, %v", entries, err)
	}
}
