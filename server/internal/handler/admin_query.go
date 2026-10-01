package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
)

// AdminListScope binds a cursor to its authorization and resource. Window=0
// permits all history; execution feeds use a bounded 31-day default and maximum.
type AdminListScope struct {
	Resource, ActorID, OrganizationID string
	Window                            time.Duration
}
type AdminListQuery struct {
	Filters                   map[string]string
	AsOf, From, To, AfterTime time.Time
	AfterID, Timezone         string
	Limit                     int32
	scope                     AdminListScope
	fingerprint               string
}
type adminCursor struct {
	Version                                    int `json:"v"`
	Resource, Actor, Organization, Fingerprint string
	AsOf, CreatedAt                            time.Time
	ID                                         string
}

func adminQueryError() error {
	return &service.PlatformAdminError{Status: 400, Code: "invalid_query", Message: "Invalid filters, time range, or cursor; refresh the list"}
}

func ParseAdminListQuery(r *http.Request, scope AdminListScope, filterNames []string, now time.Time) (AdminListQuery, error) {
	p := AdminListQuery{Filters: map[string]string{}, AsOf: now.UTC(), Limit: 50, Timezone: "UTC", scope: scope}
	allowed := map[string]bool{"cursor": true, "limit": true, "time_from": true, "time_to": true, "timezone": true}
	for _, name := range filterNames {
		allowed[name] = true
	}
	values, parseErr := url.ParseQuery(r.URL.RawQuery)
	if parseErr != nil {
		return p, adminQueryError()
	}
	for name, items := range values {
		if !allowed[name] || len(items) != 1 || len(items[0]) > 4096 {
			return p, adminQueryError()
		}
		if name != "cursor" {
			p.Filters[name] = items[0]
		}
	}
	if value := values.Get("limit"); value != "" {
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 || n > 100 {
			return p, adminQueryError()
		}
		p.Limit = int32(n)
	}
	if value := values.Get("timezone"); value != "" {
		if _, err := time.LoadLocation(value); err != nil || value == "Local" {
			return p, adminQueryError()
		}
		p.Timezone = value
	}
	if allowed["time_basis"] {
		basis := values.Get("time_basis")
		if basis == "" {
			basis = "created"
		}
		if basis != "created" && basis != "finished" {
			return p, adminQueryError()
		}
		p.Filters["time_basis"] = basis
	}
	// Equivalent defaults share a fingerprint; all action-specific filters remain
	// bound even when the caller adds a new one later.
	p.Filters["limit"] = strconv.Itoa(int(p.Limit))
	p.Filters["timezone"] = p.Timezone
	encoded, _ := json.Marshal(p.Filters)
	digest := sha256.Sum256(encoded)
	p.fingerprint = base64.RawURLEncoding.EncodeToString(digest[:])
	if token := values.Get("cursor"); token != "" {
		c, err := decodeAdminCursor(token)
		if err != nil || c.Version != 1 || c.Resource != scope.Resource || c.Actor != scope.ActorID || c.Organization != scope.OrganizationID || c.Fingerprint != p.fingerprint || c.AsOf.After(now.Add(time.Minute)) || c.AsOf.Before(now.Add(-24*time.Hour)) || c.CreatedAt.After(c.AsOf) {
			return p, adminQueryError()
		}
		if _, err := uuid.Parse(c.ID); err != nil {
			return p, adminQueryError()
		}
		p.AsOf, p.AfterTime, p.AfterID = c.AsOf, c.CreatedAt, c.ID
	}
	p.To = p.AsOf
	for name, target := range map[string]*time.Time{"time_from": &p.From, "time_to": &p.To} {
		if value := values.Get(name); value != "" {
			parsed, err := time.Parse(time.RFC3339Nano, value)
			if err != nil {
				return p, adminQueryError()
			}
			*target = parsed.UTC()
		}
	}
	if scope.Window > 0 && values.Get("time_from") == "" {
		p.From = p.To.Add(-scope.Window)
	}
	if !p.From.Before(p.To) || p.To.After(now.Add(time.Minute)) || scope.Window > 0 && p.To.Sub(p.From) > scope.Window {
		return p, adminQueryError()
	}
	return p, nil
}
func (p AdminListQuery) Cursor(createdAt time.Time, id string) (string, error) {
	c := adminCursor{Version: 1, Resource: p.scope.Resource, Actor: p.scope.ActorID, Organization: p.scope.OrganizationID, Fingerprint: p.fingerprint, AsOf: p.AsOf, CreatedAt: createdAt, ID: id}
	payload, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, auth.JWTSecret())
	mac.Write([]byte("admin-list-cursor-v1\x00"))
	mac.Write(payload)
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}
func decodeAdminCursor(token string) (adminCursor, error) {
	var c adminCursor
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return c, errors.New("invalid cursor")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return c, err
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return c, err
	}
	mac := hmac.New(sha256.New, auth.JWTSecret())
	mac.Write([]byte("admin-list-cursor-v1\x00"))
	mac.Write(payload)
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return c, errors.New("invalid cursor signature")
	}
	err = json.Unmarshal(payload, &c)
	return c, err
}

func parseAdminQuery(r *http.Request, resource, actor, org string, now time.Time) (AdminListQuery, error) {
	filters := []string{"q", "workspace_id", "issue_id", "status"}
	if resource == "tasks" {
		filters = append(filters, "task_id", "runtime_id", "user_id", "installation_id", "source", "time_basis", "state_scope")
	}
	window := 31 * 24 * time.Hour
	if resource == "tasks" && r.URL.Query().Get("state_scope") == "current" {
		window = 0
	}
	p, err := ParseAdminListQuery(r, AdminListScope{Resource: resource, ActorID: actor, OrganizationID: org, Window: window}, filters, now)
	if err != nil {
		return p, err
	}
	if stateScope := p.Filters["state_scope"]; stateScope != "" {
		values := r.URL.Query()
		if stateScope != "current" || p.Filters["time_basis"] != "created" || values.Has("time_from") || values.Has("time_to") || !adminContains([]string{"queued", "preparing", "dispatched", "running", "waiting_local_directory", "deferred"}, p.Filters["status"]) {
			return p, adminQueryError()
		}
	}
	for _, name := range []string{"workspace_id", "task_id", "issue_id", "runtime_id", "user_id", "installation_id"} {
		if value := p.Filters[name]; value != "" {
			if _, err := uuid.Parse(value); err != nil {
				return p, adminQueryError()
			}
		}
	}
	if len(p.Filters["q"]) > 128 || strings.ContainsRune(p.Filters["q"], 0) {
		return p, adminQueryError()
	}
	if resource == "tasks" && p.Filters["status"] != "" && !adminContains([]string{"queued", "preparing", "dispatched", "running", "waiting_local_directory", "completed", "failed", "cancelled", "deferred"}, p.Filters["status"]) {
		return p, adminQueryError()
	}
	if len(p.Filters["status"]) > 128 {
		return p, adminQueryError()
	}
	if p.Filters["source"] != "" && !adminContains([]string{"issue", "autopilot_issue", "chat", "autopilot", "quick_create", "unknown"}, p.Filters["source"]) {
		return p, adminQueryError()
	}
	return p, nil
}
func adminContains(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}
