package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/analytics"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/handler"
	"github.com/multica-ai/multica/server/internal/integrations/channel"
	"github.com/multica-ai/multica/server/internal/integrations/channel/engine"
	"github.com/multica-ai/multica/server/internal/realtime"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util/secretbox"
)

var messagingProviderNames = []string{"lark", "slack", "dingtalk", "wecom", "telegram"}

func configureMessagingTestKeys(t *testing.T, configured bool) {
	t.Helper()
	key := ""
	if configured {
		key = base64.StdEncoding.EncodeToString(make([]byte, secretbox.KeySize))
	}
	for _, provider := range messagingProviderNames {
		t.Setenv("MULTICA_"+strings.ToUpper(provider)+"_SECRET_KEY", key)
	}
	// Backfills on an enabled router must never contact a real Lark endpoint.
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	t.Cleanup(api.Close)
	t.Setenv("MULTICA_LARK_HTTP_BASE_URL", api.URL)
	t.Setenv("MULTICA_LARK_CALLBACK_BASE_URL", api.URL)
}

func messagingTestRouter(t *testing.T) (http.Handler, *handler.Handler) {
	t.Helper()
	t.Setenv("CHANNEL_WS_LEASE_BACKEND", "postgres")
	router, h := NewRouterWithOptions(testPool, realtime.NewHub(), events.New(), analytics.NoopClient{}, nil, RouterOptions{})
	t.Cleanup(func() {
		if !h.ChannelRouter.Drain(context.Background()) {
			t.Error("channel router did not drain")
		}
	})
	return router, h
}

func messagingRequest(method, path string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+testToken)
	req.Header.Set("X-Workspace-ID", testWorkspaceID)
	req.Header.Set("Content-Type", "application/json")
	return req
}

func TestMessagingIntegrationsRuntimeConfig(t *testing.T) {
	configureMessagingTestKeys(t, false)
	for _, tc := range []struct {
		name, value string
		want        bool
	}{
		{"default_enabled", "", true},
		{"explicitly_enabled", "true", true},
		{"disabled", "false", false},
		{"disabled_numeric", "0", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("MULTICA_MESSAGING_INTEGRATIONS_ENABLED", tc.value)
			router, _ := messagingTestRouter(t)
			var config map[string]json.RawMessage
			testutil.Call(t, router.ServeHTTP, httptest.NewRequest(http.MethodGet, "/api/config", nil)).Want(http.StatusOK).JSON(&config)
			got, ok := config["messaging_integrations_enabled"]
			if !ok || string(got) != strconv.FormatBool(tc.want) {
				t.Fatalf("messaging_integrations_enabled = %s (present=%v), want explicit %v", got, ok, tc.want)
			}
		})
	}
}

func TestMessagingIntegrationsDisabledWithConfiguredCredentials(t *testing.T) {
	configureMessagingTestKeys(t, true)
	t.Setenv("MULTICA_MESSAGING_INTEGRATIONS_ENABLED", "false")
	t.Setenv("MULTICA_VCS_INTEGRATION_ENABLED", "true")
	t.Setenv("MULTICA_VCS_SECRET_KEY", base64.StdEncoding.EncodeToString(make([]byte, secretbox.KeySize)))
	router, h := messagingTestRouter(t)
	if h.LarkInstallations != nil || h.LarkRegistration != nil || h.LarkBindingTokens != nil || h.LarkAPIClient != nil ||
		h.SlackInstall != nil || h.SlackBindingTokens != nil || h.SlackHistory != nil ||
		h.DingTalkInstall != nil || h.DingTalkBindingTokens != nil ||
		h.WecomStore != nil || h.WecomCredentials != nil || h.WecomBindingTokens != nil ||
		h.TelegramInstall != nil || h.TelegramBindingTokens != nil || h.TelegramOutbound != nil {
		t.Fatal("disabled messaging must not wire provider services, clients, binding tokens, or outbound workers even with encryption keys configured")
	}

	fx := testutil.New(testPool, testWorkspaceID, testUserID)
	agentID := fx.Agent(t, "Disabled messaging agent", "")
	for _, provider := range messagingProviderNames {
		t.Run(provider, func(t *testing.T) {
			channelType := provider
			if provider == "lark" {
				channelType = "feishu"
			}
			installationID := fx.Insert(t, "channel_installation", testutil.Cols{
				"workspace_id":      testWorkspaceID,
				"agent_id":          agentID,
				"channel_type":      channelType,
				"config":            []byte(`{"retained":"encrypted-credentials"}`),
				"installer_user_id": testUserID,
				"status":            "active",
			})
			path := "/api/workspaces/" + testWorkspaceID + "/" + provider
			var list struct {
				Configured    bool              `json:"configured"`
				Installations []json.RawMessage `json:"installations"`
			}
			testutil.Call(t, router.ServeHTTP, messagingRequest(http.MethodGet, path+"/installations")).Want(http.StatusOK).JSON(&list)
			if list.Configured || len(list.Installations) != 0 {
				t.Fatalf("disabled provider list = %+v, want unconfigured with no installations", list)
			}
			err := h.ChannelRouter.Handle(context.Background(), channel.InboundMessage{
				Source: channel.Source{ChannelType: channel.Type(channelType)},
			})
			if !errors.Is(err, engine.ErrNoResolverSet) {
				t.Fatalf("disabled provider inbound pipeline error = %v, want no resolver", err)
			}
			installPath := path + "/install/byo"
			if provider == "lark" {
				installPath = path + "/install/begin"
			} else if provider == "telegram" {
				installPath = path + "/install"
			}
			for _, endpoint := range []struct{ method, path string }{
				{http.MethodPost, installPath},
				{http.MethodDelete, path + "/installations/" + installationID},
				{http.MethodPost, "/api/" + provider + "/binding/redeem"},
			} {
				testutil.Call(t, router.ServeHTTP, messagingRequest(endpoint.method, endpoint.path)).Want(http.StatusServiceUnavailable)
			}
			var status string
			var config json.RawMessage
			fx.QueryRow(t, "SELECT status, config FROM channel_installation WHERE id = $1", installationID).Scan(&status, &config)
			if status != "active" || !strings.Contains(string(config), "encrypted-credentials") {
				t.Fatalf("saved installation changed while disabled: status=%s config=%s", status, config)
			}
		})
	}

	var vcs struct {
		Available  bool `json:"available"`
		Configured bool `json:"configured"`
	}
	testutil.Call(t, router.ServeHTTP, messagingRequest(http.MethodGet, "/api/workspaces/"+testWorkspaceID+"/vcs/connections")).Want(http.StatusOK).JSON(&vcs)
	if h.VCSSecretBox == nil || !vcs.Available || !vcs.Configured {
		t.Fatalf("intranet Git must remain available and configured: %+v", vcs)
	}
}

func TestMessagingIntegrationsEnabledWithConfiguredCredentials(t *testing.T) {
	configureMessagingTestKeys(t, true)
	t.Setenv("MULTICA_MESSAGING_INTEGRATIONS_ENABLED", "true")
	router, h := messagingTestRouter(t)
	if h.LarkInstallations == nil || h.SlackInstall == nil || h.DingTalkInstall == nil || h.WecomStore == nil || h.TelegramInstall == nil || h.TelegramOutbound == nil {
		t.Fatal("enabled messaging must retain all five providers")
	}
	for _, provider := range messagingProviderNames {
		var list struct {
			Configured bool `json:"configured"`
		}
		testutil.Call(t, router.ServeHTTP, messagingRequest(http.MethodGet, "/api/workspaces/"+testWorkspaceID+"/"+provider+"/installations")).Want(http.StatusOK).JSON(&list)
		if !list.Configured {
			t.Errorf("%s must remain configured when messaging is enabled", provider)
		}
	}
}
