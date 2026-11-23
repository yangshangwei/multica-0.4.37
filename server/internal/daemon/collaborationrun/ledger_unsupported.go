//go:build !linux && !darwin && !windows

package collaborationrun

import "os"

func ownedByCurrentUser(os.FileInfo) bool            { return false }
func openPrivateFile(string) (*os.File, error)       { return nil, ErrUnsupported }
func lockPrivateFile(string) (*os.File, error)       { return nil, ErrUnsupported }
func unlockPrivateFile(*os.File) error               { return ErrUnsupported }
func createPrivateDirectory(string) error            { return ErrUnsupported }
func privateDirectoryOwner(string, os.FileInfo) bool { return false }
func protectNewPrivateFile(*os.File) error           { return ErrUnsupported }
func replacePrivateFile(string, string) error        { return ErrUnsupported }
func syncDirectory(string) error                     { return ErrUnsupported }
