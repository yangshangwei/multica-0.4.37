package service

import (
	"bytes"
	"context"
	"errors"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/issuestatus"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// CheckDeliveryProjectChangeInTx joins the real writer transaction. An accepted
// controlled adjustment is retained with the issue write; a later failure must
// roll both back. It never grants ordinary or collaboration editing authority.
func CheckDeliveryProjectChangeInTx(ctx context.Context, tx pgx.Tx, before, candidate db.Issue, credential ArtifactCredentialSnapshot, reason string, operationID pgtype.UUID) (err error) {
	defer func() {
		var busy *pgconn.PgError
		if errors.As(err, &busy) && (busy.Code == "55P03" || busy.Code == "40P01" || busy.Code == "40001") {
			err = DeliveryCompletionError{409, "delivery_project_change_conflict"}
		}
	}()
	if before.ProjectID == candidate.ProjectID {
		return nil
	}
	if tx == nil || before.ID != candidate.ID || before.WorkspaceID != candidate.WorkspaceID || !before.ID.Valid {
		return DeliveryCompletionError{400, "invalid_issue_update"}
	}
	ids := []pgtype.UUID{}
	for _, id := range []pgtype.UUID{before.ProjectID, candidate.ProjectID} {
		if id.Valid {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return bytes.Compare(ids[i].Bytes[:], ids[j].Bytes[:]) < 0 })
	policies := map[pgtype.UUID]DeliveryPolicy{}
	projects := map[pgtype.UUID]db.Project{}
	for _, id := range ids {
		project, policy, e := LockDeliveryProjectPolicy(ctx, tx, before.WorkspaceID, id)
		if e != nil {
			return e
		}
		policies[id] = policy
		projects[id] = project
	}
	if err = CheckStageProjectChangeInTx(ctx, tx, before, candidate); err != nil {
		return err
	}
	oldPolicy, newPolicy := policies[before.ProjectID], policies[candidate.ProjectID]
	if oldPolicy.Mode != "enforce" && newPolicy.Mode != "enforce" {
		return nil
	}
	q := db.New(tx)
	beforeCategory, e := IssueStatusCategory(ctx, q, before)
	if e != nil {
		return e
	}
	afterCategory := beforeCategory
	if before.Status != candidate.Status {
		status, e := issuestatus.Resolve(ctx, q, before.WorkspaceID, candidate.Status)
		if e != nil {
			return e
		}
		afterCategory = status.Category
	}
	if beforeCategory == "done" || afterCategory == "done" {
		return DeliveryCompletionError{409, "delivery_project_change_conflict"}
	}
	kind, actor, e := credential.Actor()
	if e != nil {
		return DeliveryCompletionError{403, "delivery_project_change_denied"}
	}
	if oldPolicy.Mode == "enforce" && kind != "human" {
		return DeliveryCompletionError{403, "delivery_project_change_denied"}
	}
	members, e := LockArtifactUsers(ctx, q, before.WorkspaceID, credential.Principal.UserID)
	if e != nil {
		return deliveryProjectChangeAuthorityError(e)
	}
	if e = CheckArtifactCredentialInTx(ctx, tx, credential); e != nil {
		return deliveryProjectChangeAuthorityError(e)
	}
	if oldPolicy.Mode == "enforce" {
		member := members[credential.Principal.UserID]
		if member.Role != "owner" && member.Role != "admin" {
			return DeliveryCompletionError{403, "delivery_project_change_denied"}
		}
		reason = strings.TrimSpace(reason)
		if reason == "" {
			return DeliveryCompletionError{428, "delivery_project_change_reason_required"}
		}
	}
	if !utf8.ValidString(reason) || strings.ContainsRune(reason, 0) || utf8.RuneCountInString(reason) > 1000 {
		return DeliveryCompletionError{400, "invalid_issue_update"}
	}
	if !operationID.Valid {
		return DeliveryCompletionError{503, "delivery_project_change_unavailable"}
	}
	var ready bool
	if e = tx.QueryRow(ctx, deliveryProjectChangeReadinessSQL).Scan(&ready); e != nil {
		return e
	}
	if !ready {
		return DeliveryCompletionError{503, "delivery_project_change_schema_unavailable"}
	}
	actorID, e := util.ParseUUID(actor)
	if e != nil {
		return DeliveryCompletionError{403, "delivery_project_change_denied"}
	}
	userID, e := util.ParseUUID(credential.Principal.UserID)
	if e != nil {
		return DeliveryCompletionError{403, "delivery_project_change_denied"}
	}
	_, e = tx.Exec(ctx, `INSERT INTO delivery_project_change
 (workspace_id,operation_id,issue_id,issue_revision,actor_kind,actor_id,actor_user_id,from_project_id,to_project_id,from_project_revision,to_project_revision,from_policy_revision,to_policy_revision,reason)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		before.WorkspaceID, operationID, before.ID, before.Revision, kind, actorID, userID, before.ProjectID, candidate.ProjectID, projects[before.ProjectID].Revision, projects[candidate.ProjectID].Revision, oldPolicy.Revision, newPolicy.Revision, reason)
	return e
}

const deliveryProjectChangeReadinessSQL = `SELECT EXISTS (
 SELECT 1 FROM pg_catalog.pg_index i JOIN pg_catalog.pg_class c ON c.oid=i.indexrelid JOIN pg_catalog.pg_am a ON a.oid=c.relam
 WHERE i.indrelid=to_regclass('delivery_project_change') AND c.relname='delivery_project_change_operation_idx' AND a.amname='btree'
 AND i.indisunique AND i.indisvalid AND i.indisready AND i.indislive
 AND NOT i.indisexclusion AND i.indpred IS NULL AND i.indexprs IS NULL
 AND i.indnkeyatts=3 AND i.indnatts=3 AND i.indoption::text='0 0 0'
 AND pg_get_indexdef(i.indexrelid,1,true)='workspace_id'
 AND pg_get_indexdef(i.indexrelid,2,true)='operation_id'
 AND pg_get_indexdef(i.indexrelid,3,true)='issue_id'
)`

func deliveryProjectChangeAuthorityError(err error) error {
	var failure ArtifactError
	if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, auth.ErrPasswordSession) || errors.As(err, &failure) && failure.Status == 403 {
		return DeliveryCompletionError{Status: 403, Code: "delivery_project_change_denied"}
	}
	return err
}
