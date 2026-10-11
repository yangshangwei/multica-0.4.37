//go:build !linux && !darwin && !windows

package main

import (
	"errors"
	"os"
)

func publishInitializationFile(*os.File, string, string) error {
	return errors.New("unsupported_credentials_filesystem")
}
