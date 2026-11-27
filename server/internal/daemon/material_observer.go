package daemon

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/material"
	"github.com/multica-ai/multica/server/internal/materialgit"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// materialCLITransport delegates authentication to the host's hosting CLI. Its
// input comes exclusively from the read-only provider, never model arguments.
type materialCLITransport struct {
	executable, provider, host string
	env                        []string
}
type materialBoundedBuffer struct {
	bytes.Buffer
	limit int
}

func (b *materialBoundedBuffer) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.Len() {
		return 0, errors.New("material_response_too_large")
	}
	return b.Buffer.Write(p)
}
func (t materialCLITransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Scheme != "https" || r.URL.User != nil || r.URL.Host != t.host {
		return nil, errors.New("material_observer_host_refused")
	}
	if r.Method != http.MethodGet && !(t.provider == "github" && r.Method == http.MethodPost && r.URL.Path == "/graphql") {
		return nil, errors.New("material_observer_write_refused")
	}
	host := t.host
	endpoint := r.URL.RequestURI()
	if t.provider == "github" {
		if host != "api.github.com" {
			return nil, errors.New("material_observer_host_refused")
		}
		host = "github.com"
	} else {
		endpoint = strings.TrimPrefix(endpoint, "/api/v4/")
	}
	args := []string{"api", endpoint, "--hostname", host, "--include", "--method", r.Method}
	for _, name := range []string{"Accept", "Content-Type", "X-GitHub-Api-Version"} {
		if value := r.Header.Get(name); value != "" {
			args = append(args, "--header", name+": "+value)
		}
	}
	if r.Body != nil {
		args = append(args, "--input", "-")
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, t.executable, args...)
	cmd.Stdin = r.Body
	cmd.Env = t.env
	output := &materialBoundedBuffer{limit: 9 << 20}
	cmd.Stdout = output
	cmd.Stderr = io.Discard
	err := cmd.Run()
	// Hosting CLIs exit nonzero for HTTP errors, but their bounded status/header
	// output still lets the provider classify 404/403 without leaking stderr.
	reader := bufio.NewReader(bytes.NewReader(output.Bytes()))
	response, parseErr := http.ReadResponse(reader, r)
	if parseErr != nil {
		if err != nil {
			return nil, errors.New("material_host_cli_unavailable")
		}
		return nil, errors.New("material_host_response_invalid")
	}
	if err != nil && response.StatusCode < 400 {
		return nil, errors.New("material_host_cli_failed")
	}
	// CLI stdout is already decoded HTTP content, and glab may pretty-print
	// JSON. Original wire framing (chunking/length/compression) must not be
	// applied a second time to that output.
	response.Body = io.NopCloser(reader)
	response.ContentLength = -1
	response.TransferEncoding = nil
	response.Header.Del("Content-Length")
	response.Header.Del("Transfer-Encoding")
	response.Header.Del("Content-Encoding")
	return response, nil
}
func materialObserverEnvironment() []string {
	var env []string
	for _, v := range os.Environ() {
		name, _, _ := strings.Cut(v, "=")
		switch name {
		case "GH_DEBUG", "GLAB_DEBUG_HTTP", "GITLAB_API_HOST", "GITLAB_API_PROTOCOL", "GITLAB_HOST", "GITLAB_URI", "GITLAB_URL", "GH_HOST", "GH_PROMPT_DISABLED", "GLAB_CHECK_UPDATE", "NO_COLOR", "CLICOLOR_FORCE":
			continue
		}
		env = append(env, v)
	}
	return append(env, "GH_PROMPT_DISABLED=1", "GLAB_CHECK_UPDATE=false", "NO_COLOR=1", "CLICOLOR_FORCE=0")
}
func newMaterialObserver(e protocol.MaterialExecution) (materialgit.Observer, error) {
	name := "gh"
	if e.Provider == "gitlab" {
		name = "glab"
	}
	instance := e.InstanceURL
	if e.Provider == "github" && instance == "https://github.com" {
		instance = "https://api.github.com"
	}
	if instance == "" {
		instance = "https://api.github.com"
		if e.Provider == "gitlab" {
			instance = "https://gitlab.com"
		}
	}
	u, err := url.Parse(instance)
	if err != nil || u.Host == "" {
		return nil, errors.New("material_instance_invalid")
	}
	// glab's API endpoint is relative to its configured /api/v4 base. A
	// prefixed instance cannot be inferred from --hostname without trusting a
	// separate host CLI API-URL override, so reject it explicitly.
	if e.Provider == "gitlab" && u.Path != "" && u.Path != "/" {
		return nil, errors.New("material_instance_prefix_unsupported")
	}
	executable, err := exec.LookPath(name)
	if err != nil {
		return nil, errors.New("material_host_cli_missing")
	}
	return materialgit.NewObserver(e.Provider, instance, e.Repository, materialCLITransport{executable: executable, provider: e.Provider, host: u.Host, env: materialObserverEnvironment()})
}
func readMaterialObservation(ctx context.Context, e protocol.MaterialExecution, b protocol.MaterialClaimBinding, reader materialgit.Observer) (protocol.MaterialObservation, error) {
	if e.TargetKind == "repository_initialization" {
		return readRepositoryObservation(ctx, e, b, reader)
	}
	o := protocol.MaterialObservation{ProtocolVersion: 1, OperationID: e.OperationID, TaskID: e.TaskID, RuntimeID: e.RuntimeID, ClaimGeneration: b.ClaimGeneration, HostChallenge: b.HostChallenge, ExpectedDigest: e.ExpectedDigest, ObservationID: uuid.NewString(), ObservedAt: time.Now().UTC(), Outcome: "observed", RepositoryIdentity: e.Provider + ":" + e.InstanceURL + ":" + e.Repository, Files: []protocol.MaterialFile{}}
	if e.RepositoryExecution != nil && len(e.RepositoryExecution.ReadyProof) > 0 {
		var err error
		o.RulesSnapshot, err = readMaterialRules(ctx, e, reader)
		if err != nil {
			return o, err
		}
	}
	commit := e.BaselineCommit
	if e.Action != "verify_baseline" {
		number := e.ReviewNumber
		var err error
		if number == 0 {
			number, err = reader.FindReview(ctx, e.SourceBranch, e.BaseBranch)
			if err != nil {
				var remote *materialgit.Error
				if e.Action == "observe" && e.ReconcileAction == "publish" && errors.As(err, &remote) && remote.Code == "candidate_effect_absent" {
					o.Outcome = "not_found"
					o.SourceComplete = true
					return o, nil
				}
				return o, err
			}
		}
		r, err := reader.Observe(ctx, number)
		if err != nil {
			return o, err
		}
		if r.BaseBranch != e.BaseBranch || r.Branch != e.SourceBranch {
			return o, errors.New("material_review_target_changed")
		}
		changes := make([]protocol.MaterialChange, 0, len(r.Changes))
		for _, c := range r.Changes {
			changes = append(changes, protocol.MaterialChange{Path: c.Path, PreviousPath: c.PreviousPath, Status: c.Status, Mode: c.Mode})
		}
		o.Review = &protocol.MaterialReview{Number: r.Number, URL: r.URL, SourceBranch: r.Branch, BaseBranch: r.BaseBranch, HeadSHA: r.HeadSHA, State: r.State, MergeSHA: r.MergeSHA, MergedAt: r.MergedAt, MergeActor: r.MergeActor, GatesPassed: r.GatesPassed, BlockReason: r.BlockReason, Changes: changes}
		commit = r.HeadSHA
		if r.State == "merged" {
			commit = r.MergeSHA
		}
		if e.AuditNote != nil {
			actor, err := reader.ObserveDecision(ctx, number, e.AuditNote.Marker, e.AuditNote.Body)
			if err != nil {
				var remote *materialgit.Error
				if e.Action != "observe" || e.ReconcileAction != "review_sync" || !errors.As(err, &remote) || remote.Code != "decision_note_missing" {
					return o, err
				}
				// Complete note pagination proved absence. Still read every frozen
				// file below; absent notes cannot conceal changed review contents.
				o.Outcome = "not_found"
			} else {
				o.AuditNote = &protocol.MaterialAuditNote{Marker: e.AuditNote.Marker, Body: e.AuditNote.Body, Actor: actor}
			}
		}
	}
	if !material.IsSHA(commit) {
		return o, errors.New("material_commit_invalid")
	}
	for _, f := range e.Files {
		content, err := reader.ReadFile(ctx, commit, f.Path)
		if err != nil {
			return o, err
		}
		o.Files = append(o.Files, protocol.MaterialFile{Path: f.Path, Mode: "100644", Content: content, SHA256: material.Digest(content), Commit: commit})
	}
	o.SourceComplete = true
	return o, nil
}
