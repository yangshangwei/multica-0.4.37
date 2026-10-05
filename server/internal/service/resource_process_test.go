package service

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestResourceStoreProcessHelper(t *testing.T) {
	mode := os.Getenv("MULTICA_RESOURCE_TEST_MODE")
	if mode == "" {
		return
	}
	p := &ResourcePublisher{Root: os.Getenv("MULTICA_RESOURCE_TEST_ROOT")}
	if mode == "lock" {
		root, err := p.openRoot()
		if err != nil {
			t.Fatal(err)
		}
		defer root.Close()
		lock, err := acquireResourceLock(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		defer lock.Close()
		defer resourceUnlock(lock)
		fmt.Println("locked")
		var input [1]byte
		_, _ = os.Stdin.Read(input[:])
		return
	}
	if mode == "commit" {
		preview, err := p.Preview(context.Background(), "skill", "example", "SKILL.md", resourceSkillText(), "organization")
		if err != nil {
			t.Fatal(err)
		}
		request := ResourceMutation{Kind: "skill", Key: "example", Filename: "SKILL.md", Data: resourceSkillText(), PreviewDigest: preview.PreviewDigest, Reason: "reviewed", ActorID: "actor", OrganizationID: "organization", OperationID: os.Getenv("MULTICA_RESOURCE_TEST_OPERATION")}
		_, err = p.Publish(context.Background(), request, func(_ context.Context, apply func() (ResourceMutationResult, error)) (ResourceMutationResult, error) {
			result, err := apply()
			if err != nil {
				return result, err
			}
			os.Exit(0)
			return result, nil
		})
		t.Fatalf("helper did not exit after filesystem commit: %v", err)
	}
}

func TestResourceStoreOSLockSerializesProcesses(t *testing.T) {
	p := resourcePublisherFixture(t)
	request := resourcePublishRequest(t, p)
	cmd := exec.Command(os.Args[0], "-test.run=^TestResourceStoreProcessHelper$")
	cmd.Env = append(os.Environ(), "MULTICA_RESOURCE_TEST_MODE=lock", "MULTICA_RESOURCE_TEST_ROOT="+p.Root)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = stdin.Close()
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
	scanner := bufio.NewScanner(stdout)
	if !scanner.Scan() || scanner.Text() != "locked" {
		t.Fatal("helper failed to acquire lock")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	_, err = p.Publish(ctx, request, resourceTestGuard)
	resourceErrorCode(t, err, "resource_store_unavailable")
	if err = stdin.Close(); err != nil {
		t.Fatal(err)
	}
	if err = cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	if _, err = p.Publish(context.Background(), request, resourceTestGuard); err != nil {
		t.Fatal(err)
	}
}

func TestResourceStoreReceiptSurvivesProcessExitAfterCommit(t *testing.T) {
	p := resourcePublisherFixture(t)
	operation := uuid.NewString()
	cmd := exec.Command(os.Args[0], "-test.run=^TestResourceStoreProcessHelper$")
	cmd.Env = append(os.Environ(), "MULTICA_RESOURCE_TEST_MODE=commit", "MULTICA_RESOURCE_TEST_ROOT="+p.Root, "MULTICA_RESOURCE_TEST_OPERATION="+operation)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("helper: %v %s", err, output)
	}
	result, err := p.Receipt(context.Background(), "organization", "actor", operation)
	if err != nil || result.Resource.Key != "example" {
		t.Fatalf("committed receipt unavailable after process exit: %+v %v", result, err)
	}
	if _, err := p.List(context.Background(), "skill", "organization"); err != nil {
		t.Fatal(err)
	}
}
