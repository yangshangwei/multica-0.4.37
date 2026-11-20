package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
)

const collaborationOutputMaxBytes = 100 << 20

type CollaborationOutputFile struct {
	Name string
	Data []byte
}

// The current CLI process directory is the available file anchor. OpenRoot
// pins it across path resolution, including symlinks and directory replacement.
func ReadCollaborationOutputFile(filePath string) (CollaborationOutputFile, error) {
	var out CollaborationOutputFile
	if filePath == "" || strings.Contains(filePath, "://") || strings.HasPrefix(filePath, "~") || strings.ContainsRune(filePath, 0) {
		return out, errors.New("--attachment requires one local regular file inside the current working directory")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return out, fmt.Errorf("read current working directory: %w", err)
	}
	absolute, err := filepath.Abs(filePath)
	if err != nil {
		return out, fmt.Errorf("resolve --attachment: %w", err)
	}
	relative, err := filepath.Rel(cwd, absolute)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return out, errors.New("--attachment must stay inside the current working directory")
	}
	root, err := os.OpenRoot(cwd)
	if err != nil {
		return out, fmt.Errorf("open current working directory: %w", err)
	}
	defer root.Close()
	// O_NONBLOCK prevents a substituted FIFO from blocking before Stat can
	// reject it. Regular file reads are unchanged by this flag.
	file, err := root.OpenFile(relative, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return out, fmt.Errorf("open contained --attachment: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return out, errors.New("--attachment requires a regular file inside the current working directory")
	}
	if info.Size() < 0 || info.Size() > collaborationOutputMaxBytes {
		return out, errors.New("--attachment exceeds the 100 MiB output limit")
	}
	data, err := io.ReadAll(io.LimitReader(file, collaborationOutputMaxBytes+1))
	if err != nil || int64(len(data)) != info.Size() || len(data) > collaborationOutputMaxBytes {
		return out, errors.New("--attachment changed or could not be read within the 100 MiB output limit")
	}
	return CollaborationOutputFile{Name: filepath.Base(filePath), Data: data}, nil
}

type CollaborationRuntimeOutputAttachment struct {
	ID                string  `json:"id"`
	DraftID           *string `json:"draft_id"`
	IssueID           string  `json:"issue_id"`
	IntendedIssueID   *string `json:"intended_issue_id"`
	SourceWorkspaceID string  `json:"source_workspace_id"`
	UploaderUserID    *string `json:"uploader_user_id"`
	UploaderAgentID   string  `json:"uploader_agent_id"`
	RunID             string  `json:"run_id"`
	AttemptSeq        string  `json:"attempt_seq"`
	Name              string  `json:"name"`
	ContentType       string  `json:"content_type"`
	SizeBytes         int64   `json:"size_bytes"`
	Version           string  `json:"version"`
	CreatedAt         string  `json:"created_at"`
	DeliveredAt       string  `json:"delivered_at"`
}

type CollaborationRuntimeCommentAttachmentReceipt struct {
	CommentID         string                                `json:"comment_id"`
	RunIDs            []string                              `json:"run_ids"`
	ContinuationRunID *string                               `json:"continuation_run_id"`
	Attachment        *CollaborationRuntimeOutputAttachment `json:"attachment"`
}

func canonicalOutputUUID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id != uuid.Nil && id.String() == value
}

func decodeCollaborationOutputReceipt(raw []byte, c *CollaborationContext, requestID, agentID string, file CollaborationOutputFile) (CollaborationRuntimeCommentAttachmentReceipt, error) {
	var out CollaborationRuntimeCommentAttachmentReceipt
	invalid := errors.New("invalid collaboration output receipt")
	fields, err := collaborationObjectFields(string(raw))
	if err != nil {
		return out, invalid
	}
	for _, name := range []string{"comment_id", "run_ids", "continuation_run_id", "attachment"} {
		if _, present := fields[name]; !present {
			return out, invalid
		}
	}
	attachmentFields, err := collaborationObjectFields(string(fields["attachment"]))
	if err != nil {
		return out, invalid
	}
	for _, name := range []string{"id", "draft_id", "issue_id", "intended_issue_id", "source_workspace_id", "uploader_user_id", "uploader_agent_id", "run_id", "attempt_seq", "name", "content_type", "size_bytes", "version", "created_at", "delivered_at"} {
		if _, present := attachmentFields[name]; !present {
			return out, invalid
		}
	}
	if bytes.Equal(bytes.TrimSpace(attachmentFields["size_bytes"]), []byte("null")) {
		return out, invalid
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&out) != nil || out.Attachment == nil || out.CommentID != requestID || out.RunIDs == nil {
		return out, invalid
	}
	a := out.Attachment
	seq, err := strconv.ParseInt(a.AttemptSeq, 10, 64)
	if err != nil || seq < 1 || strconv.FormatInt(seq, 10) != a.AttemptSeq || !canonicalOutputUUID(a.ID) || !canonicalOutputUUID(a.SourceWorkspaceID) || !canonicalOutputUUID(agentID) || a.IssueID != c.Identity.IssueID || a.RunID != c.Identity.RunID || a.UploaderAgentID != agentID || a.UploaderUserID != nil || a.DraftID != nil || a.IntendedIssueID != nil || a.Version != "1" || a.Name != file.Name || a.ContentType != http.DetectContentType(file.Data) || a.SizeBytes != int64(len(file.Data)) {
		return out, invalid
	}
	if _, err = time.Parse(time.RFC3339Nano, a.CreatedAt); err != nil {
		return out, invalid
	}
	if _, err = time.Parse(time.RFC3339Nano, a.DeliveredAt); err != nil {
		return out, invalid
	}
	seen := make(map[string]bool, len(out.RunIDs))
	for _, id := range out.RunIDs {
		if !canonicalOutputUUID(id) || seen[id] {
			return out, invalid
		}
		seen[id] = true
	}
	if out.ContinuationRunID != nil && !canonicalOutputUUID(*out.ContinuationRunID) {
		return out, invalid
	}
	return out, nil
}

func (c *CollaborationContext) CreateCommentAttachment(ctx context.Context, client *APIClient, requestID, parentID, content, agentID string, file CollaborationOutputFile) (CollaborationRuntimeCommentAttachmentReceipt, error) {
	var empty CollaborationRuntimeCommentAttachmentReceipt
	if !canonicalOutputUUID(requestID) || parentID != "" && !canonicalOutputUUID(parentID) || !canonicalOutputUUID(agentID) || file.Name == "" || filepath.Base(file.Name) != file.Name || len(file.Data) > collaborationOutputMaxBytes {
		return empty, errors.New("invalid collaboration output request")
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, field := range []struct{ name, value string }{{"comment_id", requestID}, {"content", content}, {"parent_id", parentID}} {
		if field.name == "parent_id" && field.value == "" {
			continue
		}
		if err := writer.WriteField(field.name, field.value); err != nil {
			return empty, err
		}
	}
	part, err := writer.CreateFormFile("file", file.Name)
	if err != nil {
		return empty, err
	}
	if _, err = part.Write(file.Data); err != nil {
		return empty, err
	}
	if err = writer.Close(); err != nil {
		return empty, err
	}
	path := c.Path("/comments/attachments")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, client.BaseURL+path, bytes.NewReader(body.Bytes()))
	if err != nil {
		return empty, err
	}
	req.GetBody = nil
	req.Header.Set("Content-Type", writer.FormDataContentType())
	client.setHeaders(req)
	response, err := client.HTTPClient.Do(req)
	err = wrapTransport(req, err)
	if err != nil {
		return empty, CollaborationError(err, requestID)
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusMultipleChoices && response.StatusCode < http.StatusBadRequest {
		// Refusing a redirect confirms no file receipt. Keep the original write
		// unconfirmed rather than treating the 3xx as a settled HTTP refusal.
		return empty, CollaborationError(fmt.Errorf("collaboration output redirect refused: HTTP %d", response.StatusCode), requestID)
	}
	if response.StatusCode != http.StatusCreated {
		return empty, CollaborationError(newHTTPError(http.MethodPost, path, response), requestID)
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	if err != nil || len(raw) > 1<<20 {
		return empty, CollaborationError(errors.New("invalid collaboration output receipt"), requestID)
	}
	out, err := decodeCollaborationOutputReceipt(raw, c, requestID, agentID, file)
	if err != nil {
		return empty, CollaborationError(err, requestID)
	}
	return out, nil
}
