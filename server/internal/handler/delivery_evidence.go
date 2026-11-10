package handler

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const deliveryEvidencePolicy = service.DeliveryEvidencePolicy

type deliveryEvidenceRequest = service.DeliveryEvidenceRequest
type deliveryEvidenceResponse = service.DeliveryObservation

func deliveryHex(s string, sizes ...int) bool { return service.DeliveryHex(s, sizes...) }

func decodeDeliveryEvidence(w http.ResponseWriter, r *http.Request) (deliveryEvidenceRequest, bool) {
	var req deliveryEvidenceRequest
	var fields map[string]json.RawMessage
	if !decodeCollaborationWorkflowBodyLimit(w, r, &fields, 4096) {
		return req, false
	}
	req, valid := deliveryEvidenceFields(fields)
	if !valid {
		writeError(w, 400, "invalid_delivery_evidence_request")
	}
	return req, valid
}

// deliveryEvidenceFields is shared by the stateless and durable boundaries.
// Both require the same exact original observation contract.
func deliveryEvidenceFields(fields map[string]json.RawMessage) (deliveryEvidenceRequest, bool) {
	var req deliveryEvidenceRequest
	keys := []string{"policy_version", "expected_issue_revision", "expected_project_id", "expected_project_revision", "approval_id", "expected_source_revision", "expected_spec_digest", "expected_artifact_digest"}
	valid := len(fields) == len(keys)
	for _, key := range keys {
		value, ok := fields[key]
		valid = valid && ok
		if key != "expected_project_id" && key != "expected_project_revision" && bytes.Equal(value, []byte("null")) {
			valid = false
		}
	}
	raw, _ := json.Marshal(fields)
	if json.Unmarshal(raw, &req) != nil {
		valid = false
	}
	id, err := util.ParseUUID(req.ApprovalID)
	if err != nil || uuidToString(id) != req.ApprovalID || req.PolicyVersion != deliveryEvidencePolicy || req.IssueRevision < 0 || !deliveryHex(req.SourceRevision, 40, 64) || !deliveryHex(req.SpecDigest, 64) || !deliveryHex(req.ArtifactDigest, 64) {
		valid = false
	}
	if (req.ProjectID == nil) != (req.ProjectRevision == nil) {
		valid = false
	}
	if req.ProjectID != nil {
		id, err = util.ParseUUID(*req.ProjectID)
		if err != nil || uuidToString(id) != *req.ProjectID || req.ProjectRevision == nil || *req.ProjectRevision < 0 {
			valid = false
		}
	}
	return req, valid
}

// EvaluateDeliveryEvidence observes a locked snapshot. It never grants a reusable
// completion permission, writes a verdict, or invokes the artifact packager.
func (h *Handler) EvaluateDeliveryEvidence(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	p, kind, _, err := artifactPrincipal(r)
	if err != nil || kind != "human" {
		writeArtifactError(w, artifactDeny())
		return
	}
	req, ok := decodeDeliveryEvidence(w, r)
	if !ok {
		return
	}
	issue, ok := h.loadIssueForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	tx, q, err := h.artifactTx(r.Context())
	if err != nil {
		writeArtifactError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	out, err := h.evaluateDeliveryEvidence(r, tx, q, p, issue, req)
	if err != nil {
		writeArtifactError(w, err)
		return
	}
	// A failed commit has no usable observation, even though this is read-only.
	if err = tx.Commit(r.Context()); err != nil {
		writeArtifactError(w, err)
		return
	}
	writeJSON(w, 200, out)
}

func (h *Handler) evaluateDeliveryEvidence(r *http.Request, tx pgx.Tx, q *db.Queries, p auth.AuthenticatedPrincipal, initial db.Issue, req deliveryEvidenceRequest) (deliveryEvidenceResponse, error) {
	access := service.ArtifactAccess{Credential: artifactCredentialSnapshot(r.Context(), p), RequestedWorkspaceID: h.resolveWorkspaceID(r), Metrics: h.Metrics}
	// Supplied actor headers are deny-only and remain an HTTP boundary concern.
	// Task credentials retain their existing typed identity independent of headers.
	if r.Header.Get("X-Agent-ID") != "" || r.Header.Get("X-Task-ID") != "" {
		access.Credential.Principal.AgentActor = true
	}
	out, err := service.EvaluateDeliveryEvidenceInTx(r.Context(), tx, access, initial, req)
	return out, artifactCoreError(err)
}
