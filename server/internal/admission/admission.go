// Package admission owns the formal-work boundary independently of execution status.
package admission

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Formal is an allowlist: unknown states cannot become execution authority.
func Formal(status string) bool { return status == "not_required" || status == "accepted" }

// Blocked identifies an input which must be reviewed before execution or linking.
type Blocked struct{ IssueID pgtype.UUID }

func (e *Blocked) Error() string {
	return "This issue requires triage review before execution or changing its workflow; open the triage item to review it."
}

type Reader interface {
	GetIssue(context.Context, pgtype.UUID) (db.Issue, error)
}

// Check always reads persisted state. Callers holding locks pass their transaction
// queries, never another pool connection or an already-loaded issue snapshot.
func Check(ctx context.Context, q Reader, id pgtype.UUID) error {
	if !id.Valid {
		return nil
	}
	issue, err := q.GetIssue(ctx, id)
	if err != nil {
		return fmt.Errorf("read issue admission: %w", err)
	}
	if !Formal(issue.AdmissionStatus) {
		return &Blocked{IssueID: id}
	}
	return nil
}
