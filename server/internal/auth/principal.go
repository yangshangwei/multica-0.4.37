package auth

import "context"

// CredentialKind records the credential actually verified by the server. It
// must never be inferred from a client-supplied actor header or missing fields.
type CredentialKind string

const (
	CredentialJWT   CredentialKind = "jwt"
	CredentialPAT   CredentialKind = "pat"
	CredentialTask  CredentialKind = "task_token"
	CredentialCloud CredentialKind = "cloud_pat"
)

type AuthenticatedPrincipal struct {
	UserID     string
	Kind       CredentialKind
	Restricted bool
	// AgentActor is a deny-only legacy request hint. It is not proof of an
	// agent identity and must never grant machine or human authority.
	AgentActor bool
}

type authenticatedPrincipalKey struct{}

// WithAuthenticatedPrincipal is for authentication adapters, after verification.
// The private context key keeps request headers and JSON from supplying it.
func WithAuthenticatedPrincipal(ctx context.Context, p AuthenticatedPrincipal) context.Context {
	return context.WithValue(ctx, authenticatedPrincipalKey{}, p)
}

func AuthenticatedPrincipalFromContext(ctx context.Context) (AuthenticatedPrincipal, bool) {
	p, ok := ctx.Value(authenticatedPrincipalKey{}).(AuthenticatedPrincipal)
	return p, ok && p.UserID != ""
}
