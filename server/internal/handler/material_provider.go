package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/material"
	"github.com/multica-ai/multica/server/internal/materialgit"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func (h *Handler) resolveMaterialProvider(ctx context.Context, q *db.Queries, ws pgtype.UUID, c material.Settings) (materialgit.Provider, func(), error) {
	deny := errors.New("material connection unavailable")
	if c.ConnectionID == nil {
		return nil, nil, deny
	}
	id, err := util.ParseUUID(*c.ConnectionID)
	if err != nil {
		return nil, nil, deny
	}
	conn := materialgit.Connection{Provider: c.Provider, Repository: c.Repository}
	var cleanup func()
	if c.Provider == "gitlab" {
		if !h.isVCSAvailable() || !h.isVCSConfigured() {
			return nil, nil, deny
		}
		row, err := q.GetVCSConnectionByID(ctx, id)
		if err != nil || row.WorkspaceID != ws || row.Provider != "gitlab" {
			return nil, nil, deny
		}
		conn.BaseURL = row.InstanceUrl
		conn.Token, err = h.openVCSSecret(row.AccessTokenEncrypted)
		if err != nil {
			return nil, nil, deny
		}
	} else if c.Provider == "github" {
		row, err := q.GetGitHubInstallationByID(ctx, id)
		if err != nil || row.WorkspaceID != ws {
			return nil, nil, deny
		}
		appJWT, err := signGitHubAppJWT(time.Now())
		if err != nil || appJWT == "" {
			return nil, nil, deny
		}
		// Request only this repository and the actions used by material publication.
		parts := strings.Split(c.Repository, "/")
		if len(parts) != 2 || !strings.EqualFold(parts[0], row.AccountLogin) {
			return nil, nil, deny
		}
		payload, _ := json.Marshal(map[string]any{"repositories": []string{parts[1]}, "permissions": map[string]string{"contents": "write", "pull_requests": "write", "checks": "read", "statuses": "read", "metadata": "read", "administration": "read"}})
		client := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/app/installations/%d/access_tokens", githubAPIBase, row.InstallationID), strings.NewReader(string(payload)))
		if err != nil {
			return nil, nil, deny
		}
		setGitHubAPIHeaders(req, appJWT)
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			return nil, nil, deny
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
			return nil, nil, deny
		}
		var token struct {
			Token string `json:"token"`
		}
		if json.NewDecoder(io.LimitReader(resp.Body, 64*1024)).Decode(&token) != nil || token.Token == "" {
			return nil, nil, deny
		}
		conn.BaseURL = githubAPIBase
		conn.Token = token.Token
		cleanup = func() { revokeGitHubInstallationToken(client, token.Token) }
	} else {
		return nil, nil, deny
	}
	p, err := materialgit.New(conn)
	if err != nil && cleanup != nil {
		cleanup()
		cleanup = nil
	}
	return p, cleanup, err
}
