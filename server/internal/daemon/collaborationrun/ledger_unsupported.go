//go:build !linux && !darwin

package collaborationrun

import "os"

func ownedByCurrentUser(os.FileInfo) bool      { return false }
func openPrivateFile(string) (*os.File, error) { return nil, ErrUnsupported }
func lockPrivateFile(string) (*os.File, error) { return nil, ErrUnsupported }
func unlockPrivateFile(*os.File) error         { return ErrUnsupported }
