package auth

import (
	"context"
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
func RevokePasswordCredentials(ctx context.Context, tx pgx.Tx, userID pgtype.UUID) (PasswordRevocation, error) {
	var result PasswordRevocation
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
	q := db.New(tx)
	result.CancelledTasks, err = q.CancelAgentTasksByRuntimeOrAgent(ctx, db.CancelAgentTasksByRuntimeOrAgentParams{RuntimeIds: ids, AgentIds: []pgtype.UUID{}})
	if err != nil {
		return result, err
	}
	result.OfflineRuntimes, err = q.ForceOfflineRuntimesByIDs(ctx, ids)
	return result, err
}
