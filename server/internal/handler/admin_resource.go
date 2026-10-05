package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Bound simultaneous in-memory uploads and archive validation for this process.
var resourceUploadSlots = make(chan struct{}, 4)

func resourceInvalid(message string) error {
	return &service.ResourceError{Code: "resource_invalid", Message: message, Status: http.StatusBadRequest}
}

func resourceHTTPError(w http.ResponseWriter, r *http.Request, err error) {
	var resourceErr *service.ResourceError
	if errors.As(err, &resourceErr) {
		body := map[string]string{"code": resourceErr.Code, "error": resourceErr.Message, "request_id": adminRequestID(r)}
		if resourceErr.OperationID != "" {
			body["operation_id"] = resourceErr.OperationID
		}
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, resourceErr.Status, body)
		return
	}
	var adminErr *service.PlatformAdminError
	if errors.As(err, &adminErr) || errors.Is(err, auth.ErrPasswordSession) {
		adminServiceError(w, r, err)
		return
	}
	adminError(w, r, http.StatusServiceUnavailable, "resource_store_unavailable", "The resource store is temporarily unavailable")
}

func (h *Handler) resourcePublisher() *service.ResourcePublisher {
	if h.ResourcePublisher != nil {
		return h.ResourcePublisher
	}
	return &service.ResourcePublisher{}
}

func (h *Handler) requireResourcePublishing(w http.ResponseWriter, r *http.Request) bool {
	if strings.TrimSpace(h.resourcePublisher().Root) == "" {
		adminError(w, r, 503, "resource_publishing_disabled", "Resource publishing is not configured")
		return false
	}
	return true
}

func (h *Handler) AdminResources(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.requirePlatformAccess(w, r, false)
	if !ok {
		return
	}
	publisher := h.resourcePublisher()
	items, err := publisher.List(r.Context(), r.URL.Query().Get("kind"), uuidToString(actor.OrganizationID))
	if err != nil {
		resourceHTTPError(w, r, err)
		return
	}
	enabled := strings.TrimSpace(publisher.Root) != ""
	writeJSON(w, 200, map[string]any{"enabled": enabled, "can_publish": enabled && actor.Role == service.PlatformRoleSuperAdmin, "items": items, "limits": service.ResourcePublishingLimits()})
}

func (h *Handler) resourceWriteLimit(w http.ResponseWriter, r *http.Request, actor service.PlatformAdminIdentity) bool {
	return h.passwordLimit(w, r,
		auth.PasswordLimit{Key: "resource-ip:" + auth.PasswordClientIP(r), Count: 60, Window: time.Minute},
		auth.PasswordLimit{Key: "resource-actor:" + uuidToString(actor.UserID), Count: 30, Window: time.Minute})
}

func acquireResourceUpload(w http.ResponseWriter, r *http.Request) (func(), bool) {
	select {
	case resourceUploadSlots <- struct{}{}:
		return func() { <-resourceUploadSlots }, true
	default:
		w.Header().Set("Retry-After", "1")
		adminError(w, r, 503, "resource_store_unavailable", "Resource validation is busy; try again shortly")
		return nil, false
	}
}

type resourceMultipart struct {
	Filename string
	Data     []byte
	Fields   map[string]string
}

func readResourceMultipart(w http.ResponseWriter, r *http.Request, allowed map[string]bool) (resourceMultipart, error) {
	var upload resourceMultipart
	// WebSocket routes require the server's global ReadTimeout to stay unset.
	// Bound only this upload so a stalled authorized sender cannot retain a slot.
	controller := http.NewResponseController(w)
	deadline := time.Now().Add(30 * time.Second)
	if contextual, ok := r.Context().Deadline(); ok && contextual.Before(deadline) {
		deadline = contextual
	}
	if err := controller.SetReadDeadline(deadline); err != nil && !errors.Is(err, http.ErrNotSupported) {
		return upload, resourceInvalid("The upload could not be read")
	}
	defer func() {
		// Close while the deadline still applies: net/http may otherwise drain
		// an unfinished body indefinitely before writing an error response.
		_ = r.Body.Close()
		_ = controller.SetReadDeadline(time.Time{})
	}()
	upload.Fields = make(map[string]string)
	limit := service.ResourcePublishingLimits().MaxUploadBytes
	r.Body = http.MaxBytesReader(w, r.Body, limit+16*1024)
	m, err := r.MultipartReader()
	if err != nil {
		return upload, resourceInvalid("A multipart file upload is required")
	}
	for count := 0; ; count++ {
		part, err := m.NextRawPart()
		if err == io.EOF {
			break
		}
		if err != nil || count >= len(allowed)+1 {
			return upload, resourceInvalid("The upload has invalid or excessive parts")
		}
		name := part.FormName()
		if name == "file" {
			if upload.Data != nil || part.FileName() == "" || len(part.FileName()) > 255 {
				return upload, resourceInvalid("Exactly one named file is required")
			}
			upload.Filename = part.FileName()
			upload.Data, err = io.ReadAll(io.LimitReader(part, limit+1))
			if err != nil || int64(len(upload.Data)) > limit || len(upload.Data) == 0 {
				return upload, resourceInvalid("The uploaded file is empty or exceeds the upload limit")
			}
		} else {
			if !allowed[name] || part.FileName() != "" {
				return upload, resourceInvalid("The upload contains an unexpected field")
			}
			if _, found := upload.Fields[name]; found {
				return upload, resourceInvalid("Upload fields must not be repeated")
			}
			data, readErr := io.ReadAll(io.LimitReader(part, 4097))
			if readErr != nil || len(data) > 4096 || !utf8.Valid(data) || strings.ContainsRune(string(data), 0) {
				return upload, resourceInvalid("An upload field is invalid or too long")
			}
			upload.Fields[name] = string(data)
		}
		if err := part.Close(); err != nil {
			return upload, resourceInvalid("The upload could not be read")
		}
	}
	if upload.Data == nil {
		return upload, resourceInvalid("Exactly one named file is required")
	}
	return upload, nil
}

func (h *Handler) AdminResourcePreview(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.requirePlatformAccess(w, r, true)
	if !ok || !h.requireResourcePublishing(w, r) || !h.resourceWriteLimit(w, r, actor) {
		return
	}
	release, ok := acquireResourceUpload(w, r)
	if !ok {
		return
	}
	defer release()
	upload, err := readResourceMultipart(w, r, map[string]bool{"key": true})
	if err != nil {
		resourceHTTPError(w, r, err)
		return
	}
	preview, err := h.resourcePublisher().Preview(r.Context(), chi.URLParam(r, "kind"), upload.Fields["key"], upload.Filename, upload.Data, uuidToString(actor.OrganizationID))
	if err != nil {
		resourceHTTPError(w, r, err)
		return
	}
	writeJSON(w, 200, preview)
}

func resourceOperationID(r *http.Request) (string, error) {
	id, err := uuid.Parse(r.Header.Get("Idempotency-Key"))
	if err != nil || id == uuid.Nil {
		return "", resourceInvalid("A UUID Idempotency-Key header is required")
	}
	return id.String(), nil
}

func validateResourceReason(reason string) error {
	if strings.TrimSpace(reason) == "" || len(reason) > 1000 || !utf8.ValidString(reason) || strings.ContainsRune(reason, 0) {
		return resourceInvalid("A reason of at most 1000 bytes is required")
	}
	return nil
}

func validateResourceExpectedVersion(version string, required bool) error {
	if version == "" && !required {
		return nil
	}
	id, err := uuid.Parse(version)
	if err != nil || id == uuid.Nil || id.String() != version {
		return resourceInvalid("The expected version must be a resource revision UUID")
	}
	return nil
}

func (h *Handler) AdminResourcePublish(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.requirePlatformAccess(w, r, true)
	if !ok || !h.requireResourcePublishing(w, r) || !h.resourceWriteLimit(w, r, actor) {
		return
	}
	op, err := resourceOperationID(r)
	if err != nil {
		resourceHTTPError(w, r, err)
		return
	}
	release, ok := acquireResourceUpload(w, r)
	if !ok {
		return
	}
	defer release()
	upload, err := readResourceMultipart(w, r, map[string]bool{"preview_digest": true, "expected_version": true, "reason": true})
	if err != nil {
		resourceHTTPError(w, r, err)
		return
	}
	p := service.ResourceMutation{Kind: chi.URLParam(r, "kind"), Key: chi.URLParam(r, "key"), Filename: upload.Filename, Data: upload.Data,
		PreviewDigest: upload.Fields["preview_digest"], ExpectedVersion: upload.Fields["expected_version"], Reason: strings.TrimSpace(upload.Fields["reason"]),
		ActorID: uuidToString(actor.UserID), OrganizationID: uuidToString(actor.OrganizationID), OperationID: op}
	if err = validateResourceReason(p.Reason); err != nil {
		resourceHTTPError(w, r, err)
		return
	}
	if err = validateResourceExpectedVersion(p.ExpectedVersion, false); err != nil {
		resourceHTTPError(w, r, err)
		return
	}
	result, err := h.resourcePublisher().Publish(r.Context(), p, h.resourceCommitGuard(actor, p, "publish", adminRequestID(r)))
	if err != nil {
		resourceHTTPError(w, r, err)
		return
	}
	writeJSON(w, 200, result)
}

func (h *Handler) AdminResourceWithdraw(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.requirePlatformAccess(w, r, true)
	if !ok || !h.requireResourcePublishing(w, r) || !h.resourceWriteLimit(w, r, actor) {
		return
	}
	op, err := resourceOperationID(r)
	if err != nil {
		resourceHTTPError(w, r, err)
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		resourceHTTPError(w, r, resourceInvalid("Content-Type must be application/json"))
		return
	}
	var body struct {
		ExpectedVersion string `json:"expected_version"`
		Reason          string `json:"reason"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if decoder.Decode(&body) != nil || decoder.Decode(new(any)) != io.EOF {
		resourceHTTPError(w, r, resourceInvalid("The request body is invalid"))
		return
	}
	if err = validateResourceReason(body.Reason); err != nil {
		resourceHTTPError(w, r, err)
		return
	}
	if err = validateResourceExpectedVersion(body.ExpectedVersion, true); err != nil {
		resourceHTTPError(w, r, err)
		return
	}
	p := service.ResourceMutation{Kind: chi.URLParam(r, "kind"), Key: chi.URLParam(r, "key"), ExpectedVersion: body.ExpectedVersion, Reason: strings.TrimSpace(body.Reason),
		ActorID: uuidToString(actor.UserID), OrganizationID: uuidToString(actor.OrganizationID), OperationID: op}
	result, err := h.resourcePublisher().Withdraw(r.Context(), p, h.resourceCommitGuard(actor, p, "withdraw", adminRequestID(r)))
	if err != nil {
		resourceHTTPError(w, r, err)
		return
	}
	writeJSON(w, 200, result)
}

func (h *Handler) AdminResourceOperation(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.requirePlatformAccess(w, r, false)
	if !ok || !h.requireResourcePublishing(w, r) {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil || id == uuid.Nil {
		resourceHTTPError(w, r, resourceInvalid("A valid operation UUID is required"))
		return
	}
	result, err := h.resourcePublisher().Receipt(r.Context(), uuidToString(actor.OrganizationID), uuidToString(actor.UserID), id.String())
	if err != nil {
		resourceHTTPError(w, r, err)
		return
	}
	writeJSON(w, 200, result)
}

func resourceOutcomeUnknown(operationID string) error {
	return &service.ResourceError{Status: 503, Code: "resource_outcome_unknown", Message: "The operation outcome is uncertain; refresh its operation receipt before retrying", OperationID: operationID}
}

// The store calls this only inside its filesystem lock, including replays.
// Keep lock order FS -> platform advisory -> actor user for all new writers.
func (h *Handler) resourceCommitGuard(actor service.PlatformAdminIdentity, p service.ResourceMutation, action, requestID string) service.ResourceCommitGuard {
	return func(ctx context.Context, apply func() (service.ResourceMutationResult, error)) (service.ResourceMutationResult, error) {
		var empty service.ResourceMutationResult
		if h.TxStarter == nil {
			return empty, errors.New("administration transactions unavailable")
		}
		ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		tx, err := h.TxStarter.Begin(ctx)
		if err != nil {
			return empty, err
		}
		defer tx.Rollback(context.Background())
		if err = service.LockPlatformAdminMutation(ctx, tx, actor.UserID); err != nil {
			return empty, err
		}
		q := db.New(tx)
		current, err := service.NewPlatformAdminService(q, nil).Authorize(ctx, true)
		if err != nil {
			return empty, err
		}
		if current.OrganizationID != actor.OrganizationID || current.UserID != actor.UserID {
			return empty, &service.PlatformAdminError{Status: 403, Code: "admin_forbidden", Message: "The administrative scope changed"}
		}
		target := uuid.NewSHA1(uuid.NameSpaceURL, []byte("multica:resource:"+p.OrganizationID+":"+p.Kind+":"+p.Key))
		before, _ := json.Marshal(map[string]string{"kind": p.Kind, "key": p.Key, "expected_version": p.ExpectedVersion, "operation_id": p.OperationID})
		audit := db.CreateAdminAuditEventParams{OrganizationID: actor.OrganizationID, ActorKind: "user", ActorUserID: actor.UserID,
			TargetKind: "resource", TargetID: pgtype.UUID{Bytes: target, Valid: true}, Action: "resource." + action, Phase: "requested", RequestID: requestID,
			Reason: p.Reason, BeforeState: before, AfterState: []byte(`{}`), ResultCode: "resource_requested"}
		// Independent autocommit survives a failed filesystem/actor transaction.
		// OperationID intentionally stays NULL: receipts belong to the file index.
		if err = h.Queries.CreateAdminAuditEvent(ctx, audit); err != nil {
			return empty, err
		}
		result, err := apply()
		if err != nil {
			return result, err
		}
		if !result.Replayed {
			audit.Phase, audit.ResultCode = "applied", "resource_"+action+"_applied"
			audit.AfterState, _ = json.Marshal(map[string]any{"kind": p.Kind, "key": p.Key, "operation_id": result.OperationID, "version": result.Resource.Version, "content_digest": result.Resource.ContentDigest, "state": result.Resource.State, "file_count": result.Resource.FileCount, "byte_count": result.Resource.ByteCount})
			if err = q.CreateAdminAuditEvent(ctx, audit); err != nil {
				return result, resourceOutcomeUnknown(p.OperationID)
			}
		}
		if err = tx.Commit(ctx); err != nil {
			return result, resourceOutcomeUnknown(p.OperationID)
		}
		return result, nil
	}
}
