//go:build !unix

package service

import "os"

func mcpManifestOpenFlags() int { return os.O_RDONLY }
