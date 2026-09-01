package handler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
)

// enableDeviceAuth points device auth at a workspace slug this test owns and
// restores the handler config afterwards. Each test uses its own slug so two
// of them can never provision into the same workspace.
func enableDeviceAuth(t *testing.T, slug, role string) {
	t.Helper()
	orig := testHandler.cfg
	t.Cleanup(func() { testHandler.cfg = orig })
	testHandler.cfg.DeviceAuthEnabled = true
	testHandler.cfg.DeviceAuthWorkspaceSlug = slug
	testHandler.cfg.DeviceAuthWorkspaceName = "Device Auth Tests"
	testHandler.cfg.DeviceAuthRole = role

	// Registered before the first request so the rows the handler creates are
	// removed even when an assertion fails mid-test. Cleanups run LIFO and
	// issue_status has no foreign key to workspace, so the status delete is
	// registered last to run first — while the workspace row it identifies
	// them by still exists.
	dbfx.Cleanup(t, `DELETE FROM workspace WHERE slug = $1`, slug)
	dbfx.Cleanup(t, `DELETE FROM issue_status WHERE workspace_id IN (SELECT id FROM workspace WHERE slug = $1)`, slug)
}

// callDeviceLogin posts one device login and schedules the device user's
// removal. Handler-created users are outside the suite fixture, so nothing
// else would clean them up.
func callDeviceLogin(t *testing.T, deviceID, deviceName string) *testutil.Response {
	t.Helper()
	dbfx.Cleanup(t, `DELETE FROM "user" WHERE email = $1`, deviceUserEmail(strings.ToLower(deviceID)))
	body := fmt.Sprintf(`{"device_id":%q,"device_name":%q}`, deviceID, deviceName)
	req := httptest.NewRequest(http.MethodPost, "/auth/device", strings.NewReader(body))
	return testutil.Call(t, testHandler.DeviceLogin, req)
}

func deviceWorkspaceID(t *testing.T, slug string) string {
	t.Helper()
	var id string
	dbfx.QueryRow(t, `SELECT id FROM workspace WHERE slug = $1`, slug).Scan(&id)
	return id
}

func deviceMemberRole(t *testing.T, workspaceID, userID string) string {
	t.Helper()
	var role string
	dbfx.QueryRow(t, `SELECT role FROM member WHERE workspace_id = $1 AND user_id = $2`, workspaceID, userID).Scan(&role)
	return role
}

func TestDeviceLoginRejectedWhenSwitchOff(t *testing.T) {
	orig := testHandler.cfg
	t.Cleanup(func() { testHandler.cfg = orig })
	testHandler.cfg.DeviceAuthEnabled = false

	const deviceID = "00112233445566778899aabbccddee00"
	callDeviceLogin(t, deviceID, "off-switch").Want(http.StatusForbidden)

	// The switch has to gate provisioning, not just the response body: a 403
	// that still wrote the user would leave an identity behind on a deployment
	// that never opted in.
	if n := dbfx.Count(t, `SELECT count(*) FROM "user" WHERE email = $1`, deviceUserEmail(deviceID)); n != 0 {
		t.Fatalf("user rows for a rejected device: want 0, got %d", n)
	}
}

func TestDeviceLoginRejectsMalformedDeviceID(t *testing.T) {
	enableDeviceAuth(t, "devauth-malformed", "member")

	cases := []struct {
		name string
		body string
	}{
		{"absent", `{}`},
		{"too_short", `{"device_id":"abc123"}`},
		{"non_hex", `{"device_id":"zzzz2233445566778899aabbccddee00"}`},
		{"too_long", `{"device_id":"` + strings.Repeat("a", 65) + `"}`},
		{"unparsable_body", `{`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/auth/device", strings.NewReader(tc.body))
			testutil.Call(t, testHandler.DeviceLogin, req).Want(http.StatusBadRequest)
		})
	}

	if n := dbfx.Count(t, `SELECT count(*) FROM workspace WHERE slug = $1`, "devauth-malformed"); n != 0 {
		t.Fatalf("rejected requests provisioned a workspace: want 0 rows, got %d", n)
	}
}

func TestDeviceLoginProvisionsIdentityAndWorkspace(t *testing.T) {
	const (
		slug     = "devauth-provision"
		deviceID = "1111222233334444555566667777888a"
	)
	enableDeviceAuth(t, slug, "member")

	var out LoginResponse
	callDeviceLogin(t, deviceID, "artisan@mac-mini").Want(http.StatusOK).JSON(&out)

	if out.Token == "" {
		t.Fatal("token: want a signed session, got empty string")
	}
	if want := deviceUserEmail(deviceID); out.User.Email != want {
		t.Fatalf("user email: want %q, got %q", want, out.User.Email)
	}
	if out.User.Name != "artisan@mac-mini" {
		t.Fatalf("user name: want the client label, got %q", out.User.Name)
	}
	// onboarded_at != null is the only path into the desktop dashboard; a
	// device identity that lands un-onboarded is held behind the onboarding
	// overlay and "opens straight into the app" stops being true.
	if out.User.OnboardedAt == nil {
		t.Fatal("onboarded_at: want a timestamp so the shell lets this user in, got null")
	}

	wsID := deviceWorkspaceID(t, slug)
	if role := deviceMemberRole(t, wsID, out.User.ID); role != "owner" {
		t.Fatalf("role of the device that created the workspace: want owner, got %q", role)
	}
	// An issue cannot be created before its status can be resolved, so the
	// auto-provisioned workspace is only usable with its catalog seeded.
	if n := dbfx.Count(t, `SELECT count(*) FROM issue_status WHERE workspace_id = $1`, wsID); n != 7 {
		t.Fatalf("built-in issue statuses: want 7, got %d", n)
	}
}

func TestDeviceLoginIsIdempotentPerDevice(t *testing.T) {
	const (
		slug     = "devauth-idempotent"
		deviceID = "99998888777766665555444433332222"
	)
	enableDeviceAuth(t, slug, "member")

	var first, second, upper LoginResponse
	callDeviceLogin(t, deviceID, "first-boot").Want(http.StatusOK).JSON(&first)
	callDeviceLogin(t, deviceID, "second-boot").Want(http.StatusOK).JSON(&second)
	// The id is lowercased before it becomes an identity, so a client that
	// reports the same bytes in upper case is the same device, not a new one.
	callDeviceLogin(t, strings.ToUpper(deviceID), "third-boot").Want(http.StatusOK).JSON(&upper)

	if second.User.ID != first.User.ID || upper.User.ID != first.User.ID {
		t.Fatalf("repeat logins resolved to different users: %q, %q, %q",
			first.User.ID, second.User.ID, upper.User.ID)
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM "user" WHERE email = $1`, deviceUserEmail(deviceID)); n != 1 {
		t.Fatalf("user rows after three logins: want 1, got %d", n)
	}
	wsID := deviceWorkspaceID(t, slug)
	if n := dbfx.Count(t, `SELECT count(*) FROM member WHERE workspace_id = $1 AND user_id = $2`, wsID, first.User.ID); n != 1 {
		t.Fatalf("member rows after three logins: want 1, got %d", n)
	}
	// The name is set at creation and not rewritten on every boot — doing so
	// would overwrite whatever the member had chosen to be called.
	if second.User.Name != "first-boot" {
		t.Fatalf("user name after a second boot: want first-boot, got %q", second.User.Name)
	}
}

func TestDeviceLoginSecondDeviceJoinsSharedWorkspace(t *testing.T) {
	const (
		slug    = "devauth-shared"
		deviceA = "aaaa000011112222333344445555666a"
		deviceB = "bbbb000011112222333344445555666b"
	)
	// admin here also covers the configured-role path; owner stays reserved
	// for whichever device brought the workspace into existence.
	enableDeviceAuth(t, slug, "admin")

	var first, secondDevice LoginResponse
	callDeviceLogin(t, deviceA, "machine-a").Want(http.StatusOK).JSON(&first)
	callDeviceLogin(t, deviceB, "machine-b").Want(http.StatusOK).JSON(&secondDevice)

	if first.User.ID == secondDevice.User.ID {
		t.Fatal("two devices collapsed onto one identity: assignment and inbox would stop meaning anything")
	}

	wsID := deviceWorkspaceID(t, slug)
	if n := dbfx.Count(t, `SELECT count(*) FROM workspace WHERE slug = $1`, slug); n != 1 {
		t.Fatalf("workspace rows: want the two devices to share 1, got %d", n)
	}
	if role := deviceMemberRole(t, wsID, first.User.ID); role != "owner" {
		t.Fatalf("first device role: want owner, got %q", role)
	}
	if role := deviceMemberRole(t, wsID, secondDevice.User.ID); role != "admin" {
		t.Fatalf("second device role: want the configured admin, got %q", role)
	}
}

func TestDeviceLoginReusesExistingWorkspace(t *testing.T) {
	const (
		slug     = "devauth-existing"
		deviceID = "cccc000011112222333344445555666c"
	)
	enableDeviceAuth(t, slug, "member")
	// An operator can create the shared workspace by hand before flipping the
	// switch. Joining it must not take ownership from whoever set it up, and
	// its status catalog still has to end up seeded.
	wsID := dbfx.Workspace(t, "Pre-existing Intranet", slug)

	var out LoginResponse
	callDeviceLogin(t, deviceID, "late-joiner").Want(http.StatusOK).JSON(&out)

	if got := deviceWorkspaceID(t, slug); got != wsID {
		t.Fatalf("workspace: want the existing %q reused, got %q", wsID, got)
	}
	if role := deviceMemberRole(t, wsID, out.User.ID); role != "member" {
		t.Fatalf("role in a pre-existing workspace: want the configured member, got %q", role)
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM issue_status WHERE workspace_id = $1`, wsID); n != 7 {
		t.Fatalf("built-in issue statuses seeded into a pre-existing workspace: want 7, got %d", n)
	}
}

func TestDeviceLoginProvisionsWhileSignupDisabled(t *testing.T) {
	const (
		slug     = "devauth-nosignup"
		deviceID = "dddd000011112222333344445555666d"
	)
	enableDeviceAuth(t, slug, "member")
	// ALLOW_SIGNUP governs humans registering themselves. An operator who
	// turned it off has said nothing about the intranet device path, and
	// reading it as a veto here would make the switch unusable on exactly the
	// locked-down deployments it exists for.
	testHandler.cfg.AllowSignup = false

	var out LoginResponse
	callDeviceLogin(t, deviceID, "locked-down").Want(http.StatusOK).JSON(&out)
	if out.Token == "" {
		t.Fatal("token: want a session with signup disabled, got empty string")
	}
}

func TestDeviceUserNameFallsBackToDeviceID(t *testing.T) {
	const deviceID = "0123456789abcdef0123456789abcdef"

	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", "device-01234567"},
		{"whitespace_only", "   \t ", "device-01234567"},
		{"trimmed", "  mac-mini  ", "mac-mini"},
		{"control_chars_removed", "mac mini\nlab", "mac mini lab"},
		{"truncated", strings.Repeat("n", 80), strings.Repeat("n", 60)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := deviceUserName(tc.in, deviceID); got != tc.want {
				t.Fatalf("deviceUserName(%q): want %q, got %q", tc.in, tc.want, got)
			}
		})
	}
}

func TestDeviceAuthConfigDefaults(t *testing.T) {
	cases := []struct {
		name     string
		cfg      Config
		wantSlug string
		wantName string
		wantRole string
	}{
		{"unset", Config{}, "intranet", "Intranet", "member"},
		{
			"custom",
			Config{DeviceAuthWorkspaceSlug: "Office", DeviceAuthWorkspaceName: "Office HQ", DeviceAuthRole: "ADMIN"},
			"office", "Office HQ", "admin",
		},
		// A reserved slug would collide with a global route and a malformed one
		// would fail the insert on somebody's first boot rather than at
		// configuration time. Both fall back to the default instead.
		{"reserved_slug", Config{DeviceAuthWorkspaceSlug: "login"}, "intranet", "Intranet", "member"},
		{"malformed_slug", Config{DeviceAuthWorkspaceSlug: "not a slug!"}, "intranet", "Intranet", "member"},
		// owner belongs to the device that created the workspace, and nothing
		// the member.role CHECK would reject may reach it.
		{"role_owner_not_selectable", Config{DeviceAuthRole: "owner"}, "intranet", "Intranet", "member"},
		{"role_unrecognized", Config{DeviceAuthRole: "superuser"}, "intranet", "Intranet", "member"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.cfg.deviceAuthWorkspaceSlug(); got != tc.wantSlug {
				t.Fatalf("slug: want %q, got %q", tc.wantSlug, got)
			}
			if got := tc.cfg.deviceAuthWorkspaceName(); got != tc.wantName {
				t.Fatalf("name: want %q, got %q", tc.wantName, got)
			}
			if got := tc.cfg.deviceAuthRole(); got != tc.wantRole {
				t.Fatalf("role: want %q, got %q", tc.wantRole, got)
			}
		})
	}
}
