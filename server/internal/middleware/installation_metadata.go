package middleware

import (
	"errors"
	"net/http"

	"github.com/multica-ai/multica/server/internal/auth"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// InstallationMetadata runs after authentication and cannot replace it.
func InstallationMetadata(q *db.Queries) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			proof := r.Header.Get("X-Installation-Proof")
			if proof == "" {
				next.ServeHTTP(w, r)
				return
			}
			session, ok := auth.PasswordSessionFromContext(r.Context())
			if !ok {
				writeError(w, 403, "installation proof requires password authentication")
				return
			}
			id, err := auth.ValidateInstallationMetadata(r.Context(), q, proof, session)
			if err != nil {
				status := 503
				if errors.Is(err, auth.ErrInstallationMetadata) {
					status = 403
				} else if errors.Is(err, auth.ErrPasswordSession) {
					status = 401
				}
				writeError(w, status, "installation attribution is unavailable or invalid")
				return
			}
			next.ServeHTTP(w, r.WithContext(auth.WithSubmissionInstallation(r.Context(), id)))
		})
	}
}
