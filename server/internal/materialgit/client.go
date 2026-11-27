package materialgit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	MaxFileBytes     = 1 << 20
	maxResponseBytes = 8 << 20
	maxPages         = 100
	operationTimeout = 2 * time.Minute
)

// Error deliberately excludes provider bodies, URLs and underlying transport
// errors: all three may contain a credential or private repository content.
type Error struct {
	Code      string
	Status    int
	Retryable bool
	Uncertain bool
}

func (e *Error) Error() string  { return "material git: " + e.Code }
func failure(code string) error { return &Error{Code: code} }
func isStatus(err error, status int) bool {
	var e *Error
	return errors.As(err, &e) && e.Status == status
}

type client struct {
	connection Connection
	base, repo string
	http       *http.Client
}

// New only accepts an operator-configured HTTPS instance. GitHub Enterprise is
// intentionally not inferred from an arbitrary URL. Tests use newClient with
// their own server; production callers cannot opt out of TLS validation.
func New(conn Connection) (Provider, error) {
	return newClient(conn, &http.Client{Timeout: 30 * time.Second}, false)
}

func newClient(conn Connection, hc *http.Client, testEndpoint bool) (Provider, error) {
	if conn.Provider != "github" && conn.Provider != "gitlab" {
		return nil, failure("unsupported_provider")
	}
	if !validRepository(conn.Repository) || strings.TrimSpace(conn.Token) == "" || strings.ContainsAny(conn.Token, "\r\n") {
		return nil, failure("invalid_connection")
	}
	if conn.Provider == "github" && len(strings.Split(conn.Repository, "/")) != 2 {
		return nil, failure("invalid_repository")
	}
	if conn.BaseURL == "" {
		if conn.Provider == "github" {
			conn.BaseURL = "https://api.github.com"
		} else {
			conn.BaseURL = "https://gitlab.com"
		}
	}
	u, err := url.Parse(conn.BaseURL)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" || u.RawPath != "" || (u.Path != "" && u.Path != "/" && !validPath(strings.Trim(u.Path, "/"))) || (u.Scheme != "https" && !(testEndpoint && u.Scheme == "http")) {
		return nil, failure("invalid_instance_url")
	}
	if conn.Provider == "github" && !testEndpoint && (u.Host != "api.github.com" || (u.Path != "" && u.Path != "/")) {
		return nil, failure("unsupported_github_instance")
	}
	copyClient := *hc
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	if copyClient.Timeout == 0 {
		copyClient.Timeout = 30 * time.Second
	}
	c := &client{connection: conn, base: strings.TrimRight(conn.BaseURL, "/"), http: &copyClient}
	if conn.Provider == "github" {
		c.repo = "/repos/" + escapePath(conn.Repository)
		return &githubProvider{c}, nil
	}
	c.base += "/api/v4"
	c.repo = "/projects/" + url.PathEscape(conn.Repository)
	return &gitlabProvider{c}, nil
}

func (c *client) request(ctx context.Context, method, endpoint string, body, out any) (http.Header, error) {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, failure("invalid_request")
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+endpoint, reader)
	if err != nil {
		return nil, failure("invalid_request")
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.connection.Provider == "github" {
		req.Header.Set("Authorization", "Bearer "+c.connection.Token)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	} else {
		req.Header.Set("PRIVATE-TOKEN", c.connection.Token)
	}
	mutating := method != http.MethodGet && method != http.MethodHead
	response, err := c.http.Do(req)
	if err != nil {
		return nil, &Error{Code: "transport_unavailable", Retryable: true, Uncertain: mutating}
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		code := "provider_rejected"
		switch response.StatusCode {
		case 401, 403:
			code = "provider_permission_denied"
		case 404:
			code = "provider_not_found"
		case 409, 422:
			code = "provider_conflict"
		case 429:
			code = "provider_rate_limited"
		}
		if response.StatusCode >= 300 && response.StatusCode < 400 {
			code = "provider_redirect_refused"
		}
		return response.Header, &Error{Code: code, Status: response.StatusCode, Retryable: response.StatusCode == 429 || response.StatusCode >= 500, Uncertain: mutating && response.StatusCode >= 500}
	}
	b, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil || len(b) > maxResponseBytes {
		return response.Header, &Error{Code: "provider_response_incomplete", Retryable: true, Uncertain: mutating}
	}
	if out != nil && (bytes.Equal(bytes.TrimSpace(b), []byte("null")) || json.Unmarshal(b, out) != nil) {
		return response.Header, &Error{Code: "provider_response_invalid", Uncertain: mutating}
	}
	return response.Header, nil
}

var keyPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,128}$`)
var shaPattern = regexp.MustCompile(`^(?:[a-f0-9]{40}|[a-f0-9]{64})$`)

func validPath(s string) bool {
	if s == "" || len(s) > 1024 || !utf8.ValidString(s) || path.Clean(s) != s || strings.HasPrefix(s, "/") || strings.ContainsAny(s, "\\\x00\r\n%?#") {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return false
		}
	}
	for _, p := range strings.Split(s, "/") {
		if p == "" || p == "." || p == ".." || strings.EqualFold(p, ".git") {
			return false
		}
	}
	return true
}
func validRepository(s string) bool {
	if !validPath(s) || strings.ContainsAny(s, " :@") || strings.HasSuffix(s, ".git") {
		return false
	}
	return len(strings.Split(s, "/")) >= 2
}
func validRef(s string) bool {
	if !validPath(s) || len(s) > 240 || strings.HasPrefix(s, "refs/") || strings.HasSuffix(s, ".") || strings.Contains(s, "..") || strings.Contains(s, "@{") || strings.ContainsAny(s, " ~^:?*[\t") || s == "@" {
		return false
	}
	for _, p := range strings.Split(s, "/") {
		if strings.HasPrefix(p, ".") || strings.HasSuffix(p, ".lock") {
			return false
		}
	}
	return true
}
func materialPath(s string) bool {
	if !validPath(s) {
		return false
	}
	parts := strings.Split(s, "/")
	if len(parts) < 3 || parts[len(parts)-3] != "requirements" || !keyPattern.MatchString(parts[len(parts)-2]) {
		return false
	}
	switch parts[len(parts)-1] {
	case "prd.md", "design.md", "plan.md":
		return true
	}
	return false
}
func validateRead(commit, name string) error {
	if !shaPattern.MatchString(commit) || !validPath(name) {
		return failure("invalid_file_reference")
	}
	return nil
}
func escapePath(s string) string {
	parts := strings.Split(s, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}

func (c *client) validateReviewURL(raw string, number int64) bool {
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Scheme != "https" {
		return false
	}
	if c.connection.Provider == "github" {
		return u.Host == "github.com" && u.Path == "/"+c.connection.Repository+"/pull/"+strconv.FormatInt(number, 10)
	}
	base, err := url.Parse(c.connection.BaseURL)
	if err != nil {
		return false
	}
	return u.Host == base.Host && u.Path == strings.TrimRight(base.Path, "/")+"/"+c.connection.Repository+"/-/merge_requests/"+strconv.FormatInt(number, 10)
}
func nextPage(h http.Header, current, count int, github bool) (bool, error) {
	if current >= maxPages {
		return false, failure("provider_pagination_limit")
	}
	if github {
		link := h.Get("Link")
		if strings.Contains(link, `rel="next"`) {
			// Construct our own same-origin page; never follow a provider URL.
			if count == 0 {
				return false, failure("provider_pagination_invalid")
			}
			return true, nil
		}
		// A full final page is ambiguous without Link. Request one extra page.
		return count == 100, nil
	}
	n := h.Get("X-Next-Page")
	if n != "" {
		v, err := strconv.Atoi(n)
		if err != nil || v != current+1 || count == 0 {
			return false, failure("provider_pagination_invalid")
		}
		return true, nil
	}
	return count == 100, nil
}
