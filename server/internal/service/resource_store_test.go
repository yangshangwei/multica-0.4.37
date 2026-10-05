package service

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func resourcePublisherFixture(t *testing.T) *ResourcePublisher {
	t.Helper()
	return &ResourcePublisher{Root: t.TempDir(), OrganizationID: func(context.Context) (string, error) { return "organization", nil }}
}
func resourcePublishRequest(t *testing.T, p *ResourcePublisher) ResourceMutation {
	t.Helper()
	preview, err := p.Preview(context.Background(), "skill", "example", "SKILL.md", resourceSkillText(), "organization")
	if err != nil {
		t.Fatal(err)
	}
	expected := ""
	if preview.ExpectedVersion != nil {
		expected = *preview.ExpectedVersion
	}
	return ResourceMutation{Kind: "skill", Key: "example", Filename: "SKILL.md", Data: resourceSkillText(), PreviewDigest: preview.PreviewDigest, ExpectedVersion: expected, Reason: "reviewed", ActorID: "actor", OrganizationID: "organization", OperationID: uuid.NewString()}
}

func TestResourceStoreRevisionCASAndDurableReplay(t *testing.T) {
	ctx := context.Background()
	p := resourcePublisherFixture(t)
	first := resourcePublishRequest(t, p)
	a, err := p.Publish(ctx, first, resourceTestGuard)
	if err != nil {
		t.Fatal(err)
	}
	next := resourcePublishRequest(t, p)
	b, err := p.Publish(ctx, next, resourceTestGuard)
	if err != nil {
		t.Fatal(err)
	}
	if a.Resource.Version == b.Resource.Version || *a.Resource.ContentDigest != *b.Resource.ContentDigest {
		t.Fatal("mutation revision must differ from content digest")
	}
	stale := first
	stale.OperationID = uuid.NewString()
	_, err = p.Publish(ctx, stale, resourceTestGuard)
	resourceErrorCode(t, err, "resource_changed")
	restarted := &ResourcePublisher{Root: p.Root}
	replayed, err := restarted.Publish(ctx, first, resourceTestGuard)
	if err != nil || !replayed.Replayed || replayed.Resource.Version != a.Resource.Version {
		t.Fatalf("lost original receipt: %+v %v", replayed, err)
	}
	lookup, err := restarted.Receipt(ctx, "organization", "actor", first.OperationID)
	if err != nil || lookup.Resource.Version != a.Resource.Version {
		t.Fatalf("receipt lookup: %+v %v", lookup, err)
	}
	_, err = restarted.Receipt(ctx, "organization", "another actor", first.OperationID)
	resourceErrorCode(t, err, "resource_not_found")
	changed := first
	changed.Reason = "different"
	_, err = p.Publish(ctx, changed, resourceTestGuard)
	resourceErrorCode(t, err, "resource_idempotency_conflict")
	withdrawn, err := p.Withdraw(ctx, ResourceMutation{Kind: "skill", Key: "example", ExpectedVersion: b.Resource.Version, Reason: "retired", ActorID: "actor", OrganizationID: "organization", OperationID: uuid.NewString()}, resourceTestGuard)
	if err != nil {
		t.Fatal(err)
	}
	if withdrawn.Resource.State != "withdrawn" || withdrawn.Resource.Version == b.Resource.Version {
		t.Fatal("withdraw must create mutation revision")
	}
}

func TestResourceStoreGuardScopeAndCorruption(t *testing.T) {
	ctx := context.Background()
	p := resourcePublisherFixture(t)
	request := resourcePublishRequest(t, p)
	_, err := p.Publish(ctx, request, func(context.Context, func() (ResourceMutationResult, error)) (ResourceMutationResult, error) {
		return ResourceMutationResult{}, os.ErrPermission
	})
	if err == nil {
		t.Fatal("guard rejection ignored")
	}
	rows, err := p.List(ctx, "skill", "organization")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.Source == "managed" {
			t.Fatal("guard failure changed index")
		}
	}
	_, err = p.Publish(ctx, request, resourceTestGuard)
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.List(ctx, "skill", "different organization")
	resourceErrorCode(t, err, "resource_store_unavailable")
	if err = os.WriteFile(filepath.Join(p.Root, "index.json"), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err = p.List(ctx, "skill", "organization")
	resourceErrorCode(t, err, "resource_store_unavailable")
}

func TestResourceStoreConcurrentCAS(t *testing.T) {
	ctx := context.Background()
	p := resourcePublisherFixture(t)
	a := resourcePublishRequest(t, p)
	b := a
	b.OperationID = uuid.NewString()
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, r := range []ResourceMutation{a, b} {
		wg.Go(func() { _, err := p.Publish(ctx, r, resourceTestGuard); results <- err })
	}
	wg.Wait()
	close(results)
	success, conflict := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else {
			resourceErrorCode(t, err, "resource_changed")
			conflict++
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatal("CAS did not serialize writers")
	}
}

func TestResourceStoreLockWaitIsBounded(t *testing.T) {
	p := resourcePublisherFixture(t)
	request := resourcePublishRequest(t, p)
	entered := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan error, 1)
	go func() {
		_, err := p.Publish(context.Background(), request, func(_ context.Context, apply func() (ResourceMutationResult, error)) (ResourceMutationResult, error) {
			close(entered)
			<-release
			return apply()
		})
		finished <- err
	}()
	<-entered
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	other := request
	other.OperationID = uuid.NewString()
	_, err := p.Publish(ctx, other, resourceTestGuard)
	resourceErrorCode(t, err, "resource_store_unavailable")
	close(release)
	if err = <-finished; err != nil {
		t.Fatal(err)
	}
}

func TestResourceStoreRefusesSymlinksAndManualCollisions(t *testing.T) {
	p := resourcePublisherFixture(t)
	p.SkillDirectory = t.TempDir()
	if err := os.Mkdir(filepath.Join(p.SkillDirectory, "EXAMPLE"), 0700); err != nil {
		t.Fatal(err)
	}
	_, err := p.Preview(context.Background(), "skill", "example", "SKILL.md", resourceSkillText(), "organization")
	resourceErrorCode(t, err, "resource_conflict")
	p.SkillDirectory = ""
	request := resourcePublishRequest(t, p)
	_, err = p.Publish(context.Background(), request, resourceTestGuard)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(filepath.Join(p.Root, "index.json"), filepath.Join(p.Root, "elsewhere.json")); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink("elsewhere.json", filepath.Join(p.Root, "index.json")); err != nil {
		t.Fatal(err)
	}
	_, err = p.List(context.Background(), "skill", "organization")
	resourceErrorCode(t, err, "resource_store_unavailable")
}

func TestResourceStoreRejectsMissingAndDuplicateIndexFields(t *testing.T) {
	for _, data := range []string{`{"schema_version":1}`, `{"schema_version":1,"schema_version":1,"organization_id":"organization","entries":{},"receipts":{}}`} {
		p := resourcePublisherFixture(t)
		if err := os.WriteFile(filepath.Join(p.Root, "index.json"), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		_, err := p.List(context.Background(), "skill", "organization")
		resourceErrorCode(t, err, "resource_store_unavailable")
	}
}

func TestResourceStoreAuditFinalizeFailureHasRecoverableReceipt(t *testing.T) {
	p := resourcePublisherFixture(t)
	request := resourcePublishRequest(t, p)
	result, err := p.Publish(context.Background(), request, func(_ context.Context, apply func() (ResourceMutationResult, error)) (ResourceMutationResult, error) {
		result, err := apply()
		if err != nil {
			return result, err
		}
		return result, os.ErrPermission
	})
	resourceErrorCode(t, err, "resource_outcome_unknown")
	if result.OperationID != request.OperationID {
		t.Fatal("unknown outcome lost operation identity")
	}
	receipt, err := p.Receipt(context.Background(), "organization", "actor", request.OperationID)
	if err != nil || receipt.Resource.Version != result.Resource.Version {
		t.Fatalf("receipt failed to resolve committed outcome: %+v %v", receipt, err)
	}
}

func TestResourceStoreReplayStillUsesGuard(t *testing.T) {
	p := resourcePublisherFixture(t)
	request := resourcePublishRequest(t, p)
	if _, err := p.Publish(context.Background(), request, resourceTestGuard); err != nil {
		t.Fatal(err)
	}
	_, err := p.Publish(context.Background(), request, func(context.Context, func() (ResourceMutationResult, error)) (ResourceMutationResult, error) {
		return ResourceMutationResult{}, os.ErrPermission
	})
	if !errors.Is(err, os.ErrPermission) {
		t.Fatal("replay bypassed authorization")
	}
}

func TestResourceStoreReadersSeeCompleteSnapshotsDuringWrites(t *testing.T) {
	p := resourcePublisherFixture(t)
	request := resourcePublishRequest(t, p)
	result, err := p.Publish(context.Background(), request, resourceTestGuard)
	if err != nil {
		t.Fatal(err)
	}
	errorsCh := make(chan error, 1)
	done := make(chan struct{})
	var readers sync.WaitGroup
	for range 4 {
		readers.Go(func() {
			for {
				select {
				case <-done:
					return
				default:
				}
				_, err := p.snapshot(context.Background(), "organization")
				if err != nil {
					select {
					case errorsCh <- err:
					default:
					}
					return
				}
			}
		})
	}
	for range 15 {
		request.ExpectedVersion = result.Resource.Version
		request.OperationID = uuid.NewString()
		result, err = p.Publish(context.Background(), request, resourceTestGuard)
		if err != nil {
			close(done)
			readers.Wait()
			t.Fatal(err)
		}
	}
	close(done)
	readers.Wait()
	select {
	case err := <-errorsCh:
		t.Fatalf("concurrent index replacement broke reader: %v", err)
	default:
	}
}
