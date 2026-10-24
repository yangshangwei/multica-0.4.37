//go:build !linux && !darwin

package collaborationrun

import "os"

func openBundleSource(string) (*os.File, error) { return nil, ErrUnsupported }
