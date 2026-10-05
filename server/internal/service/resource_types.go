package service

import (
	"context"
	"net/http"
	"time"
)

type Resource struct {
	Kind          string     `json:"kind"`
	Key           string     `json:"key"`
	Name          string     `json:"name"`
	Description   string     `json:"description"`
	Source        string     `json:"source"`
	State         string     `json:"state"`
	Version       string     `json:"version"`
	ContentDigest *string    `json:"content_digest"`
	FileCount     int        `json:"file_count"`
	ByteCount     int64      `json:"byte_count"`
	UpdatedAt     *time.Time `json:"updated_at"`
	UpdatedBy     *string    `json:"updated_by"`
}

type ResourceFile struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}
type ResourcePreview struct {
	Resource        Resource       `json:"resource"`
	Files           []ResourceFile `json:"files"`
	Preview         string         `json:"preview"`
	PreviewDigest   string         `json:"preview_digest"`
	ExpectedVersion *string        `json:"expected_version"`
}
type ResourceMutationResult struct {
	Resource    Resource `json:"resource"`
	Replayed    bool     `json:"replayed"`
	OperationID string   `json:"operation_id"`
}
type ResourceMutation struct {
	Kind, Key, Filename                                                          string
	Data                                                                         []byte
	PreviewDigest, ExpectedVersion, Reason, ActorID, OrganizationID, OperationID string
}

// ResourceCommitGuard must authorize again while the filesystem lock is held.
// apply is the only filesystem commit point and must be called at most once.
type ResourceCommitGuard func(context.Context, func() (ResourceMutationResult, error)) (ResourceMutationResult, error)

type ResourcePublisher struct {
	Root, SkillDirectory, McpDirectory string
	AllowHTTP                          bool
	OrganizationID                     func(context.Context) (string, error)
}

type ResourceError struct {
	Code, Message, OperationID string
	Status                     int
}

func (e *ResourceError) Error() string { return e.Message }
func resourceError(code string) *ResourceError {
	status := http.StatusServiceUnavailable
	message := "The resource store is unavailable"
	switch code {
	case "resource_invalid":
		status = 400
		message = "The resource package or request is invalid"
	case "resource_not_found":
		status = 404
		message = "The resource or operation was not found"
	case "resource_changed":
		status = 409
		message = "The resource changed; preview the current version"
	case "resource_conflict":
		status = 409
		message = "The resource key conflicts with an existing catalog entry"
	case "resource_preview_changed":
		status = 409
		message = "The uploaded package does not match the preview"
	case "resource_idempotency_conflict":
		status = 409
		message = "The operation key was used for a different request"
	case "resource_publishing_disabled":
		message = "Resource publishing is not configured"
	case "resource_store_full":
		message = "The resource store has reached its capacity"
	case "resource_outcome_unknown":
		message = "The operation outcome is uncertain; look up its operation ID"
	}
	return &ResourceError{Code: code, Message: message, Status: status}
}

type ResourceLimits struct {
	MaxUploadBytes     int64 `json:"max_upload_bytes"`
	MaxPrimaryBytes    int64 `json:"max_primary_bytes"`
	MaxFileBytes       int64 `json:"max_file_bytes"`
	MaxSupportingBytes int64 `json:"max_supporting_bytes"`
	MaxTotalBytes      int64 `json:"max_total_bytes"`
	MaxFiles           int   `json:"max_files"`
	MaxArchiveEntries  int   `json:"max_archive_entries"`
	MaxMcpBytes        int64 `json:"max_mcp_bytes"`
	MaxManagedEntries  int   `json:"max_managed_entries"`
	MaxActiveBytes     int64 `json:"max_active_bytes"`
	MaxIndexBytes      int64 `json:"max_index_bytes"`
	MaxMcpEntries      int   `json:"max_mcp_entries"`
	MaxMcpCatalogBytes int64 `json:"max_mcp_catalog_bytes"`
}

func ResourcePublishingLimits() ResourceLimits {
	return ResourceLimits{16 << 20, 1 << 20, 1 << 20, 8 << 20, 9 << 20, 257, 512, 64 << 10, 256, 64 << 20, 16 << 20, 256, 4 << 20}
}
