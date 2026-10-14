package auth

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type PasswordRevocation struct {
	CancelledTasks  []db.AgentTaskQueue
	OfflineRuntimes []db.ForceOfflineRuntimesByIDsRow
}

// RevokePasswordCredentials runs under the caller's user-row lock and transaction.
// It preserves users, memberships, installations and execution history.
// revocationKey identifies the committed credential version (password:<version>)
// or administrative operation (admin:<operation UUID>), never a new retry key.
func RevokePasswordCredentials(ctx context.Context, tx pgx.Tx, userID pgtype.UUID, revocationKey string) (PasswordRevocation, error) {
	var result PasswordRevocation
	if !userID.Valid || revocationKey == "" || len(revocationKey) > 128 {
		return result, errors.New("invalid credential revocation identity")
	}
	q := db.New(tx)
	if _, err := q.InvalidateCollaborationAccount(ctx, db.InvalidateCollaborationAccountParams{UserID: userID, RevocationKey: revocationKey}); err != nil {
		return result, err
	}
	for _, statement := range []string{
		`UPDATE personal_access_token SET revoked = true WHERE user_id = $1`,
		`DELETE FROM task_token WHERE user_id = $1`,
		`DELETE FROM daemon_token WHERE user_id = $1`,
	} {
		if _, err := tx.Exec(ctx, statement, userID); err != nil {
			return result, err
		}
	}
	rows, err := tx.Query(ctx, `SELECT id FROM agent_runtime WHERE owner_id = $1 ORDER BY id`, userID)
	if err != nil {
		return result, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[pgtype.UUID])
	if err != nil {
		return result, err
	}
	if len(ids) == 0 {
		return result, nil
	}
	result.CancelledTasks, err = q.CancelAgentTasksByRuntimeOrAgent(ctx, db.CancelAgentTasksByRuntimeOrAgentParams{RuntimeIds: ids, AgentIds: []pgtype.UUID{}})
	if err != nil {
		return result, err
	}
	result.OfflineRuntimes, err = q.ForceOfflineRuntimesByIDs(ctx, ids)
	return result, err
}
