package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const resourceIndexMaxBytes = 16 << 20

type resourceStored struct {
	Resource     Resource `json:"resource"`
	RevisionFile string   `json:"revision_file"`
	Reason       string   `json:"reason"`
	OperationID  string   `json:"operation_id"`
}
type resourceReceipt struct {
	ActorID        string                 `json:"actor_id"`
	OrganizationID string                 `json:"organization_id"`
	RequestDigest  string                 `json:"request_digest"`
	Result         ResourceMutationResult `json:"result"`
	At             time.Time              `json:"at"`
}
type resourceIndex struct {
	SchemaVersion  int                        `json:"schema_version"`
	OrganizationID string                     `json:"organization_id"`
	Entries        map[string]resourceStored  `json:"entries"`
	Receipts       map[string]resourceReceipt `json:"receipts"`
}

func newResourceIndex(org string) *resourceIndex {
	return &resourceIndex{1, org, map[string]resourceStored{}, map[string]resourceReceipt{}}
}
func resourceIdentity(kind, key string) string { return kind + "/" + key }
func validResourceKind(kind string) bool       { return kind == "skill" || kind == "mcp" }

func (p *ResourcePublisher) openRoot() (*os.Root, error) {
	if p == nil || p.Root == "" {
		return nil, resourceError("resource_publishing_disabled")
	}
	name := filepath.Clean(p.Root)
	before, err := os.Lstat(name)
	if err != nil || !before.IsDir() {
		return nil, resourceError("resource_store_unavailable")
	}
	root, err := openMcpTemplateDirectory(nil, name)
	if err != nil {
		return nil, resourceError("resource_store_unavailable")
	}
	opened, err := root.Stat(".")
	after, afterErr := os.Lstat(name)
	if err != nil || afterErr != nil || !after.IsDir() || !os.SameFile(before, opened) || !os.SameFile(before, after) {
		_ = root.Close()
		return nil, resourceError("resource_store_unavailable")
	}
	return root, nil
}

func readResourceFile(root *os.Root, name string, limit int64) ([]byte, error) {
	before, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() || before.Size() > limit {
		return nil, resourceError("resource_store_unavailable")
	}
	file, err := resourceOpenFile(root, name, mcpManifestOpenFlags(), 0)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	current, err := file.Stat()
	after, afterErr := root.Lstat(name)
	if err != nil || afterErr != nil || !current.Mode().IsRegular() || !after.Mode().IsRegular() || (name != "index.json" && (!os.SameFile(before, current) || !os.SameFile(before, after))) {
		return nil, resourceError("resource_store_unavailable")
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil || int64(len(data)) > limit {
		return nil, resourceError("resource_store_unavailable")
	}
	return data, nil
}

func readResourceIndex(root *os.Root, org string) (*resourceIndex, error) {
	if strings.TrimSpace(org) == "" {
		return nil, resourceError("resource_store_unavailable")
	}
	data, err := readResourceFile(root, "index.json", resourceIndexMaxBytes)
	if errors.Is(err, os.ErrNotExist) {
		// A revisions directory proves initialization completed. Losing its index
		// must never resurrect an empty catalog or discard operation receipts.
		if _, err = root.Lstat("revisions"); !errors.Is(err, os.ErrNotExist) {
			return nil, resourceError("resource_store_unavailable")
		}
		return newResourceIndex(org), nil
	}
	if err != nil || !utf8.Valid(data) {
		return nil, resourceError("resource_store_unavailable")
	}
	if !validResourceJSON(data) {
		return nil, resourceError("resource_store_unavailable")
	}
	index := &resourceIndex{}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(index); err != nil || index.SchemaVersion != 1 || index.OrganizationID != org || index.Entries == nil || index.Receipts == nil || len(index.Entries) > 256 {
		return nil, resourceError("resource_store_unavailable")
	}
	if err = decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return nil, resourceError("resource_store_unavailable")
	}
	var active int64
	for id, entry := range index.Entries {
		r := entry.Resource
		if !validResourceMetadata(r) || id != resourceIdentity(r.Kind, r.Key) || !validResourceUUID(entry.RevisionFile) || !validResourceUUID(entry.OperationID) || strings.TrimSpace(entry.Reason) == "" || len(entry.Reason) > 2000 {
			return nil, resourceError("resource_store_unavailable")
		}
		receipt, ok := index.Receipts[entry.OperationID]
		rowJSON, _ := json.Marshal(r)
		receiptJSON, _ := json.Marshal(receipt.Result.Resource)
		if !ok || !bytes.Equal(rowJSON, receiptJSON) {
			return nil, resourceError("resource_store_unavailable")
		}
		if r.State == "published" {
			active += r.ByteCount
		}
	}
	if active > 64<<20 {
		return nil, resourceError("resource_store_unavailable")
	}
	for id, receipt := range index.Receipts {
		if !validResourceUUID(id) || receipt.Result.OperationID != id || receipt.Result.Replayed || receipt.OrganizationID != org || receipt.ActorID == "" || !validResourceDigest(receipt.RequestDigest) || !validResourceMetadata(receipt.Result.Resource) || *receipt.Result.Resource.UpdatedBy != receipt.ActorID || receipt.At.IsZero() || !receipt.At.Equal(*receipt.Result.Resource.UpdatedAt) {
			return nil, resourceError("resource_store_unavailable")
		}
	}
	return index, nil
}

func validResourceMetadata(r Resource) bool {
	if !validResourceKind(r.Kind) || !validResourceKey(r.Key) || r.Source != "managed" || (r.State != "published" && r.State != "withdrawn") || !validResourceUUID(r.Version) || r.ContentDigest == nil || !validResourceDigest(*r.ContentDigest) || r.UpdatedAt == nil || r.UpdatedAt.IsZero() || r.UpdatedBy == nil || *r.UpdatedBy == "" || len(*r.UpdatedBy) > 128 || r.FileCount < 1 || r.FileCount > 257 || r.ByteCount < 1 || r.ByteCount > 9<<20 {
		return false
	}
	if r.Kind == "mcp" && (r.FileCount != 1 || r.ByteCount > mcpManifestMaxBytes) {
		return false
	}
	return r.Name != "" && len(r.Name) <= 8192 && len(r.Description) <= 8192 && utf8.ValidString(r.Name) && utf8.ValidString(r.Description) && !strings.ContainsRune(r.Name, 0) && !strings.ContainsRune(r.Description, 0)
}

// JSON's ordinary struct decoder accepts duplicate fields. Index corruption
// must fail closed, including ambiguous receipts and organization bindings.
func validResourceJSON(data []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value func(int) bool
	value = func(depth int) bool {
		if depth > 24 {
			return false
		}
		token, err := decoder.Token()
		if err != nil {
			return false
		}
		delim, ok := token.(json.Delim)
		if !ok {
			return true
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for decoder.More() {
				key, err := decoder.Token()
				name, ok := key.(string)
				if err != nil || !ok || seen[name] {
					return false
				}
				seen[name] = true
				if !value(depth + 1) {
					return false
				}
			}
			end, err := decoder.Token()
			return err == nil && end == json.Delim('}')
		case '[':
			for decoder.More() {
				if !value(depth + 1) {
					return false
				}
			}
			end, err := decoder.Token()
			return err == nil && end == json.Delim(']')
		default:
			return false
		}
	}
	if !value(0) {
		return false
	}
	_, err := decoder.Token()
	return errors.Is(err, io.EOF)
}
func validResourceUUID(s string) bool {
	id, err := uuid.Parse(s)
	return err == nil && id.String() == s && id != uuid.Nil
}
func validResourceDigest(s string) bool {
	if len(s) != 71 || !strings.HasPrefix(s, "sha256:") {
		return false
	}
	_, err := hex.DecodeString(s[7:])
	return err == nil
}

func (p *ResourcePublisher) snapshot(ctx context.Context, org string) (*resourceIndex, error) {
	if err := ctx.Err(); err != nil {
		return nil, resourceError("resource_store_unavailable")
	}
	root, err := p.openRoot()
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return readResourceIndex(root, org)
}

func (p *ResourcePublisher) Preview(ctx context.Context, kind, key, filename string, data []byte, org string) (ResourcePreview, error) {
	if p == nil || p.Root == "" {
		return ResourcePreview{}, resourceError("resource_publishing_disabled")
	}
	bundle, err := validateResource(kind, key, filename, data, p.AllowHTTP)
	if err != nil {
		return ResourcePreview{}, err
	}
	index, err := p.snapshot(ctx, org)
	if err != nil {
		return ResourcePreview{}, err
	}
	if err = p.checkCollision(kind, key, index); err != nil {
		return ResourcePreview{}, err
	}
	preview := ResourcePreview{Resource: bundleResource(kind, key, bundle), Files: make([]ResourceFile, 0, len(bundle.Files)), PreviewDigest: bundle.Digest}
	if existing, ok := index.Entries[resourceIdentity(kind, key)]; ok {
		version := existing.Resource.Version
		preview.ExpectedVersion = &version
		preview.Resource.Version = version
	}
	for _, f := range bundle.Files {
		preview.Files = append(preview.Files, ResourceFile{f.Path, int64(len(f.Content))})
		if f.Path == "SKILL.md" || f.Path == "mcp.json" {
			preview.Preview = f.Content
		}
	}
	return preview, nil
}
func bundleResource(kind, key string, bundle resourceBundle) Resource {
	digest := bundle.Digest
	return Resource{Kind: kind, Key: key, Name: bundle.Name, Description: bundle.Description, Source: "managed", State: "published", ContentDigest: &digest, FileCount: len(bundle.Files), ByteCount: bundle.Bytes}
}

func (p *ResourcePublisher) Publish(ctx context.Context, request ResourceMutation, guard ResourceCommitGuard) (ResourceMutationResult, error) {
	return p.mutate(ctx, "publish", request, guard)
}
func (p *ResourcePublisher) Withdraw(ctx context.Context, request ResourceMutation, guard ResourceCommitGuard) (ResourceMutationResult, error) {
	return p.mutate(ctx, "withdraw", request, guard)
}

func (p *ResourcePublisher) mutate(ctx context.Context, action string, request ResourceMutation, guard ResourceCommitGuard) (ResourceMutationResult, error) {
	var empty ResourceMutationResult
	if p == nil || p.Root == "" {
		return empty, resourceError("resource_publishing_disabled")
	}
	if !validResourceKind(request.Kind) || !validResourceKey(request.Key) || !validResourceUUID(request.OperationID) || request.ActorID == "" || len(request.ActorID) > 128 || request.OrganizationID == "" || len(request.OrganizationID) > 128 || strings.TrimSpace(request.Reason) == "" || len(request.Reason) > 2000 || !utf8.ValidString(request.Reason) || strings.ContainsRune(request.Reason, 0) || guard == nil {
		return empty, resourceError("resource_invalid")
	}
	var bundle resourceBundle
	if action == "publish" {
		var err error
		bundle, err = validateResource(request.Kind, request.Key, request.Filename, request.Data, p.AllowHTTP)
		if err != nil {
			return empty, err
		}
		if request.PreviewDigest != bundle.Digest {
			return empty, resourceError("resource_preview_changed")
		}
	} else if request.ExpectedVersion == "" {
		return empty, resourceError("resource_invalid")
	}
	root, err := p.openRoot()
	if err != nil {
		return empty, err
	}
	defer root.Close()
	lock, err := acquireResourceLock(ctx, root)
	if err != nil {
		return empty, err
	}
	defer func() { _ = resourceUnlock(lock); _ = lock.Close() }()
	index, err := readResourceIndex(root, request.OrganizationID)
	if err != nil {
		return empty, err
	}
	payload, _ := json.Marshal([]string{action, request.Kind, request.Key, request.ExpectedVersion, bundle.Digest, request.Reason})
	sum := sha256.Sum256(payload)
	requestDigest := "sha256:" + hex.EncodeToString(sum[:])
	called := false
	committed := false
	var committedResult ResourceMutationResult
	result, guardErr := guard(ctx, func() (ResourceMutationResult, error) {
		if called {
			return empty, resourceError("resource_invalid")
		}
		called = true
		if err := ctx.Err(); err != nil {
			return empty, resourceError("resource_store_unavailable")
		}
		if prior, ok := index.Receipts[request.OperationID]; ok {
			if prior.ActorID != request.ActorID || prior.OrganizationID != request.OrganizationID || prior.RequestDigest != requestDigest {
				return empty, resourceError("resource_idempotency_conflict")
			}
			replay := prior.Result
			replay.Replayed = true
			committed, committedResult = true, replay
			return replay, nil
		}
		if err := p.checkCollision(request.Kind, request.Key, index); err != nil {
			return empty, err
		}
		id := resourceIdentity(request.Kind, request.Key)
		previous, exists := index.Entries[id]
		if exists && previous.Resource.Version != request.ExpectedVersion || !exists && request.ExpectedVersion != "" {
			return empty, resourceError("resource_changed")
		}
		if action == "withdraw" && (!exists || previous.Resource.State != "published") {
			return empty, resourceError("resource_changed")
		}
		if !exists && len(index.Entries) >= 256 {
			return empty, resourceError("resource_store_full")
		}
		now := time.Now().UTC()
		version := uuid.NewString()
		row := previous
		if action == "publish" {
			row.Resource = bundleResource(request.Kind, request.Key, bundle)
			row.RevisionFile = version
		} else {
			row.Resource.State = "withdrawn"
		}
		row.Resource.Version = version
		row.Resource.UpdatedAt = &now
		row.Resource.UpdatedBy = &request.ActorID
		row.Reason = request.Reason
		row.OperationID = request.OperationID
		index.Entries[id] = row
		var active int64
		for _, entry := range index.Entries {
			if entry.Resource.State == "published" {
				active += entry.Resource.ByteCount
			}
		}
		if active > 64<<20 {
			return empty, resourceError("resource_store_full")
		}
		if err := p.checkMcpCapacity(index); err != nil {
			return empty, err
		}
		response := ResourceMutationResult{Resource: row.Resource, OperationID: request.OperationID}
		index.Receipts[request.OperationID] = resourceReceipt{request.ActorID, request.OrganizationID, requestDigest, response, now}
		encoded, err := json.Marshal(index)
		if err != nil || len(encoded) > resourceIndexMaxBytes {
			return empty, resourceError("resource_store_full")
		}
		if action == "publish" {
			if _, err := root.Lstat("index.json"); errors.Is(err, os.ErrNotExist) {
				initial, _ := json.Marshal(newResourceIndex(request.OrganizationID))
				if _, err := writeResourceIndex(root, initial); err != nil {
					return empty, resourceError("resource_store_unavailable")
				}
			}
			if err := writeResourceRevision(root, row.RevisionFile, bundle); err != nil {
				return empty, err
			}
		}
		renamed, err := writeResourceIndex(root, encoded)
		if renamed {
			committed = true
			committedResult = response
		}
		if err != nil {
			if renamed {
				unknown := resourceError("resource_outcome_unknown")
				unknown.OperationID = request.OperationID
				return response, unknown
			}
			return empty, resourceError("resource_store_unavailable")
		}
		return response, nil
	})
	if committed && guardErr != nil {
		unknown := resourceError("resource_outcome_unknown")
		unknown.OperationID = request.OperationID
		return committedResult, unknown
	}
	if guardErr == nil && !called {
		return empty, resourceError("resource_store_unavailable")
	}
	return result, guardErr
}

func acquireResourceLock(ctx context.Context, root *os.Root) (*os.File, error) {
	file, err := resourceOpenFile(root, ".lock", os.O_CREATE|os.O_RDWR|resourceWriteFlags(), 0600)
	// Some filesystems can report a transient ENOENT when two confined opens
	// race to create the same file. Reopen the stable inode; never replace it.
	for attempts := 0; errors.Is(err, os.ErrNotExist) && attempts < 3 && ctx.Err() == nil; attempts++ {
		file, err = resourceOpenFile(root, ".lock", os.O_CREATE|os.O_RDWR|resourceWriteFlags(), 0600)
	}
	if err != nil {
		return nil, resourceError("resource_store_unavailable")
	}
	before, err := root.Lstat(".lock")
	opened, statErr := file.Stat()
	if err != nil || statErr != nil || !before.Mode().IsRegular() || !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		_ = file.Close()
		return nil, resourceError("resource_store_unavailable")
	}
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			_ = file.Close()
			return nil, resourceError("resource_store_unavailable")
		}
		locked, err := resourceLock(file)
		if err != nil {
			_ = file.Close()
			return nil, resourceError("resource_store_unavailable")
		}
		if locked {
			current, err := root.Lstat(".lock")
			if err != nil || !current.Mode().IsRegular() || !os.SameFile(opened, current) {
				_ = resourceUnlock(file)
				_ = file.Close()
				return nil, resourceError("resource_store_unavailable")
			}
			return file, nil
		}
		select {
		case <-ctx.Done():
			_ = file.Close()
			return nil, resourceError("resource_store_unavailable")
		case <-timer.C:
			_ = file.Close()
			return nil, resourceError("resource_store_unavailable")
		case <-ticker.C:
		}
	}
}

func syncResourceDirectory(root *os.Root) error {
	directory, err := root.Open(".")
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}
func writeResourceIndex(root *os.Root, data []byte) (bool, error) {
	name := ".index-" + uuid.NewString() + ".tmp"
	file, err := resourceOpenFile(root, name, os.O_WRONLY|os.O_CREATE|os.O_EXCL|resourceWriteFlags(), 0600)
	if err != nil {
		return false, err
	}
	// Only this unpublished temporary is cleaned up; immutable revisions remain
	// available to readers that already captured an older index snapshot.
	defer root.Remove(name)
	_, writeErr := file.Write(data)
	syncErr := file.Sync()
	closeErr := file.Close()
	if writeErr != nil || syncErr != nil || closeErr != nil {
		return false, resourceError("resource_store_unavailable")
	}
	if err = root.Rename(name, "index.json"); err != nil {
		return false, err
	}
	return true, syncResourceDirectory(root)
}

func writeResourceRevision(root *os.Root, revision string, bundle resourceBundle) error {
	if _, err := root.Lstat("revisions"); errors.Is(err, os.ErrNotExist) {
		if err = root.Mkdir("revisions", 0700); err != nil {
			return resourceError("resource_store_unavailable")
		}
	}
	before, err := root.Lstat("revisions")
	if err != nil || !before.IsDir() {
		return resourceError("resource_store_unavailable")
	}
	revisions, err := openMcpTemplateDirectory(root, "revisions")
	if err != nil {
		return resourceError("resource_store_unavailable")
	}
	defer revisions.Close()
	opened, err := revisions.Stat(".")
	if err != nil || !os.SameFile(before, opened) {
		return resourceError("resource_store_unavailable")
	}
	data, err := json.Marshal(bundle)
	if err != nil {
		return resourceError("resource_store_unavailable")
	}
	file, err := resourceOpenFile(revisions, revision+".json", os.O_WRONLY|os.O_CREATE|os.O_EXCL|resourceWriteFlags(), 0600)
	if err != nil {
		return resourceError("resource_store_unavailable")
	}
	_, writeErr := file.Write(data)
	syncErr := file.Sync()
	closeErr := file.Close()
	if writeErr != nil || syncErr != nil || closeErr != nil {
		return resourceError("resource_store_unavailable")
	}
	if err = syncResourceDirectory(revisions); err != nil {
		return resourceError("resource_store_unavailable")
	}
	return nil
}

func (p *ResourcePublisher) Receipt(ctx context.Context, org, actor, operation string) (ResourceMutationResult, error) {
	index, err := p.snapshot(ctx, org)
	if err != nil {
		return ResourceMutationResult{}, err
	}
	receipt, ok := index.Receipts[operation]
	if !ok || receipt.ActorID != actor || receipt.OrganizationID != org {
		return ResourceMutationResult{}, resourceError("resource_not_found")
	}
	result := receipt.Result
	result.Replayed = true
	return result, nil
}

func sortedResourceRows(index *resourceIndex, kind string) []Resource {
	result := []Resource{}
	for _, entry := range index.Entries {
		if entry.Resource.Kind == kind {
			result = append(result, entry.Resource)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Key < result[j].Key })
	return result
}
