package main

import (
	"errors"
	"os"
)

// Offline server maintenance requires a Unix host with private durable files.
func openInitializationDirectory(string) (*os.File, error) {
	return nil, errors.New("unsupported_credentials_filesystem")
}
func openInitializationFile(*os.File, string, bool) (*os.File, error) {
	return nil, errors.New("unsupported_credentials_filesystem")
}
func checkInitializationFile(*os.File) error { return errors.New("unsupported_credentials_filesystem") }
func publishInitializationFile(*os.File, string, string) error {
	return errors.New("unsupported_credentials_filesystem")
}
func removeInitializationFile(*os.File, string) error {
	return errors.New("unsupported_credentials_filesystem")
}
