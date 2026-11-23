package artifact

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	StagingRoot string
	OutputRoot  string
	Identity    string
}

type Outcome struct {
	Status       string `json:"status"`
	Code         string `json:"code"`
	OutputDigest string `json:"output_digest,omitempty"`
	ArtifactID   string `json:"artifact_id,omitempty"`
	OutputSize   int64  `json:"output_size,omitempty"`
}

// Packager pins trusted root identities. It never obtains HTTP or account data.
type Packager struct {
	config   Config
	identity string
	// fault is an internal I/O boundary seam; it is nil in production.
	fault func(string) error
}

func New(config Config) (*Packager, error) {
	if config.Identity == "" || len(config.Identity) > 128 || strings.ContainsAny(config.Identity, "\x00\r\n") || !filepath.IsAbs(config.StagingRoot) || !filepath.IsAbs(config.OutputRoot) || filepath.Clean(config.StagingRoot) != config.StagingRoot || filepath.Clean(config.OutputRoot) != config.OutputRoot {
		return nil, ErrDisabled
	}
	for _, pair := range [][2]string{{config.StagingRoot, config.OutputRoot}, {config.OutputRoot, config.StagingRoot}} {
		relative, err := filepath.Rel(pair[0], pair[1])
		if err != nil || relative == "." || relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return nil, ErrDisabled
		}
	}
	p := &Packager{config: config}
	roots, err := p.openRoots()
	if err != nil {
		return nil, ErrDisabled
	}
	defer roots.close()
	p.identity = roots.identity
	return p, nil
}

func (p *Packager) step(name string) error {
	if p.fault != nil {
		return p.fault(name)
	}
	return nil
}

func (p *Packager) Prepare(ctx context.Context, r Request, b Binding, createdAt time.Time) (Spec, error) {
	if p == nil {
		return Spec{}, ErrDisabled
	}
	if err := p.ValidateStaging(ctx, r, b); err != nil {
		return Spec{}, err
	}
	return makeSpec(r, b, p.identity, createdAt)
}

func (p *Packager) ValidateStaging(ctx context.Context, r Request, b Binding) error {
	if p == nil {
		return ErrDisabled
	}
	r, err := NormalizeRequest(r)
	if err != nil {
		return err
	}
	if _, err = RequestDigest(r, b); err != nil {
		return err
	}
	roots, err := p.openRoots()
	if err != nil {
		return ErrDisabled
	}
	defer roots.close()
	if roots.identity != p.identity {
		return ErrInvalid
	}
	_, err = p.capture(ctx, roots, r, b)
	if err != nil {
		return err
	}
	return roots.recheck()
}

func (p *Packager) capture(ctx context.Context, roots *rootPair, r Request, b Binding) ([][]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	data, err := roots.staging.read(r.StagingID+"/manifest.json", MaxRequestBytes, p.fault)
	if err != nil {
		return nil, ErrInvalid
	}
	var manifest StagingManifest
	if err := decodeStrict(data, &manifest); err != nil {
		return nil, err
	}
	if err := requireKeys(data, "workspace_id", "agent_id", "task_id", "task_generation", "resource_id", "source_revision", "input_manifest_digest"); err != nil {
		return nil, err
	}
	digest, err := StagingDigest(manifest)
	if err != nil || digest != r.StagingManifestDigest {
		return nil, ErrInvalid
	}
	expected := StagingManifest{WorkspaceID: b.WorkspaceID, AgentID: r.AgentID, TaskID: r.TaskID, TaskGeneration: b.TaskGeneration, ResourceID: r.ResourceID, SourceRevision: r.SourceRevision, InputManifestDigest: r.InputManifestDigest}
	if manifest != expected {
		return nil, ErrInvalid
	}
	snapshots := make([][]byte, 0, len(r.Entries))
	for _, entry := range r.Entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		data, err := roots.staging.read(r.StagingID+"/files/"+entry.Path, entry.SizeBytes, p.fault)
		if err != nil || int64(len(data)) != entry.SizeBytes {
			return nil, ErrInvalid
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != entry.SHA256 {
			return nil, ErrInvalid
		}
		snapshots = append(snapshots, data)
	}
	return snapshots, nil
}

// Pack may be invoked only by the owner of a definitely committed unknown claim.
// It neither retries nor interprets existing output as proof of a previous success.
func (p *Packager) Pack(ctx context.Context, executionID string, spec Spec) (out Outcome) {
	out = Outcome{Status: "unknown", Code: "interrupted"}
	defer func() {
		if recover() != nil {
			out = Outcome{Status: "unknown", Code: "adapter_panic"}
		}
	}()
	if p == nil {
		return Outcome{Status: "failed", Code: "adapter_disabled"}
	}
	if !validUUID(executionID) {
		return Outcome{Status: "failed", Code: "invalid_execution"}
	}
	canonical, _, err := CanonicalSpec(spec)
	if err != nil {
		return Outcome{Status: "failed", Code: "invalid_spec"}
	}
	// A caller cannot exploit an unsorted in-memory spec to pair hashes with wrong files.
	spec, err = ParseSpec(canonical)
	if err != nil {
		return Outcome{Status: "failed", Code: "invalid_spec"}
	}
	ctx, cancel := context.WithTimeout(ctx, ExecutionTimeout)
	defer cancel()
	if ctx.Err() != nil {
		return out
	}
	roots, err := p.openRoots()
	if err != nil {
		return Outcome{Status: "failed", Code: "roots_unavailable"}
	}
	defer roots.close()
	if roots.identity != p.identity || spec.RootIdentity != p.identity {
		return Outcome{Status: "failed", Code: "root_changed"}
	}
	snapshots, err := p.capture(ctx, roots, spec.Request, spec.Binding)
	if err != nil {
		if ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return out
		}
		return Outcome{Status: "failed", Code: "input_invalid"}
	}
	if err = p.step("snapshot"); err != nil {
		return Outcome{Status: "failed", Code: "snapshot_refused"}
	}
	if ctx.Err() != nil {
		return out
	}
	if err = roots.recheck(); err != nil {
		return Outcome{Status: "failed", Code: "root_changed"}
	}
	output, err := roots.output.createOutput(executionID)
	if err != nil {
		if output != nil {
			output.close()
			return Outcome{Status: "unknown", Code: "output_creation_uncertain"}
		}
		return Outcome{Status: "failed", Code: "output_collision"}
	}
	defer output.close()
	// Once an output exists, uncertainty defaults to unknown. Failed is possible
	// only after a proven prepublication error and successful durable cleanup.
	cleanFailure := func(code string) Outcome {
		if ctx.Err() != nil {
			return Outcome{Status: "unknown", Code: "deadline"}
		}
		if p.step("cleanup") != nil || output.cleanup() != nil {
			return Outcome{Status: "unknown", Code: "cleanup_uncertain"}
		}
		return Outcome{Status: "failed", Code: code}
	}
	writer := tar.NewWriter(output.file)
	for i, entry := range spec.Request.Entries {
		if ctx.Err() != nil {
			return Outcome{Status: "unknown", Code: "deadline"}
		}
		if err = p.step("write"); err != nil {
			return cleanFailure("write_refused")
		}
		header := &tar.Header{Name: entry.Path, Size: entry.SizeBytes, Mode: 0644, Typeflag: tar.TypeReg, ModTime: time.Unix(0, 0), Format: tar.FormatUSTAR}
		if err = writer.WriteHeader(header); err != nil {
			return cleanFailure("archive_failed")
		}
		// These are the exact bytes whose approved hashes were checked above.
		hash := sha256.New()
		if _, err = io.Copy(io.MultiWriter(writer, hash), bytes.NewReader(snapshots[i])); err != nil {
			return cleanFailure("archive_failed")
		}
		if hex.EncodeToString(hash.Sum(nil)) != entry.SHA256 {
			return cleanFailure("snapshot_changed")
		}
	}
	if err = writer.Close(); err != nil {
		return cleanFailure("archive_failed")
	}
	if p.step("sync_partial") != nil || output.file.Sync() != nil {
		return Outcome{Status: "unknown", Code: "sync_uncertain"}
	}
	if err = output.file.Close(); err != nil {
		return Outcome{Status: "unknown", Code: "close_uncertain"}
	}
	output.file = nil
	before, size, err := output.digest("artifact.partial")
	if err != nil {
		return Outcome{Status: "unknown", Code: "reopen_uncertain"}
	}
	if ctx.Err() != nil {
		return Outcome{Status: "unknown", Code: "deadline"}
	}
	if err = roots.recheck(); err != nil {
		return Outcome{Status: "unknown", Code: "root_changed"}
	}
	if p.step("publish") != nil {
		return Outcome{Status: "unknown", Code: "publication_uncertain"}
	}
	if err = output.publish(); err != nil {
		return Outcome{Status: "unknown", Code: "publication_uncertain"}
	}
	if p.step("sync_final") != nil || output.sync() != nil {
		return Outcome{Status: "unknown", Code: "sync_uncertain"}
	}
	after, finalSize, err := output.digest("artifact.tar")
	if err != nil || after != before || size != finalSize {
		return Outcome{Status: "unknown", Code: "verification_uncertain"}
	}
	if ctx.Err() != nil {
		return Outcome{Status: "unknown", Code: "deadline"}
	}
	if err = roots.recheck(); err != nil {
		return Outcome{Status: "unknown", Code: "root_changed"}
	}
	return Outcome{Status: "succeeded", Code: "packaged", OutputDigest: after, ArtifactID: executionID, OutputSize: size}
}

// RootIdentity is an opaque hash of the operator configuration and pinned roots.
func (p *Packager) RootIdentity() string {
	if p == nil {
		return ""
	}
	return p.identity
}

// Keep os in the shared contract so unsupported-platform roots have the same
// lifetime and file types as the Unix implementation.
type rootPair struct {
	staging, output *secureRoot
	identity        string
}

func (r *rootPair) close() { r.staging.close(); r.output.close() }
func (r *rootPair) recheck() error {
	if err := r.staging.recheck(); err != nil {
		return err
	}
	return r.output.recheck()
}

func (p *Packager) openRoots() (*rootPair, error) {
	staging, err := openSecureRoot(p.config.StagingRoot, 0500)
	if err != nil {
		return nil, err
	}
	output, err := openSecureRoot(p.config.OutputRoot, 0700)
	if err != nil {
		staging.close()
		return nil, err
	}
	if os.SameFile(staging.info, output.info) {
		staging.close()
		output.close()
		return nil, ErrInvalid
	}
	identity, err := domainDigest("multica-artifact-roots-v1", struct{ Config, Staging, Output string }{p.config.Identity, staging.identity, output.identity})
	if err != nil {
		staging.close()
		output.close()
		return nil, err
	}
	return &rootPair{staging: staging, output: output, identity: identity}, nil
}
