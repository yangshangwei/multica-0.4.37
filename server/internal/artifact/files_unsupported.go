//go:build !linux && !darwin

package artifact

import "os"

type secureRoot struct {
	info     os.FileInfo
	identity string
}

func openSecureRoot(string, os.FileMode) (*secureRoot, error)              { return nil, ErrDisabled }
func (*secureRoot) close()                                                 {}
func (*secureRoot) recheck() error                                         { return ErrDisabled }
func (*secureRoot) read(string, int64, func(string) error) ([]byte, error) { return nil, ErrDisabled }
func (*secureRoot) createOutput(string) (*artifactOutput, error)           { return nil, ErrDisabled }

type artifactOutput struct{ file *os.File }

func (*artifactOutput) close()                               {}
func (*artifactOutput) cleanup() error                       { return ErrDisabled }
func (*artifactOutput) digest(string) (string, int64, error) { return "", 0, ErrDisabled }
func (*artifactOutput) publish() error                       { return ErrDisabled }
func (*artifactOutput) sync() error                          { return ErrDisabled }
