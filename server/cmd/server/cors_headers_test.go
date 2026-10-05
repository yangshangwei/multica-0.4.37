package main

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/go-chi/cors"
	"github.com/multica-ai/multica/server/internal/handler"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// The app advertises its capabilities on the cancel request (#5219). Browsers
// preflight a custom request header, so an entry missing from AllowedHeaders is
// not a degraded feature — it is a failed request: the cancel never reaches the
// server, and the user's prompt is lost in a way no server-side test can see.
func TestCORSAllowedHeaders_IncludeClientCapabilities(t *testing.T) {
	if !slices.Contains(corsAllowedHeaders, "X-Client-Capabilities") {
		t.Fatalf("X-Client-Capabilities missing from CORS allowed headers: %v", corsAllowedHeaders)
	}
	// Named so the constant and the header travel together: the capability is
	// useless if the header carrying it cannot cross the preflight.
	if protocol.AppCapabilityChatDraftRestoreV1 == "" {
		t.Fatal("AppCapabilityChatDraftRestoreV1 must be a non-empty capability token")
	}
}

// Workspace subscription checkout and portal requests use Idempotency-Key to
// make retries safe. Browsers preflight that custom request header, so omitting
// it from AllowedHeaders prevents the billing request from reaching the server.
func TestCORSAllowedHeaders_IncludeIdempotencyKey(t *testing.T) {
	if !slices.Contains(corsAllowedHeaders, "Idempotency-Key") {
		t.Fatalf("Idempotency-Key missing from CORS allowed headers: %v", corsAllowedHeaders)
	}
}

// Timeline and comment-list endpoints report defensive hard-cap clamps with
// custom response headers.
// Custom response headers are not readable from browser JS unless the server
// exposes them, and only the CORS-safelisted headers are exposed by default — so
// an entry missing here is not a degraded signal, it is no signal at all: the
// header arrives on the wire and the client cannot see it (MUL-5492).
func TestCORSExposedHeaders_IncludeTruncationSignals(t *testing.T) {
	for _, want := range []string{
		handler.HeaderCommentsTruncated,
		handler.HeaderTimelineTruncated,
	} {
		if !slices.Contains(corsExposedHeaders, want) {
			t.Errorf("%s missing from CORS exposed headers: %v", want, corsExposedHeaders)
		}
	}
}

// Password forms use Retry-After to disable resubmission. A wire header alone
// is insufficient: cross-origin browser fetch hides it unless CORS exposes it.
func TestCORSExposesPasswordRetryAfter(t *testing.T) {
	const origin = "https://app.example.test"
	for _, status := range []int{http.StatusTooManyRequests, http.StatusServiceUnavailable} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			h := cors.Handler(cors.Options{
				AllowedOrigins:   []string{origin},
				ExposedHeaders:   corsExposedHeaders,
				AllowCredentials: true,
			})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Retry-After", "3")
				w.WriteHeader(status)
			}))
			req := httptest.NewRequest(http.MethodPost, "https://api.example.test/auth/login", nil)
			req.Header.Set("Origin", origin)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != status || rec.Header().Get("Retry-After") != "3" {
				t.Fatalf("cooldown response changed: status=%d retry-after=%q", rec.Code, rec.Header().Get("Retry-After"))
			}
			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != origin {
				t.Fatalf("allowed origin=%q, want %q", got, origin)
			}
			exposed := strings.Join(rec.Header().Values("Access-Control-Expose-Headers"), ",")
			for _, header := range strings.Split(exposed, ",") {
				if strings.EqualFold(strings.TrimSpace(header), "Retry-After") {
					return
				}
			}
			t.Fatalf("browser cannot read Retry-After; exposed response headers: %q", exposed)
		})
	}
}
