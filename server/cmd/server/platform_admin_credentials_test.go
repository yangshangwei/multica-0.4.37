package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func credentialFileFixture(t *testing.T) (*platformAdminCredentialFile, platformAdminCredentials) {
	t.Helper()
	file, err := openPlatformAdminCredentialFile(filepath.Join(t.TempDir(), "admin-initial-password.txt"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(file.close)
	c, err := newPlatformAdminCredentials(uuid.NewString(), uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	return file, c
}
func TestPlatformAdminCredentialFileRandomPrivateAndStable(t *testing.T) {
	file, c := credentialFileFixture(t)
	_, other := credentialFileFixture(t)
	if len(c.InitialPassword) != 32 || c.InitialPassword == other.InitialPassword {
		t.Fatal("initial passwords lack independent randomness")
	}
	if err := file.durable(c); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(file.path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("initial password file is not private")
	}
	got, exists, err := file.read()
	if err != nil || !exists || got != c {
		t.Fatal("credential file round trip failed")
	}
	before, err := os.ReadFile(file.path)
	if err != nil {
		t.Fatal(err)
	}
	if err = file.durable(c); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(file.path)
	if err != nil || string(after) != string(before) {
		t.Fatal("retry changed the credential file")
	}
	if err = file.durable(other); err == nil {
		t.Fatal("another intent replaced existing credentials")
	}
	entries, err := os.ReadDir(filepath.Dir(file.path))
	if err != nil || len(entries) != 1 {
		t.Fatal("private staging file leaked")
	}
}
func TestPlatformAdminCredentialFileRejectsUnsafeExistingFiles(t *testing.T) {
	for _, kind := range []string{"symlink", "hardlink", "directory", "public", "unknown"} {
		t.Run(kind, func(t *testing.T) {
			file, c := credentialFileFixture(t)
			switch kind {
			case "symlink":
				target := filepath.Join(t.TempDir(), "target")
				if err := os.WriteFile(target, []byte("unchanged"), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, file.path); err != nil {
					t.Fatal(err)
				}
			case "hardlink":
				if err := file.durable(c); err != nil {
					t.Fatal(err)
				}
				if err := os.Link(file.path, file.path+".alias"); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Mkdir(file.path, 0700); err != nil {
					t.Fatal(err)
				}
			case "public":
				if err := file.durable(c); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(file.path, 0644); err != nil {
					t.Fatal(err)
				}
			case "unknown":
				if err := os.WriteFile(file.path, []byte("unknown SECRET content"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			err := file.durable(c)
			if err == nil || strings.Contains(err.Error(), "SECRET") {
				t.Fatal("unsafe file was accepted or leaked")
			}
		})
	}
}
func TestPlatformAdminCredentialFileFailureAndResync(t *testing.T) {
	for _, stage := range []string{"write", "file_sync", "publish", "directory_sync"} {
		t.Run(stage, func(t *testing.T) {
			file, c := credentialFileFixture(t)
			realSync := file.syncFile
			switch stage {
			case "write":
				file.writeFile = func(*os.File, []byte) error { return errors.New("injected secret must not escape") }
			case "file_sync":
				file.syncFile = func(*os.File) error { return errors.New("sync failure") }
			case "publish":
				file.publish = func(*os.File, string, string) error { return errors.New("publish failure") }
			case "directory_sync":
				file.syncFile = func(f *os.File) error {
					if f == file.directory {
						return errors.New("directory sync failure")
					}
					return realSync(f)
				}
			}
			err := file.durable(c)
			if err == nil || strings.Contains(err.Error(), "secret") {
				t.Fatal("failure was ignored or exposed")
			}
			_, exists, readErr := file.read()
			if stage != "directory_sync" {
				if exists || readErr != nil {
					t.Fatal("pre-publication failure exposed a final file")
				}
				return
			}
			if !exists || readErr != nil {
				t.Fatal("published file must survive failed directory fsync")
			}
			if err = file.durable(c); err == nil {
				t.Fatal("retry skipped directory fsync")
			}
			files, dirs := 0, 0
			file.syncFile = func(f *os.File) error {
				if f == file.directory {
					dirs++
				} else {
					files++
				}
				return realSync(f)
			}
			if err = file.durable(c); err != nil {
				t.Fatal(err)
			}
			if files != 1 || dirs != 1 {
				t.Fatalf("retry syncs: file=%d directory=%d", files, dirs)
			}
		})
	}
}
func TestPlatformAdminCredentialFileNoClobberPublication(t *testing.T) {
	file, c := credentialFileFixture(t)
	realPublish := file.publish
	file.publish = func(dir *os.File, temp, name string) error {
		if err := os.WriteFile(file.path, []byte("other operator file"), 0600); err != nil {
			return err
		}
		return realPublish(dir, temp, name)
	}
	if err := file.durable(c); err == nil {
		t.Fatal("exclusive publication overwrote the race winner")
	}
	data, err := os.ReadFile(file.path)
	if err != nil || string(data) != "other operator file" {
		t.Fatal("race winner content changed")
	}
}
