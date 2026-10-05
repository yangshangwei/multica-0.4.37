//go:build unix

package service

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestMcpTemplateDir_DirectorySwapDoesNotBlock(t *testing.T) {
	if kind := os.Getenv("MULTICA_TEST_MCP_DIRECTORY_SWAP"); kind != "" {
		base := os.Getenv("MULTICA_TEST_MCP_DIRECTORY_SWAP_DIR")
		path := filepath.Join(base, "candidate")
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		before, err := os.Lstat(path)
		if err != nil || !before.IsDir() {
			t.Fatal("directory precheck failed")
		}
		var parent *os.Root
		name := path
		if kind == "entry" {
			parent, err = os.OpenRoot(base)
			if err != nil {
				t.Fatal(err)
			}
			defer parent.Close()
			name = "candidate"
		}
		// Deterministically schedule the writer between the catalog's Lstat and
		// directory open. No FIFO writer exists to release a blocking open.
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := syscall.Mkfifo(path, 0o600); err != nil {
			t.Fatal(err)
		}
		opened, err := openMcpTemplateDirectory(parent, name)
		if err == nil {
			opened.Close()
			t.Fatal("replacement FIFO was accepted as a directory")
		}
		return
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"root", "entry"} {
		t.Run(kind, func(t *testing.T) {
			// Isolate the potentially blocked syscall so a regression is killed
			// and reaped instead of leaking a goroutine in the test process.
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, executable, "-test.run=^TestMcpTemplateDir_DirectorySwapDoesNotBlock$")
			command.Env = append(os.Environ(), "MULTICA_TEST_MCP_DIRECTORY_SWAP="+kind, "MULTICA_TEST_MCP_DIRECTORY_SWAP_DIR="+t.TempDir())
			output, err := command.CombinedOutput()
			if ctx.Err() != nil {
				t.Fatal("directory-to-FIFO replacement blocked OpenRoot")
			}
			if err != nil {
				t.Fatalf("directory swap subprocess failed: %v\n%s", err, output)
			}
		})
	}
}

func TestMcpTemplateDir_SpecialFileDoesNotBlock(t *testing.T) {
	catalog := McpCatalog{Directory: t.TempDir()}
	directory := filepath.Join(catalog.Directory, "pipe")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(directory, "mcp.json"), 0o600); err != nil {
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	go func() { _, err := catalog.List(); finished <- err }()
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("special-file manifest blocked the catalog")
	}
}

func TestMcpTemplateDir_RootPermissionFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read mode-000 directories")
	}
	directory := t.TempDir()
	if err := os.Chmod(directory, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(directory, 0o700); err != nil {
			t.Error(err)
		}
	})
	if _, err := (McpCatalog{Directory: directory}).List(); !errors.Is(err, ErrMcpCatalogUnavailable) {
		t.Fatalf("permission failure was hidden: %v", err)
	}
}
