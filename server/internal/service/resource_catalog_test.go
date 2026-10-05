package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

func TestResourceCatalogSkillPublishWithdrawAndCollision(t *testing.T) {
	ctx := context.Background()
	p := resourcePublisherFixture(t)
	request := resourcePublishRequest(t, p)
	published, err := p.Publish(ctx, request, resourceTestGuard)
	if err != nil {
		t.Fatal(err)
	}
	consumer := &TaskService{ResourcePublisher: p}
	templates, err := consumer.SkillTemplates()
	if err != nil {
		t.Fatal(err)
	}
	if template, ok := templateByName(templates, "example"); !ok || template.Content != string(resourceSkillText()) {
		t.Fatal("managed skill not visible as ordinary template")
	}
	manual := t.TempDir()
	consumer.SkillTemplateDir = manual
	if err = os.Mkdir(filepath.Join(manual, "example"), 0700); err != nil {
		t.Fatal(err)
	}
	_, err = consumer.SkillTemplates()
	resourceErrorCode(t, err, "resource_conflict")
	consumer.SkillTemplateDir = ""
	_, err = p.Withdraw(ctx, ResourceMutation{Kind: "skill", Key: "example", ExpectedVersion: published.Resource.Version, Reason: "retired", ActorID: "actor", OrganizationID: "organization", OperationID: uuid.NewString()}, resourceTestGuard)
	if err != nil {
		t.Fatal(err)
	}
	templates, err = consumer.SkillTemplates()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := templateByName(templates, "example"); ok {
		t.Fatal("withdrawn skill still offered")
	}
}

func TestResourceCatalogMcpListResolveAndChangedVersion(t *testing.T) {
	ctx := context.Background()
	p := resourcePublisherFixture(t)
	data := []byte(`{"schema_version":1,"titles":{"en":"Example"},"config":{"command":"node","args":["server.js"]}}`)
	preview, err := p.Preview(ctx, "mcp", "example", "mcp.json", data, "organization")
	if err != nil {
		t.Fatal(err)
	}
	published, err := p.Publish(ctx, ResourceMutation{Kind: "mcp", Key: "example", Filename: "mcp.json", Data: data, PreviewDigest: preview.PreviewDigest, Reason: "reviewed", ActorID: "actor", OrganizationID: "organization", OperationID: uuid.NewString()}, resourceTestGuard)
	if err != nil {
		t.Fatal(err)
	}
	catalog := McpCatalog{Publisher: p}
	items, err := catalog.List()
	if err != nil {
		t.Fatal(err)
	}
	var version string
	for _, item := range items {
		if item.Key == "example" && item.Source == "deployment" {
			version = item.Version
			if item.Config != nil {
				t.Fatal("catalog leaked command recipe")
			}
		}
	}
	if version == "" {
		t.Fatal("managed MCP missing from deployment catalog")
	}
	if version == published.Resource.Version {
		t.Fatal("consumer version must remain content hash")
	}
	config, err := catalog.Resolve("deployment", "example", version, nil)
	if err != nil || config["command"] != "node" {
		t.Fatalf("resolve: %+v %v", config, err)
	}
	_, err = catalog.Resolve("deployment", "example", "changed", nil)
	if !errors.Is(err, ErrMcpTemplateChanged) {
		t.Fatalf("wrong stale-version error: %v", err)
	}
	catalog.Directory = t.TempDir()
	if err = os.Mkdir(filepath.Join(catalog.Directory, "EXAMPLE"), 0700); err != nil {
		t.Fatal(err)
	}
	_, err = catalog.Resolve("deployment", "example", version, nil)
	if !errors.Is(err, ErrMcpCatalogUnavailable) {
		t.Fatal("manual collision not surfaced by resolve")
	}
}

func TestResourceCatalogCorruptRevisionAndScopeAreExplicit(t *testing.T) {
	p := resourcePublisherFixture(t)
	request := resourcePublishRequest(t, p)
	result, err := p.Publish(context.Background(), request, resourceTestGuard)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(p.Root, "revisions", result.Resource.Version+".json"), []byte(`{"files":[{"path":"SKILL.md","content":"tampered"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	_, err = (&TaskService{ResourcePublisher: p}).SkillTemplates()
	resourceErrorCode(t, err, "resource_store_unavailable")
	p.OrganizationID = func(context.Context) (string, error) { return "other organization", nil }
	_, err = (&TaskService{ResourcePublisher: p}).SkillTemplates()
	resourceErrorCode(t, err, "resource_store_unavailable")
}

func TestResourceCatalogEnabledChecksUnreadableManualDirectoryEvenWhenEmpty(t *testing.T) {
	p := resourcePublisherFixture(t)
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := (&TaskService{SkillTemplateDir: file, ResourcePublisher: p}).SkillTemplates()
	resourceErrorCode(t, err, "resource_store_unavailable")
}

func TestResourceCatalogMcpRetainsCollisionFromManualSnapshot(t *testing.T) {
	ctx := context.Background()
	p := resourcePublisherFixture(t)
	data := []byte(`{"schema_version":1,"titles":{"en":"Example"},"config":{"command":"node"}}`)
	preview, err := p.Preview(ctx, "mcp", "example", "mcp.json", data, "organization")
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.Publish(ctx, ResourceMutation{Kind: "mcp", Key: "example", Filename: "mcp.json", Data: data, PreviewDigest: preview.PreviewDigest, Reason: "reviewed", ActorID: "actor", OrganizationID: "organization", OperationID: uuid.NewString()}, resourceTestGuard)
	if err != nil {
		t.Fatal(err)
	}
	manual := t.TempDir()
	directory := filepath.Join(manual, "example")
	if err = os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(directory, "mcp.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	p.OrganizationID = func(context.Context) (string, error) {
		if err := os.Remove(filepath.Join(directory, "mcp.json")); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(directory); err != nil {
			t.Fatal(err)
		}
		return "organization", nil
	}
	_, err = (McpCatalog{Directory: manual, Publisher: p}).List()
	if !errors.Is(err, ErrMcpCatalogUnavailable) {
		t.Fatalf("manual snapshot collision disappeared during read: %v", err)
	}
}

func TestResourceCatalogMcpSnapshotCapacity(t *testing.T) {
	index := newResourceIndex("organization")
	index.Entries["mcp/example"] = resourceStored{Resource: Resource{Kind: "mcp", State: "published", ByteCount: 1}}
	for _, test := range []struct {
		name      string
		count     int
		bytes     int64
		wantError bool
	}{
		{"entry boundary", 255, 0, false}, {"entry overflow", 256, 0, true},
		{"byte boundary", 1, (4 << 20) - 1, false}, {"byte overflow", 1, 4 << 20, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := checkMcpSnapshotCapacity(index, mcpDeploymentSnapshot{Names: make([]string, test.count), Bytes: test.bytes})
			if (err != nil) != test.wantError {
				t.Fatalf("capacity result: %v", err)
			}
		})
	}
}
