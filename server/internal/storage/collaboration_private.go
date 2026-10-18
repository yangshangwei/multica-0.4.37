package storage

import (
	"path"
	"strings"
)

// CollaborationPrivatePrefix contains encrypted, account-authorized objects.
// It is never a public object URL or a namespace for ordinary attachments.
const CollaborationPrivatePrefix = "private/collaboration/"

func IsCollaborationPrivateKey(key string) bool {
	clean := strings.TrimPrefix(path.Clean("/"+strings.ReplaceAll(key, "\\", "/")), "/")
	return clean == "private/collaboration" || strings.HasPrefix(clean, CollaborationPrivatePrefix)
}
