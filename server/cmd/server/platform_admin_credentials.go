package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/service"
)

const initializationStagingPrefix = ".admin-initial-password."

// This structure contains a secret. Never format it into diagnostics or receipts.
type platformAdminCredentials struct {
	SchemaVersion    int    `json:"schema_version"`
	InitializationID string `json:"initialization_id"`
	OrganizationID   string `json:"organization_id"`
	DeploymentID     string `json:"deployment_id"`
	Username         string `json:"username"`
	InitialPassword  string `json:"initial_password"`
	CreatedAt        string `json:"created_at"`
}

func newPlatformAdminCredentials(organization, deployment string) (platformAdminCredentials, error) {
	var secret [24]byte
	if _, err := rand.Read(secret[:]); err != nil {
		return platformAdminCredentials{}, errors.New("randomness_unavailable")
	}
	return platformAdminCredentials{1, uuid.NewString(), organization, deployment, service.InitialPlatformAdminUsername, base64.RawURLEncoding.EncodeToString(secret[:]), time.Now().UTC().Format(time.RFC3339Nano)}, nil
}

func decodePlatformAdminCredentials(data []byte) (platformAdminCredentials, error) {
	var result platformAdminCredentials
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return result, errors.New("credentials_file_invalid")
	}
	if decoder.Decode(new(any)) != io.EOF {
		return result, errors.New("credentials_file_invalid")
	}
	if result.SchemaVersion != 1 || result.Username != service.InitialPlatformAdminUsername {
		return result, errors.New("credentials_file_invalid")
	}
	for _, value := range []string{result.InitializationID, result.OrganizationID, result.DeploymentID} {
		id, err := uuid.Parse(value)
		if err != nil || id == uuid.Nil || id.String() != value {
			return result, errors.New("credentials_file_invalid")
		}
	}
	secret, err := base64.RawURLEncoding.DecodeString(result.InitialPassword)
	if err != nil || len(secret) != 24 || base64.RawURLEncoding.EncodeToString(secret) != result.InitialPassword {
		return result, errors.New("credentials_file_invalid")
	}
	if _, err := time.Parse(time.RFC3339Nano, result.CreatedAt); err != nil {
		return result, errors.New("credentials_file_invalid")
	}
	return result, nil
}

// Pinned directory handles and no-follow relative opens keep inspection,
// publication and fsync on the same directory and inode. Function fields are
// syscall fault-injection seams for durability tests, not production overrides.
type platformAdminCredentialFile struct {
	directory *os.File
	path      string
	name      string
	syncFile  func(*os.File) error
	writeFile func(*os.File, []byte) error
	publish   func(*os.File, string, string) error
}

func openPlatformAdminCredentialFile(path string) (*platformAdminCredentialFile, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || filepath.Base(path) == "." {
		return nil, errors.New("invalid_credentials_path")
	}
	directory, err := openInitializationDirectory(filepath.Dir(path))
	if err != nil {
		return nil, errors.New("credentials_directory_unsafe")
	}
	return &platformAdminCredentialFile{
		directory: directory, path: path, name: filepath.Base(path),
		syncFile:  func(f *os.File) error { return f.Sync() },
		writeFile: func(f *os.File, data []byte) error { _, err := f.Write(data); return err },
		publish:   publishInitializationFile,
	}, nil
}

func (f *platformAdminCredentialFile) close() { _ = f.directory.Close() }

func (f *platformAdminCredentialFile) read() (platformAdminCredentials, bool, error) {
	var empty platformAdminCredentials
	file, err := openInitializationFile(f.directory, f.name, false)
	if errors.Is(err, os.ErrNotExist) {
		return empty, false, nil
	}
	if err != nil {
		return empty, true, errors.New("credentials_file_unsafe")
	}
	defer file.Close()
	if err = checkInitializationFile(file); err != nil {
		return empty, true, err
	}
	data, err := io.ReadAll(io.LimitReader(file, 4097))
	if err != nil || len(data) > 4096 {
		return empty, true, errors.New("credentials_file_invalid")
	}
	result, err := decodePlatformAdminCredentials(data)
	return result, true, err
}

func (f *platformAdminCredentialFile) durable(expected platformAdminCredentials) error {
	current, exists, err := f.read()
	if err != nil {
		return err
	}
	if exists {
		if current != expected {
			return errors.New("credentials_file_conflict")
		}
	} else {
		tempName := initializationStagingPrefix + uuid.NewString()
		temp, err := openInitializationFile(f.directory, tempName, true)
		if err != nil {
			return errors.New("credentials_file_create_failed")
		}
		// Only this invocation's exclusive staging file is ever removed. A final
		// file survives every rollback, sync failure, and uncertain commit.
		published := false
		defer func() {
			if !published {
				_ = removeInitializationFile(f.directory, tempName)
			}
		}()
		data, err := json.MarshalIndent(expected, "", "  ")
		if err == nil {
			err = f.writeFile(temp, append(data, '\n'))
		}
		if err == nil {
			err = f.syncFile(temp)
		}
		closeErr := temp.Close()
		if err != nil || closeErr != nil {
			return errors.New("credentials_file_write_failed")
		}
		if err = f.publish(f.directory, tempName, f.name); err != nil {
			return errors.New("credentials_file_publish_failed")
		}
		// Exclusive rename consumes the staging name atomically. Do not unlink
		// that name afterward: it no longer belongs to this invocation.
		published = true
	}
	// Reopen and validate the final inode, including on retry after a failed
	// directory sync. Existence alone is never treated as durable publication.
	file, err := openInitializationFile(f.directory, f.name, false)
	if err != nil {
		return errors.New("credentials_file_unsafe")
	}
	defer file.Close()
	if err = checkInitializationFile(file); err != nil {
		return err
	}
	data, err := io.ReadAll(io.LimitReader(file, 4097))
	if err != nil || len(data) > 4096 {
		return errors.New("credentials_file_invalid")
	}
	actual, err := decodePlatformAdminCredentials(data)
	if err != nil || actual != expected {
		return errors.New("credentials_file_conflict")
	}
	if err = f.syncFile(file); err != nil {
		return errors.New("credentials_file_sync_failed")
	}
	// Verify the path still names the pinned directory before committing a
	// receipt for it. The operator controls this trusted parent directory.
	live, err := os.Stat(filepath.Dir(f.path))
	pinned, statErr := f.directory.Stat()
	if err != nil || statErr != nil || !os.SameFile(live, pinned) {
		return errors.New("credentials_directory_changed")
	}
	if err = f.syncFile(f.directory); err != nil {
		return errors.New("credentials_directory_sync_failed")
	}
	return nil
}
