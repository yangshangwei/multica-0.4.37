package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestGetConfigReportsCdnSignedMode(t *testing.T) {
	origStorage := testHandler.Storage
	origSigner := testHandler.CFSigner
	testHandler.Storage = &mockStorage{}
	defer func() {
		testHandler.Storage = origStorage
		testHandler.CFSigner = origSigner
	}()

	fetch := func() AppConfig {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
		w := httptest.NewRecorder()
		testHandler.GetConfig(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("GetConfig: expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var cfg AppConfig
		if err := json.Unmarshal(w.Body.Bytes(), &cfg); err != nil {
			t.Fatalf("decode config: %v", err)
		}
		return cfg
	}

	testHandler.CFSigner = nil
	if cfg := fetch(); cfg.CdnSigned {
		t.Fatalf("cdn_signed: want false without a CloudFront signer, got true")
	}

	// With signing enabled the same cdn_domain serves private content via
	// signed URLs only — clients must be told raw storage URLs won't load.
	testHandler.CFSigner = &auth.CloudFrontSigner{}
	cfg := fetch()
	if !cfg.CdnSigned {
		t.Fatalf("cdn_signed: want true with a CloudFront signer, got false")
	}
	if cfg.CdnDomain != "cdn.example.com" {
		t.Fatalf("cdn_domain: want cdn.example.com alongside cdn_signed, got %q", cfg.CdnDomain)
	}
}

func TestGetConfigIncludesRuntimeAuthConfig(t *testing.T) {
	origStorage := testHandler.Storage
	testHandler.Storage = &mockStorage{}
	defer func() { testHandler.Storage = origStorage }()

	t.Setenv("ALLOW_SIGNUP", "false")
	t.Setenv("GOOGLE_CLIENT_ID", "google-client-id")
	t.Setenv("POSTHOG_API_KEY", "phc_test")
	t.Setenv("POSTHOG_HOST", "https://eu.i.posthog.com")
	t.Setenv("MULTICA_DAEMON_SERVER_URL", "")
	t.Setenv("MULTICA_PUBLIC_URL", "https://api.example.com/")
	t.Setenv("MULTICA_APP_URL", "https://app.example.com/")
	t.Setenv("MULTICA_CLI_INSTALL_COMMAND", " curl -fsSL https://packages.internal/multica/install.sh | bash ")

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	w := httptest.NewRecorder()

	testHandler.GetConfig(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetConfig: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var cfg AppConfig
	if err := json.Unmarshal(w.Body.Bytes(), &cfg); err != nil {
		t.Fatalf("decode config: %v", err)
	}

	if cfg.CdnDomain != "cdn.example.com" {
		t.Fatalf("cdn_domain: want cdn.example.com, got %q", cfg.CdnDomain)
	}
	if cfg.AllowSignup {
		t.Fatalf("allow_signup: want false, got true")
	}
	if cfg.GoogleClientID != "google-client-id" {
		t.Fatalf("google_client_id: want google-client-id, got %q", cfg.GoogleClientID)
	}
	if cfg.PosthogKey != "phc_test" {
		t.Fatalf("posthog_key: want phc_test, got %q", cfg.PosthogKey)
	}
	if cfg.PosthogHost != "https://eu.i.posthog.com" {
		t.Fatalf("posthog_host: want https://eu.i.posthog.com, got %q", cfg.PosthogHost)
	}
	if cfg.AnalyticsEnvironment != "dev" {
		t.Fatalf("analytics_environment: want dev, got %q", cfg.AnalyticsEnvironment)
	}
	if cfg.WorkspaceCreationDisabled {
		t.Fatalf("workspace_creation_disabled: want false by default, got true")
	}
	if cfg.DaemonServerURL != "https://api.example.com" {
		t.Fatalf("daemon_server_url: want https://api.example.com, got %q", cfg.DaemonServerURL)
	}
	if cfg.DaemonAppURL != "https://app.example.com" {
		t.Fatalf("daemon_app_url: want https://app.example.com, got %q", cfg.DaemonAppURL)
	}
	if cfg.CliInstallCommand != "curl -fsSL https://packages.internal/multica/install.sh | bash" {
		t.Fatalf("cli_install_command: got %q", cfg.CliInstallCommand)
	}
}

func TestGetConfigPlatformInstallCommands(t *testing.T) {
	t.Setenv("MULTICA_CLI_INSTALL_COMMAND", " legacy installer ")
	t.Setenv("MULTICA_CLI_INSTALL_COMMAND_WINDOWS", " windows installer ")
	t.Setenv("MULTICA_CLI_INSTALL_COMMAND_MACOS", " macos installer ")
	t.Setenv("MULTICA_CLI_INSTALL_COMMAND_LINUX", " linux installer ")
	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	var cfg map[string]json.RawMessage
	testutil.Call(t, testHandler.GetConfig, req).Want(http.StatusOK).JSON(&cfg)
	var commands map[string]string
	if err := json.Unmarshal(cfg["cli_install_commands"], &commands); err != nil {
		t.Fatalf("decode platform commands: %v", err)
	}
	want := map[string]string{"windows": "windows installer", "macos": "macos installer", "linux": "linux installer"}
	if !reflect.DeepEqual(commands, want) {
		t.Fatalf("platform commands: got %v, want %v", commands, want)
	}
	if string(cfg["cli_install_command"]) != `"legacy installer"` {
		t.Fatalf("legacy command must remain available: %s", cfg["cli_install_command"])
	}

	t.Setenv("MULTICA_CLI_INSTALL_COMMAND_WINDOWS", "")
	t.Setenv("MULTICA_CLI_INSTALL_COMMAND_MACOS", "  ")
	cfg = nil
	testutil.Call(t, testHandler.GetConfig, req).Want(http.StatusOK).JSON(&cfg)
	commands = nil
	if err := json.Unmarshal(cfg["cli_install_commands"], &commands); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(commands, map[string]string{"linux": "linux installer"}) {
		t.Fatalf("empty platform commands must be omitted: %v", commands)
	}

	t.Setenv("MULTICA_CLI_INSTALL_COMMAND_LINUX", "")
	cfg = nil
	testutil.Call(t, testHandler.GetConfig, req).Want(http.StatusOK).JSON(&cfg)
	if _, present := cfg["cli_install_commands"]; present {
		t.Fatal("unconfigured platform commands must be omitted")
	}
}

func TestGetConfigDesktopDownloadURL(t *testing.T) {
	cases := []struct {
		name string
		env  string
		want string
	}{
		{name: "unset", env: "", want: ""},
		{name: "blank", env: "   ", want: ""},
		{name: "intranet http", env: " http://10.10.10.20:18080/desktop/ ", want: "http://10.10.10.20:18080/desktop/"},
		{name: "https with path", env: "https://downloads.internal/desktop/", want: "https://downloads.internal/desktop/"},
		{name: "script scheme", env: "javascript:alert(1)", want: ""},
		{name: "relative path", env: "/desktop/", want: ""},
		{name: "missing host", env: "http:///desktop/", want: ""},
		{name: "credentials", env: "http://admin:secret@10.10.10.20:18080/desktop/", want: ""},
		{name: "ipv6 literal", env: "http://[::1]:18080/desktop/", want: "http://[::1]:18080/desktop/"},
		{name: "underscore host", env: "http://dl_srv:18080/desktop/", want: "http://dl_srv:18080/desktop/"},
		{name: "max port", env: "http://dl.internal:65535/desktop/", want: "http://dl.internal:65535/desktop/"},
		// Go's URL parser keeps these; the web client's URL parser rejects
		// them, so the server must drop them too and warn at startup.
		{name: "ipv4 octet out of range", env: "http://10.10.10.256:18080/desktop/", want: ""},
		{name: "ipv4 too many parts", env: "http://1.2.3.4.5/desktop/", want: ""},
		{name: "short ipv4", env: "http://10.1/desktop/", want: ""},
		{name: "hex ipv4 label", env: "http://dl.0x1/desktop/", want: ""},
		{name: "port out of range", env: "http://dl.internal:180800/desktop/", want: ""},
		{name: "forbidden host character", env: "http://dl<srv/desktop/", want: ""},
		{name: "non-breaking space in host", env: "http://dl\u00a0srv/desktop/", want: ""},
		{name: "idn host", env: "http://下载.内网/desktop/", want: ""},
		{name: "punycode host", env: "http://xn--fiqs8s/desktop/", want: ""},
		{name: "empty label", env: "http://dl..internal/desktop/", want: ""},
		{name: "ipv6 zone", env: "http://[fe80::1%25en0]/desktop/", want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("MULTICA_DESKTOP_DOWNLOAD_URL", tc.env)
			var cfg map[string]json.RawMessage
			testutil.Call(t, testHandler.GetConfig, httptest.NewRequest(http.MethodGet, "/api/config", nil)).Want(http.StatusOK).JSON(&cfg)
			raw, present := cfg["desktop_download_url"]
			if tc.want == "" {
				if present {
					t.Fatalf("desktop_download_url must be omitted for %q, got %s", tc.env, raw)
				}
				return
			}
			var got string
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatalf("decode desktop_download_url: %v", err)
			}
			if got != tc.want {
				t.Fatalf("desktop_download_url: got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestGetConfigPreservesServerURLWithoutAppURL(t *testing.T) {
	t.Setenv("MULTICA_DAEMON_SERVER_URL", " https://api.internal.example/ ")
	t.Setenv("MULTICA_PUBLIC_URL", "")
	t.Setenv("MULTICA_APP_URL", "")
	t.Setenv("FRONTEND_ORIGIN", "")
	var cfg AppConfig
	testutil.Call(t, testHandler.GetConfig, httptest.NewRequest(http.MethodGet, "/api/config", nil)).Want(http.StatusOK).JSON(&cfg)
	if cfg.DaemonServerURL != "https://api.internal.example" || cfg.DaemonAppURL != "" {
		t.Fatalf("incomplete self-host config must survive for client validation: server=%q app=%q", cfg.DaemonServerURL, cfg.DaemonAppURL)
	}
}

func TestGetConfigUsesDaemonServerURLOverride(t *testing.T) {
	t.Setenv("MULTICA_DAEMON_SERVER_URL", " https://api.internal.example/// ")
	t.Setenv("MULTICA_PUBLIC_URL", "https://hooks.example.com/")
	t.Setenv("MULTICA_APP_URL", "https://app.example.com/")

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	var cfg AppConfig
	testutil.Call(t, testHandler.GetConfig, req).Want(http.StatusOK).JSON(&cfg)
	if cfg.DaemonServerURL != "https://api.internal.example" {
		t.Fatalf("daemon_server_url: want override URL, got %q", cfg.DaemonServerURL)
	}
	if cfg.DaemonAppURL != "https://app.example.com" {
		t.Fatalf("daemon_app_url: want app URL, got %q", cfg.DaemonAppURL)
	}
}

func TestGetConfigHonorsVCSIntegrationSwitch(t *testing.T) {
	origCfg := testHandler.cfg
	t.Cleanup(func() { testHandler.cfg = origCfg })

	fetch := func() map[string]json.RawMessage {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
		w := httptest.NewRecorder()
		testHandler.GetConfig(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("GetConfig: expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var cfg map[string]json.RawMessage
		if err := json.Unmarshal(w.Body.Bytes(), &cfg); err != nil {
			t.Fatalf("decode config: %v", err)
		}
		return cfg
	}

	testHandler.cfg.VCSIntegrationEnabled = false
	if _, ok := fetch()["vcs_integration_available"]; ok {
		t.Fatal("vcs_integration_available must be omitted when the integration is disabled")
	}

	testHandler.cfg.VCSIntegrationEnabled = true
	raw, ok := fetch()["vcs_integration_available"]
	if !ok {
		t.Fatal("vcs_integration_available must be present when the integration is enabled")
	}
	if string(raw) != "true" {
		t.Fatalf("vcs_integration_available: want true, got %s", raw)
	}
}

func TestGetConfigUsesAppURLForSameOriginDaemonSetup(t *testing.T) {
	t.Setenv("MULTICA_DAEMON_SERVER_URL", "")
	t.Setenv("MULTICA_PUBLIC_URL", "")
	t.Setenv("MULTICA_APP_URL", "https://multica.internal.example/")

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	w := httptest.NewRecorder()

	testHandler.GetConfig(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetConfig: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var cfg AppConfig
	if err := json.Unmarshal(w.Body.Bytes(), &cfg); err != nil {
		t.Fatalf("decode config: %v", err)
	}
	if cfg.DaemonServerURL != "https://multica.internal.example" {
		t.Fatalf("daemon_server_url: want same-origin URL, got %q", cfg.DaemonServerURL)
	}
	if cfg.DaemonAppURL != "https://multica.internal.example" {
		t.Fatalf("daemon_app_url: want app URL, got %q", cfg.DaemonAppURL)
	}
}

func TestGetConfigUsesFrontendOriginForSameOriginDaemonSetup(t *testing.T) {
	t.Setenv("MULTICA_DAEMON_SERVER_URL", "")
	t.Setenv("MULTICA_PUBLIC_URL", "")
	t.Setenv("MULTICA_APP_URL", "")
	t.Setenv("FRONTEND_ORIGIN", "https://multica.internal.example/")

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	w := httptest.NewRecorder()

	testHandler.GetConfig(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetConfig: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var cfg AppConfig
	if err := json.Unmarshal(w.Body.Bytes(), &cfg); err != nil {
		t.Fatalf("decode config: %v", err)
	}
	if cfg.DaemonServerURL != "https://multica.internal.example" {
		t.Fatalf("daemon_server_url: want same-origin URL, got %q", cfg.DaemonServerURL)
	}
	if cfg.DaemonAppURL != "https://multica.internal.example" {
		t.Fatalf("daemon_app_url: want frontend origin, got %q", cfg.DaemonAppURL)
	}
}

func TestGetConfigOmitsOfficialCloudDaemonSetup(t *testing.T) {
	t.Setenv("MULTICA_PUBLIC_URL", "https://api.multica.ai")
	t.Setenv("MULTICA_APP_URL", "")
	t.Setenv("FRONTEND_ORIGIN", "https://multica.ai")

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	w := httptest.NewRecorder()

	testHandler.GetConfig(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetConfig: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var cfg AppConfig
	if err := json.Unmarshal(w.Body.Bytes(), &cfg); err != nil {
		t.Fatalf("decode config: %v", err)
	}
	if cfg.DaemonServerURL != "" {
		t.Fatalf("daemon_server_url: want omitted for cloud, got %q", cfg.DaemonServerURL)
	}
	if cfg.DaemonAppURL != "" {
		t.Fatalf("daemon_app_url: want omitted for cloud, got %q", cfg.DaemonAppURL)
	}
}

// TestGetConfigOmitsCloudDaemonSetupWithoutPublicURL reproduces the production
// regression behind the broken "Add a computer" command: the official cloud
// frontend is multica.ai, but the deployment does not set MULTICA_PUBLIC_URL to
// the api host. Previously this fell through to the same-origin branch and
// emitted daemon_server_url=https://multica.ai, which the dialog turned into
// `multica setup self-host --server-url https://multica.ai` — pointing the
// daemon's backend at the frontend (no /health, no WebSocket proxy). The
// official cloud must be recognised by its frontend host alone so the daemon
// setup URLs are omitted and the dialog falls back to `multica setup`.
func TestGetConfigOmitsCloudDaemonSetupWithoutPublicURL(t *testing.T) {
	t.Setenv("MULTICA_PUBLIC_URL", "")
	t.Setenv("MULTICA_APP_URL", "")
	t.Setenv("FRONTEND_ORIGIN", "https://multica.ai")

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	w := httptest.NewRecorder()

	testHandler.GetConfig(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetConfig: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var cfg AppConfig
	if err := json.Unmarshal(w.Body.Bytes(), &cfg); err != nil {
		t.Fatalf("decode config: %v", err)
	}
	if cfg.DaemonServerURL != "" {
		t.Fatalf("daemon_server_url: want omitted for official cloud, got %q", cfg.DaemonServerURL)
	}
	if cfg.DaemonAppURL != "" {
		t.Fatalf("daemon_app_url: want omitted for official cloud, got %q", cfg.DaemonAppURL)
	}
}

// TestGetConfigOmitsCloudDaemonSetupForConfiguredAppURL covers the official
// cloud frontend when it is configured through MULTICA_APP_URL.
func TestGetConfigOmitsCloudDaemonSetupForConfiguredAppURL(t *testing.T) {
	t.Setenv("MULTICA_PUBLIC_URL", "")
	t.Setenv("MULTICA_APP_URL", "https://multica.ai")
	t.Setenv("FRONTEND_ORIGIN", "")

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	w := httptest.NewRecorder()

	testHandler.GetConfig(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetConfig: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var cfg AppConfig
	if err := json.Unmarshal(w.Body.Bytes(), &cfg); err != nil {
		t.Fatalf("decode config: %v", err)
	}
	if cfg.DaemonServerURL != "" {
		t.Fatalf("daemon_server_url: want omitted for official cloud, got %q", cfg.DaemonServerURL)
	}
	if cfg.DaemonAppURL != "" {
		t.Fatalf("daemon_app_url: want omitted for official cloud, got %q", cfg.DaemonAppURL)
	}
}

func TestURLHostEqualsCanonicalizesCommonHostForms(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "full URL", raw: "https://api.multica.ai", want: true},
		{name: "bare host", raw: "api.multica.ai", want: true},
		{name: "host port", raw: "api.multica.ai:8080", want: true},
		{name: "trailing dot", raw: "https://api.multica.ai.", want: true},
		{name: "different host", raw: "https://evil.example", want: false},
		{name: "empty", raw: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := urlHostEquals(tt.raw, "api.multica.ai"); got != tt.want {
				t.Fatalf("urlHostEquals(%q): want %v, got %v", tt.raw, tt.want, got)
			}
		})
	}
}

// TestGetConfigExposesWorkspaceCreationDisabled verifies that the self-host
// gate added by #3433 surfaces to the frontend through /api/config so the UI
// can hide every "Create workspace" affordance.
func TestGetConfigExposesWorkspaceCreationDisabled(t *testing.T) {
	origStorage := testHandler.Storage
	testHandler.Storage = &mockStorage{}
	defer func() { testHandler.Storage = origStorage }()

	t.Setenv("DISABLE_WORKSPACE_CREATION", "true")

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	w := httptest.NewRecorder()

	testHandler.GetConfig(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetConfig: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var cfg AppConfig
	if err := json.Unmarshal(w.Body.Bytes(), &cfg); err != nil {
		t.Fatalf("decode config: %v", err)
	}
	if !cfg.WorkspaceCreationDisabled {
		t.Fatalf("workspace_creation_disabled: want true with env on, got false (body=%s)", w.Body.String())
	}
}

// TestGetConfigExposesServerVersion verifies /api/config reports the build
// version main.go stamps into handler.Config via -X main.version on a
// self-hosted deployment, so operators can confirm what's deployed from the
// Help popover.
func TestGetConfigExposesServerVersion(t *testing.T) {
	origCfg := testHandler.cfg
	defer func() { testHandler.cfg = origCfg }()

	// Self-hosted frontend origin: the version row is meant for these deployments.
	t.Setenv("MULTICA_APP_URL", "https://multica.self-hosted.example")
	t.Setenv("FRONTEND_ORIGIN", "")

	testHandler.cfg.ServerVersion = ""
	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	w := httptest.NewRecorder()
	testHandler.GetConfig(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetConfig: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var cfg AppConfig
	if err := json.Unmarshal(w.Body.Bytes(), &cfg); err != nil {
		t.Fatalf("decode config: %v", err)
	}
	if cfg.ServerVersion != "" {
		t.Fatalf("server_version: want empty for dev build, got %q", cfg.ServerVersion)
	}

	testHandler.cfg.ServerVersion = "1.2.3"
	w = httptest.NewRecorder()
	testHandler.GetConfig(w, req)
	if err := json.Unmarshal(w.Body.Bytes(), &cfg); err != nil {
		t.Fatalf("decode config: %v", err)
	}
	if cfg.ServerVersion != "1.2.3" {
		t.Fatalf("server_version: want 1.2.3, got %q", cfg.ServerVersion)
	}
}

// TestGetConfigOmitsServerVersionOnOfficialCloud verifies the build version is
// suppressed on the managed cloud (frontend host multica.ai) even when the
// binary is stamped, while a self-hosted frontend origin still reports it. The
// managed cloud is continuously deployed, so its users don't need the row.
func TestGetConfigOmitsServerVersionOnOfficialCloud(t *testing.T) {
	origCfg := testHandler.cfg
	defer func() { testHandler.cfg = origCfg }()
	testHandler.cfg.ServerVersion = "1.2.3"

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)

	// Official cloud: frontend host multica.ai -> version omitted.
	t.Setenv("MULTICA_APP_URL", "https://multica.ai")
	t.Setenv("FRONTEND_ORIGIN", "")
	w := httptest.NewRecorder()
	testHandler.GetConfig(w, req)
	var cfg AppConfig
	if err := json.Unmarshal(w.Body.Bytes(), &cfg); err != nil {
		t.Fatalf("decode config: %v", err)
	}
	if cfg.ServerVersion != "" {
		t.Fatalf("server_version: want omitted on official cloud, got %q", cfg.ServerVersion)
	}

	// Self-hosted: operator's own frontend origin -> version reported.
	t.Setenv("MULTICA_APP_URL", "https://multica.self-hosted.example")
	w = httptest.NewRecorder()
	testHandler.GetConfig(w, req)
	if err := json.Unmarshal(w.Body.Bytes(), &cfg); err != nil {
		t.Fatalf("decode config: %v", err)
	}
	if cfg.ServerVersion != "1.2.3" {
		t.Fatalf("server_version: want 1.2.3 on self-hosted, got %q", cfg.ServerVersion)
	}
}

func TestGetConfigExposesFrontendFeatureFlags(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	w := httptest.NewRecorder()
	h.GetConfig(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetConfig default flags: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var cfg AppConfig
	if err := json.Unmarshal(w.Body.Bytes(), &cfg); err != nil {
		t.Fatalf("decode default config: %v", err)
	}
	if cfg.FeatureFlags["composio_mcp_apps"] {
		t.Fatalf("composio_mcp_apps: want false by default, got true")
	}
	if cfg.FeatureFlags["billing_workspace_subscriptions"] {
		t.Fatalf("billing_workspace_subscriptions: want false by default, got true")
	}
	if cfg.FeatureFlags["plugins_v1"] {
		t.Fatalf("plugins_v1: want false by default, got true")
	}
	for _, retired := range []string{"private_plugins_v1", "remote_mcp_plugins_v1"} {
		if _, published := cfg.FeatureFlags[retired]; published {
			t.Fatalf("retired Plugin sub-flag %q must not be published", retired)
		}
	}
	if !cfg.FeatureFlags["agents_skill_toggles"] {
		t.Fatalf("agents_skill_toggles: want true for installed v0.4.0 clients, got false")
	}
	if !cfg.FeatureFlags["settings_resource_labels"] {
		t.Fatalf("settings_resource_labels: want true for installed clients, got false")
	}
	// Deliberately unpublished: pre-v0.4.33 clients gate their "New status"
	// button on this key and fail closed, which is how a client that predates
	// the v0.4.31 rendering fixes is kept from creating one. See
	// featureflags.TestCustomIssueStatusesIsNotPublished.
	if _, published := cfg.FeatureFlags["custom_issue_statuses"]; published {
		t.Fatalf("custom_issue_statuses: want unpublished, got %v", cfg.FeatureFlags["custom_issue_statuses"])
	}

	withComposioMCPAppsFlag(t, h, true)
	w = httptest.NewRecorder()
	h.GetConfig(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetConfig enabled flags: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &cfg); err != nil {
		t.Fatalf("decode enabled config: %v", err)
	}
	if !cfg.FeatureFlags["composio_mcp_apps"] {
		t.Fatalf("composio_mcp_apps: want true with flag enabled, got false")
	}
}

func TestGetConfigExposesEnabledPluginsV1Flag(t *testing.T) {
	h := &Handler{}
	withPluginsV1Flag(t, h, true)

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	w := httptest.NewRecorder()
	h.GetConfig(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetConfig enabled plugins_v1: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var cfg AppConfig
	if err := json.Unmarshal(w.Body.Bytes(), &cfg); err != nil {
		t.Fatalf("decode enabled config: %v", err)
	}
	if !cfg.FeatureFlags["plugins_v1"] {
		t.Fatal("plugins_v1: want true with flag enabled, got false")
	}
}

// Clients fail closed on this flag: absent covers every server that predates
// the signal, including the ones that accept a worktree resource, silently drop
// execution_mode and run the task in the user's working copy (#7113). A build
// that HAS the save gate therefore has to say so,
// unconditionally — not behind a deployment check, an env var or a feature
// flag, all of which would disable worktree mode for the users who can run it.
func TestGetConfigDeclaresLocalWorktreeSupport(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	w := httptest.NewRecorder()
	testHandler.GetConfig(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetConfig: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var cfg AppConfig
	if err := json.Unmarshal(w.Body.Bytes(), &cfg); err != nil {
		t.Fatalf("decode config: %v", err)
	}
	if !cfg.LocalWorktreeSupported {
		t.Fatal("this build runs the worktree save gate but does not advertise it; clients will hide the mode")
	}
	// Serialised as a real key, not omitted when false-by-accident: the client
	// distinguishes "absent" (old server) from an explicit answer.
	var raw map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode raw config: %v", err)
	}
	if _, ok := raw["local_worktree_supported"]; !ok {
		t.Fatal("local_worktree_supported missing from the JSON body")
	}
}

// Web/Desktop can run ahead of a manually deployed backend. Handlers that
// predate conversation_starters ignore the unknown JSON field and still answer 200,
// so clients must see an explicit declaration before sending either create or
// update writes that contain it.
func TestGetConfigDeclaresAgentConversationStartersSupport(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	w := httptest.NewRecorder()
	testHandler.GetConfig(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetConfig: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var cfg AppConfig
	if err := json.Unmarshal(w.Body.Bytes(), &cfg); err != nil {
		t.Fatalf("decode config: %v", err)
	}
	if !cfg.AgentConversationStartersSupported {
		t.Fatal("this build persists conversation_starters but does not advertise the contract")
	}
	var raw map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode raw config: %v", err)
	}
	if _, ok := raw["agent_conversation_starters_supported"]; !ok {
		t.Fatal("agent_conversation_starters_supported missing from the JSON body")
	}
}

// Device auth is a per-deployment switch, and a client cannot discover it any
// other way: the endpoint itself answers 403 when it is off, which is
// indistinguishable from "this server is older than the feature". So the
// declaration has to be here, and it has to be absent — not false — when the
// switch is off, keeping the managed cloud's response shape unchanged.
func TestGetConfigDeclaresDeviceAuthOnlyWhenEnabled(t *testing.T) {
	orig := testHandler.cfg.DeviceAuthEnabled
	t.Cleanup(func() { testHandler.cfg.DeviceAuthEnabled = orig })

	fetch := func() (AppConfig, map[string]any) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
		w := httptest.NewRecorder()
		testHandler.GetConfig(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("GetConfig: expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var cfg AppConfig
		if err := json.Unmarshal(w.Body.Bytes(), &cfg); err != nil {
			t.Fatalf("decode config: %v", err)
		}
		var raw map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
			t.Fatalf("decode raw config: %v", err)
		}
		return cfg, raw
	}

	testHandler.cfg.DeviceAuthEnabled = false
	cfg, raw := fetch()
	if cfg.DeviceAuthAvailable {
		t.Fatal("device_auth_available: want false with the switch off, got true")
	}
	if _, present := raw["device_auth_available"]; present {
		t.Fatal("device_auth_available must be omitted while the switch is off")
	}

	testHandler.cfg.DeviceAuthEnabled = true
	cfg, raw = fetch()
	if !cfg.DeviceAuthAvailable {
		t.Fatal("device_auth_available: want true with the switch on, got false")
	}
	if _, present := raw["device_auth_available"]; !present {
		t.Fatal("device_auth_available missing from the JSON body while the switch is on")
	}
}
