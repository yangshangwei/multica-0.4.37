package collaborationrun

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/pkg/agent"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// PreparationController performs only Go file/HTTP work before nonce burn.
// The returned root belongs to this attempt; after Start it may be removed only
// after the controller observes the complete namespace exit.
type PreparationController interface {
	Prepare(context.Context, ProviderInput, *Client) (*PreparedRun, error)
}

type PreparedRun struct {
	root      string
	execution preparedExecution
}

type preparedExecution struct {
	Entrypoint         string            `json:"entrypoint"`
	AdapterID          string            `json:"adapter_id"`
	Arguments          []string          `json:"arguments"`
	AccountEnvironment []string          `json:"account_environment"`
	Checksums          map[string]string `json:"checksums"`
	GitExecutable      string            `json:"git_executable,omitempty"`
	Devices            []string          `json:"devices,omitempty"`
	GitDevices         []string          `json:"git_devices,omitempty"`
	Deadline           time.Time         `json:"deadline"`
}

type preparationFact struct {
	Stage string `json:"stage"`
	State string `json:"state"`
	Code  string `json:"code,omitempty"`
}

// PreparationFailure exposes only a bounded stage/code. It must never include
// file paths, downloaded contents, provider stderr or account source values.
type PreparationFailure struct {
	fact  preparationFact
	cause error
}

func (e *PreparationFailure) Error() string {
	return "collaboration preparation " + e.fact.Stage + ": " + e.fact.Code
}
func (e *PreparationFailure) Unwrap() error { return e.cause }

func preparationFailure(stage, code string, err error) error {
	return &PreparationFailure{fact: preparationFact{Stage: stage, State: "failed", Code: code}, cause: err}
}

func (p *PreparedRun) discard() error {
	if p == nil || p.root == "" {
		return nil
	}
	return os.RemoveAll(p.root)
}

func (c *NamespaceController) Prepare(ctx context.Context, input ProviderInput, client *Client) (*PreparedRun, error) {
	manifest := input.Preparation
	if c.Bundle == nil || client == nil || protocol.ValidateCollaborationPreparation(manifest, input.Identity, input.Inputs) != nil {
		return nil, preparationFailure("bundle", "configuration_changed", ErrUnsupported)
	}
	caps, err := c.Bundle.Capabilities()
	if err != nil || caps.Provider != manifest.Provider || caps.Limits != manifest.Limits || !reflect.DeepEqual(caps.Git, manifest.Git) {
		return nil, preparationFailure("bundle", "configuration_changed", ErrUnsupported)
	}
	if c.Bundle.AdapterID == "claude_stream_json_v1" {
		invocation := manifest.Invocation
		if _, err := agent.ClaudeFreshStreamArguments(invocation.Instructions, invocation.Model, invocation.ThinkingLevel); err != nil {
			return nil, preparationFailure("bundle", "configuration_changed", err)
		}
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(time.Duration(manifest.Limits.TimeoutSeconds) * time.Second)
	}
	root, err := c.prepareBundleRoot(ctx, false)
	if err != nil {
		return nil, preparationFailure("bundle", "bundle_unavailable", err)
	}
	prepared := &PreparedRun{root: root, execution: preparedExecution{Entrypoint: c.Bundle.Entrypoint, AdapterID: c.Bundle.AdapterID,
		Arguments: append([]string(nil), c.Bundle.Arguments...), Checksums: make(map[string]string), Devices: append([]string(nil), c.Bundle.Devices...), Deadline: deadline}}
	finished := false
	defer func() {
		if !finished {
			_ = prepared.discard()
		}
	}()
	if err = client.CheckPreparation(ctx, *manifest, input.Inputs, input.BusinessToken); err != nil {
		return nil, preparationFailure("workspace", "configuration_changed", err)
	}
	for _, file := range manifest.Attachments {
		destination := filepath.Join(root, "inputs", file.Destination)
		if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
			return nil, preparationFailure("attachments", "attachment_unavailable", err)
		}
		f, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0400)
		if err != nil {
			return nil, preparationFailure("attachments", "attachment_unavailable", err)
		}
		hash := sha256.New()
		err = client.DownloadAttachment(ctx, *manifest, file, input.BusinessToken, io.MultiWriter(f, hash))
		err = errors.Join(err, f.Sync(), f.Close())
		if err != nil {
			return nil, preparationFailure("attachments", "attachment_unavailable", err)
		}
		prepared.execution.Checksums[file.Destination] = hex.EncodeToString(hash.Sum(nil))
	}
	if c.Bundle.Git != nil {
		git := c.Bundle.Git
		gitRoot := filepath.Join(root, ".preparation", "git-root")
		if err := createNamespaceRoot(gitRoot, git.Devices); err != nil {
			return nil, preparationFailure("git", "git_unavailable", err)
		}
		if err := snapshotBundleFiles(ctx, gitRoot, git.Files); err != nil {
			return nil, preparationFailure("git", "git_unavailable", err)
		}
		file := ProviderBundleFile{Source: git.BundleFile, Destination: "/inputs/source.bundle", SHA256: git.BundleSHA256, SizeBytes: git.SizeBytes, Mode: "readonly"}
		if err := snapshotBundleFile(ctx, file, filepath.Join(gitRoot, "inputs", "source.bundle")); err != nil {
			return nil, preparationFailure("git", "git_unavailable", err)
		}
		prepared.execution.GitExecutable = git.Executable
		prepared.execution.GitDevices = append([]string(nil), git.Devices...)
	}
	// This is the only account read; probes and capability computation never
	// reach it. Only the selected provider key enters the in-memory request.
	prepared.execution.AccountEnvironment, err = c.Bundle.accountEnvironment()
	if err != nil {
		return nil, preparationFailure("bundle", "bundle_unavailable", err)
	}
	sealed, err := json.Marshal(struct {
		Manifest  *protocol.CollaborationPreparation `json:"manifest"`
		Checksums map[string]string                  `json:"checksums"`
	}{manifest, prepared.execution.Checksums})
	if err != nil {
		return nil, preparationFailure("workspace", "preparation_unavailable", err)
	}
	if err := atomicPrivateWrite(filepath.Join(root, ".preparation", "manifest.json"), sealed); err != nil {
		return nil, preparationFailure("workspace", "preparation_unavailable", err)
	}
	if err := syncDirectory(root); err != nil {
		return nil, preparationFailure("workspace", "preparation_unavailable", err)
	}
	finished = true
	return prepared, nil
}

func createNamespaceRoot(root string, devices []string) error {
	if !validBundleDevices(devices) {
		return ErrUnsupported
	}
	if err := os.Mkdir(root, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	for _, sub := range []string{"proc", "work", "tmp", "home", "dev", "inputs", ".oldroot", ".preparation"} {
		if err := os.Mkdir(filepath.Join(root, sub), 0700); err != nil {
			return err
		}
	}
	for _, device := range append([]string{"/dev/null"}, devices...) {
		if err := os.WriteFile(filepath.Join(root, strings.TrimPrefix(device, "/")), nil, 0600); err != nil {
			return err
		}
	}
	return nil
}

func snapshotBundleFiles(ctx context.Context, root string, files []ProviderBundleFile) error {
	for _, file := range files {
		if err := snapshotBundleFile(ctx, file, filepath.Join(root, strings.TrimPrefix(file.Destination, "/"))); err != nil {
			return err
		}
	}
	return validateBundleELFs(root, files)
}

func (c *NamespaceController) prepareBundleRoot(ctx context.Context, probe bool) (string, error) {
	if c.Bundle == nil || c.Bundle.Validate() != nil || !filepath.IsAbs(c.Directory) || c.Executable != "" || c.CLIExecutable != "" || len(c.Arguments) != 0 {
		return "", ErrUnsupported
	}
	if err := privateDirectory(c.Directory); err != nil {
		return "", err
	}
	root, err := os.MkdirTemp(c.Directory, "run-*")
	if err != nil {
		return "", err
	}
	ok := false
	defer func() {
		if !ok {
			_ = os.RemoveAll(root)
		}
	}()
	if err := createNamespaceRoot(root, c.Bundle.preparationDevices()); err != nil {
		return "", err
	}
	if err := snapshotBundleFiles(ctx, root, c.Bundle.Files); err != nil {
		return "", err
	}
	if c.Bundle.Git != nil {
		helper := c.HelperExecutable
		if helper == "" {
			helper, err = os.Executable()
			if err != nil {
				return "", ErrUnsupported
			}
		}
		f, err := openStaticExecutable(helper)
		if err != nil {
			return "", err
		}
		defer f.Close()
		out, err := os.OpenFile(filepath.Join(root, ".preparation", "helper"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0500)
		if err != nil {
			return "", err
		}
		_, copyErr := io.Copy(out, contextReader{ctx, f})
		if err := errors.Join(copyErr, out.Sync(), out.Close()); err != nil {
			return "", err
		}
		if probe {
			// A nested containment probe has an empty root. It must not open the
			// configured Git bundle, account or any business input.
			if err := createNamespaceRoot(filepath.Join(root, ".preparation", "git-root"), c.Bundle.Git.Devices); err != nil {
				return "", err
			}
		}
	}
	ok = true
	return root, nil
}

func (r *Runner) recordPreparationFact(ctx context.Context, claim protocol.CollaborationRunClaimResponse, permit protocol.CollaborationRunPermitResponse, fact preparationFact) error {
	if claim.Preparation == nil {
		return ErrProtocol
	}
	nonce, err := uuid.Parse(permit.AttemptNonce)
	if err != nil {
		return ErrProtocol
	}
	eventID := uuid.NewSHA1(nonce, []byte("collaboration-preparation/"+fact.Stage+"/"+fact.State)).String()
	observation := protocol.CollaborationPreparationObservation{Identity: claim.Identity, EventID: eventID, AttemptSeq: permit.AttemptSeq,
		AttemptNonce: permit.AttemptNonce, ManifestDigest: claim.Preparation.ManifestDigest, Stage: fact.Stage, State: fact.State, Code: fact.Code}
	if claim.ProtocolVersion == protocol.CollaborationHostRuntimeProtocolVersion {
		observation.ProtocolVersion, observation.Containment = claim.ProtocolVersion, claim.Containment
	}
	observationCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	return r.Client.ObservePreparation(observationCtx, observation, permit.BusinessToken)
}
