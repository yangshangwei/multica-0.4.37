package service

import "github.com/jackc/pgx/v5/pgtype"

func collaborationCommandHistoryReason(reason pgtype.Text) *string {
	if !reason.Valid || reason.String == "" {
		return nil
	}
	return &reason.String
}
