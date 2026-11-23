package service

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func collaborationRecipientAvailable(account db.FindCollaborationRecipientsRow) bool {
	return account.UserID.Valid && account.Available &&
		!auth.IsTemporarilyDisabledUser(util.UUIDToString(account.UserID), account.Email.String)
}

// collaborationRecipientEligible borrows the writer's queries after its sorted
// user locks. Password setup/recovery/change take that same user lock, so the
// resolver's enabled/credential/restriction rules stay valid through commit.
// This check is for enrollment and responsibility, not notification fanout.
func collaborationRecipientEligible(ctx context.Context, q *db.Queries, target, source pgtype.UUID) (bool, error) {
	accounts, err := q.FindCollaborationRecipients(ctx, db.FindCollaborationRecipientsParams{
		Kind: "user_id", UserID: target, SourceWorkspaceID: source,
		TrimChars: collaborationRecipientTrimChars, RequirePasswordCredential: auth.PasswordMode(),
	})
	if err != nil {
		return false, err
	}
	return len(accounts) == 1 && accounts[0].UserID == target && collaborationRecipientAvailable(accounts[0]), nil
}
