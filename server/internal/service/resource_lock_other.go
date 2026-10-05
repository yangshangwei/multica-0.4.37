//go:build !unix && !windows

package service

import "os"

func resourceLock(*os.File) (bool, error) { return false, resourceError("resource_store_unavailable") }
func resourceUnlock(*os.File) error       { return nil }
func resourceWriteFlags() int             { return 0 }

func resourceOpenFile(root *os.Root, name string, flags int, mode os.FileMode) (*os.File, error) {
	return nil, resourceError("resource_store_unavailable")
}
