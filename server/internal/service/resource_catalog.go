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
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/multica-ai/multica/server/internal/skill"
)

func (p *ResourcePublisher) consumerOrganization(ctx context.Context) (string, error) {
	if p.OrganizationID == nil {
		return "", resourceError("resource_store_unavailable")
	}
	org, err := p.OrganizationID(ctx)
	if err != nil || org == "" {
		return "", resourceError("resource_store_unavailable")
	}
	return org, nil
}

func resourceManualNames(directory string, limit int) ([]string, error) {
	if directory == "" {
		return nil, nil
	}
	name := filepath.Clean(directory)
	before, err := os.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil || !before.IsDir() {
		return nil, resourceError("resource_store_unavailable")
	}
	root, err := openMcpTemplateDirectory(nil, name)
	if err != nil {
		return nil, resourceError("resource_store_unavailable")
	}
	defer root.Close()
	opened, err := root.Stat(".")
	if err != nil || !os.SameFile(before, opened) {
		return nil, resourceError("resource_store_unavailable")
	}
	directoryFile, err := root.Open(".")
	if err != nil {
		return nil, resourceError("resource_store_unavailable")
	}
	defer directoryFile.Close()
	names, err := directoryFile.Readdirnames(limit + 1)
	if err != nil && !errors.Is(err, io.EOF) || len(names) > limit {
		return nil, resourceError("resource_store_unavailable")
	}
	return names, nil
}

func (p *ResourcePublisher) reservedKeys(kind string) (map[string]bool, error) {
	directory, limit := p.SkillDirectory, 4096
	if kind == "mcp" {
		directory, limit = p.McpDirectory, mcpCatalogMaxEntries
	}
	names, err := resourceManualNames(directory, limit)
	if err != nil {
		return nil, err
	}
	reserved := make(map[string]bool, len(names))
	for _, name := range names {
		reserved[strings.ToLower(name)] = true
	}
	if kind == "skill" {
		for _, builtin := range RoleSkillTemplates() {
			reserved[strings.ToLower(builtin.Name)] = true
		}
		for _, builtin := range loadBuiltinSkills() {
			reserved[strings.ToLower(builtin.Name)] = true
		}
	}
	return reserved, nil
}

func (p *ResourcePublisher) checkCollision(kind, key string, index *resourceIndex) error {
	reserved, err := p.reservedKeys(kind)
	if err != nil {
		return err
	}
	return checkResourceCollision(kind, key, index, reserved)
}

func checkResourceCollision(kind, key string, index *resourceIndex, reserved map[string]bool) error {
	if reserved[strings.ToLower(key)] {
		return resourceError("resource_conflict")
	}
	for _, entry := range index.Entries {
		if entry.Resource.Kind == kind && entry.Resource.Key != key && strings.EqualFold(entry.Resource.Key, key) {
			return resourceError("resource_conflict")
		}
	}
	return nil
}

func (p *ResourcePublisher) checkMcpCapacity(index *resourceIndex) error {
	names, err := resourceManualNames(p.McpDirectory, mcpCatalogMaxEntries)
	if err != nil {
		return err
	}
	count := len(names)
	var size int64
	if len(names) > 0 {
		root, err := openMcpTemplateDirectory(nil, filepath.Clean(p.McpDirectory))
		if err != nil {
			return resourceError("resource_store_unavailable")
		}
		defer root.Close()
		remaining := mcpCatalogMaxBytes
		for _, name := range names {
			if !mcpTemplateName.MatchString(name) || len(name) > 128 {
				continue
			}
			_, readErr := readMcpManifestFile(root, name, &remaining)
			if errors.Is(readErr, ErrMcpCatalogUnavailable) {
				return resourceError("resource_store_full")
			}
		}
		size = int64(mcpCatalogMaxBytes - remaining)
	}
	for _, entry := range index.Entries {
		if entry.Resource.Kind == "mcp" && entry.Resource.State == "published" {
			count++
			size += entry.Resource.ByteCount
		}
	}
	if count > mcpCatalogMaxEntries || size > mcpCatalogMaxBytes {
		return resourceError("resource_store_full")
	}
	return nil
}

func (p *ResourcePublisher) managedBundles(ctx context.Context, kind, org string) (*resourceIndex, map[string]resourceBundle, error) {
	root, err := p.openRoot()
	if err != nil {
		return nil, nil, err
	}
	defer root.Close()
	index, err := readResourceIndex(root, org)
	if err != nil {
		return nil, nil, err
	}
	reserved, err := p.reservedKeys(kind)
	if err != nil {
		return nil, nil, err
	}
	bundles := map[string]resourceBundle{}
	for _, entry := range index.Entries {
		if entry.Resource.Kind != kind {
			continue
		}
		if err = checkResourceCollision(kind, entry.Resource.Key, index, reserved); err != nil {
			return nil, nil, err
		}
		if entry.Resource.State != "published" {
			continue
		}
		if ctx.Err() != nil {
			return nil, nil, resourceError("resource_store_unavailable")
		}
		bundle, err := p.readRevision(root, entry)
		if err != nil {
			return nil, nil, err
		}
		bundles[entry.Resource.Key] = bundle
	}
	if kind == "mcp" {
		if err = p.checkMcpCapacity(index); err != nil {
			return nil, nil, err
		}
	}
	return index, bundles, nil
}

func (p *ResourcePublisher) readRevision(root *os.Root, entry resourceStored) (resourceBundle, error) {
	invalid := func() (resourceBundle, error) { return resourceBundle{}, resourceError("resource_store_unavailable") }
	before, err := root.Lstat("revisions")
	if err != nil || !before.IsDir() {
		return invalid()
	}
	revisions, err := openMcpTemplateDirectory(root, "revisions")
	if err != nil {
		return invalid()
	}
	defer revisions.Close()
	opened, err := revisions.Stat(".")
	after, afterErr := root.Lstat("revisions")
	if err != nil || afterErr != nil || !after.IsDir() || !os.SameFile(before, opened) || !os.SameFile(before, after) {
		return invalid()
	}
	data, err := readResourceFile(revisions, entry.RevisionFile+".json", 64<<20)
	if err != nil || !validResourceJSON(data) {
		return invalid()
	}
	var bundle resourceBundle
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&bundle); err != nil {
		return invalid()
	}
	if err = decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return invalid()
	}
	if len(bundle.Files) < 1 || len(bundle.Files) > 257 {
		return invalid()
	}
	aliases := map[string]string{}
	seen := map[string]bool{}
	supporting := int64(0)
	primary := ""
	for i, f := range bundle.Files {
		if !portableResourcePath(f.Path) || seen[f.Path] || len(f.Content) > 1<<20 || !utf8.ValidString(f.Content) || strings.ContainsRune(f.Content, 0) || (i > 0 && bundle.Files[i-1].Path >= f.Path) {
			return invalid()
		}
		seen[f.Path] = true
		for name := f.Path; name != "."; name = path.Dir(name) {
			lower := strings.ToLower(name)
			if prior, ok := aliases[lower]; ok && prior != name {
				return invalid()
			}
			aliases[lower] = name
		}
		bundle.Bytes += int64(len(f.Content))
		if f.Path == "SKILL.md" {
			primary = f.Content
		} else {
			supporting += int64(len(f.Content))
		}
	}
	for name := range seen {
		for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
			if seen[parent] {
				return invalid()
			}
		}
	}
	if supporting > 8<<20 || bundle.Bytes > 9<<20 || bundle.Bytes != entry.Resource.ByteCount || len(bundle.Files) != entry.Resource.FileCount {
		return invalid()
	}
	canonical, _ := json.Marshal(bundle.Files)
	sum := sha256.Sum256(append([]byte("multica-resource-content:v1\n"+entry.Resource.Kind+"\n"), canonical...))
	bundle.Digest = "sha256:" + hex.EncodeToString(sum[:])
	if entry.Resource.ContentDigest == nil || bundle.Digest != *entry.Resource.ContentDigest {
		return invalid()
	}
	if entry.Resource.Kind == "skill" {
		fm, err := strictResourceFrontmatter(primary)
		if err != nil {
			return invalid()
		}
		bundle.Name = fm.Name
		bundle.Description = fm.Description
		bundle.Category = fm.Category
		bundle.Icon = fm.Icon
	} else {
		if len(bundle.Files) != 1 || bundle.Files[0].Path != "mcp.json" {
			return invalid()
		}
		template, err := parseMcpManifest(entry.Resource.Key, []byte(bundle.Files[0].Content), p.AllowHTTP)
		if err != nil {
			return invalid()
		}
		bundle.Name = template.Titles["en"]
		bundle.Description = template.Descriptions["en"]
	}
	if bundle.Name != entry.Resource.Name || bundle.Description != entry.Resource.Description {
		return invalid()
	}
	return bundle, nil
}

func (p *ResourcePublisher) List(ctx context.Context, kind, org string) ([]Resource, error) {
	if !validResourceKind(kind) {
		return nil, resourceError("resource_invalid")
	}
	result := []Resource{}
	snapshotKeys := map[string]bool{}
	var mcpSnapshot mcpDeploymentSnapshot
	if kind == "skill" {
		if p.Root != "" {
			names, err := resourceManualNames(p.SkillDirectory, 4096)
			if err != nil {
				return nil, err
			}
			for _, name := range names {
				snapshotKeys[strings.ToLower(name)] = true
			}
		}
		names := map[string]struct{}{}
		for _, template := range RoleSkillTemplates() {
			names[template.Name] = struct{}{}
			result = append(result, resourceSkillRow(template, "builtin"))
		}
		for _, template := range scanSkillTemplateDir(p.SkillDirectory, names) {
			result = append(result, resourceSkillRow(template, "deployment"))
			snapshotKeys[strings.ToLower(template.Name)] = true
		}
	} else {
		var err error
		mcpSnapshot, err = (McpCatalog{Directory: p.McpDirectory, AllowHTTP: p.AllowHTTP}).readDeploymentTemplates()
		if err != nil {
			return nil, resourceError("resource_store_unavailable")
		}
		builtins, err := (McpCatalog{}).List()
		if err != nil {
			return nil, resourceError("resource_store_unavailable")
		}
		for _, name := range mcpSnapshot.Names {
			snapshotKeys[strings.ToLower(name)] = true
		}
		for _, template := range append(builtins, mcpSnapshot.Templates...) {
			result = append(result, Resource{Kind: "mcp", Key: template.Key, Name: template.Titles["en"], Description: template.Descriptions["en"], Source: template.Source, State: "published", Version: template.Version})
		}
	}
	if p.Root != "" {
		index, _, err := p.managedBundles(ctx, kind, org)
		if err != nil {
			return nil, err
		}
		if err := checkResourceSnapshotCollision(index, kind, snapshotKeys); err != nil {
			return nil, err
		}
		if kind == "mcp" {
			if err := checkMcpSnapshotCapacity(index, mcpSnapshot); err != nil {
				return nil, err
			}
		}
		result = append(result, sortedResourceRows(index, kind)...)
	}
	return result, nil
}

func resourceSkillRow(template RoleSkillTemplate, source string) Resource {
	size := int64(len(template.Content))
	for _, file := range template.Files {
		size += int64(len(file.Content))
	}
	return Resource{Kind: "skill", Key: template.Name, Name: template.Name, Description: template.Description, Source: source, State: "published", Version: strconv.Itoa(int(template.Version)), FileCount: len(template.Files) + 1, ByteCount: size}
}

func (p *ResourcePublisher) skillTemplates(ctx context.Context, existing []RoleSkillTemplate, reservedNames []string) ([]RoleSkillTemplate, error) {
	org, err := p.consumerOrganization(ctx)
	if err != nil {
		return nil, err
	}
	index, bundles, err := p.managedBundles(ctx, "skill", org)
	if err != nil {
		return nil, err
	}
	snapshotKeys := map[string]bool{}
	for _, template := range existing {
		snapshotKeys[strings.ToLower(template.Name)] = true
	}
	for _, name := range reservedNames {
		snapshotKeys[strings.ToLower(name)] = true
	}
	if err := checkResourceSnapshotCollision(index, "skill", snapshotKeys); err != nil {
		return nil, err
	}
	result := []RoleSkillTemplate{}
	for key, bundle := range bundles {
		template := RoleSkillTemplate{Name: key, Description: bundle.Description, Category: bundle.Category, Icon: bundle.Icon, Version: mountedSkillTemplateVersion}
		for _, f := range bundle.Files {
			if f.Path == skill.ContentFilename {
				template.Content = f.Content
			} else {
				template.Files = append(template.Files, AgentSkillFileData{Path: f.Path, Content: f.Content})
			}
		}
		result = append(result, template)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

func (c McpCatalog) deploymentTemplates() ([]McpServerTemplate, error) {
	snapshot, err := c.readDeploymentTemplates()
	manual := snapshot.Templates
	if err != nil {
		return nil, err
	}
	if c.Publisher == nil || c.Publisher.Root == "" {
		return manual, nil
	}
	publisher := *c.Publisher
	publisher.McpDirectory = c.Directory
	publisher.AllowHTTP = c.AllowHTTP
	org, err := publisher.consumerOrganization(context.Background())
	if err != nil {
		return nil, errors.Join(ErrMcpCatalogUnavailable, err)
	}
	index, bundles, err := publisher.managedBundles(context.Background(), "mcp", org)
	if err != nil {
		return nil, errors.Join(ErrMcpCatalogUnavailable, err)
	}
	snapshotKeys := map[string]bool{}
	for _, name := range snapshot.Names {
		snapshotKeys[strings.ToLower(name)] = true
	}
	if err := checkResourceSnapshotCollision(index, "mcp", snapshotKeys); err != nil {
		return nil, errors.Join(ErrMcpCatalogUnavailable, err)
	}
	if err := checkMcpSnapshotCapacity(index, snapshot); err != nil {
		return nil, errors.Join(ErrMcpCatalogUnavailable, err)
	}
	keys := make([]string, 0, len(bundles))
	for key := range bundles {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		template, err := parseMcpManifest(key, []byte(bundles[key].Files[0].Content), c.AllowHTTP)
		if err != nil {
			return nil, ErrMcpCatalogUnavailable
		}
		manual = append(manual, template)
	}
	return manual, nil
}

func checkResourceSnapshotCollision(index *resourceIndex, kind string, keys map[string]bool) error {
	for _, entry := range index.Entries {
		if entry.Resource.Kind == kind && keys[strings.ToLower(entry.Resource.Key)] {
			return resourceError("resource_conflict")
		}
	}
	return nil
}

func checkMcpSnapshotCapacity(index *resourceIndex, snapshot mcpDeploymentSnapshot) error {
	count, size := len(snapshot.Names), snapshot.Bytes
	for _, entry := range index.Entries {
		if entry.Resource.Kind == "mcp" && entry.Resource.State == "published" {
			count++
			size += entry.Resource.ByteCount
		}
	}
	if count > mcpCatalogMaxEntries || size > mcpCatalogMaxBytes {
		return resourceError("resource_store_full")
	}
	return nil
}
