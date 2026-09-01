package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/multica-ai/multica/server/internal/analytics"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/issuestatus"
	"github.com/multica-ai/multica/server/internal/logger"
	obsmetrics "github.com/multica-ai/multica/server/internal/metrics"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Device auth is the intranet login path: a client that can present a stable
// device id gets a session without a mail relay, an OAuth provider, or a human
// typing anything. See Config.DeviceAuthEnabled for what enabling it costs.
//
// The identity is per device, not one shared account, because everything above
// the auth layer is built on distinguishable actors: an issue has an assignee,
// a comment has an author, an inbox belongs to someone. Collapsing every
// client onto one user would keep the app running and make all of that
// meaningless.
const (
	defaultDeviceAuthWorkspaceSlug = "intranet"
	defaultDeviceAuthWorkspaceName = "Intranet"
	defaultDeviceAuthRole          = "member"

	// deviceUserEmailDomain carries the device→user mapping in the existing
	// "user".email unique index instead of a new table: the uniqueness that
	// makes provisioning idempotent is already there, and a device identity
	// needs no column of its own. The .local suffix cannot be a real mailbox,
	// so no code path can ever try to deliver to one of these addresses, and
	// it is the marker to filter on when cleaning device users up.
	deviceUserEmailDomain = "device.multica.local"

	// deviceAuthSignupSource labels the signup event. The cookie-based
	// attribution signupSourceFromRequest reads is a web-frontend artifact
	// that this path never has.
	deviceAuthSignupSource = "device_auth"

	// maxDeviceNameRunes bounds the client-supplied label. It lands in a
	// member list and an avatar tooltip, not in a mail Subject, so this only
	// has to stop a device from writing an essay into every issue it touches.
	maxDeviceNameRunes = 60
)

const deviceIDFormatError = "device_id must be 32 to 64 hexadecimal characters"

// deviceIDPattern is deliberately narrow: the id is interpolated into the
// synthetic email below, so anything that is not lowercase hex has no business
// reaching the users table.
var deviceIDPattern = regexp.MustCompile(`^[0-9a-f]{32,64}$`)

// deviceNameControlChars strips control and format characters from the
// client-supplied label — a device could otherwise embed a newline or a
// bidi override in a name that renders in every other member's UI.
var deviceNameControlChars = regexp.MustCompile(`[\p{Cc}\p{Cf}]+`)

// errDeviceProvisionRace reports that a concurrent first boot inserted the row
// this pass had just read as absent. It is a retry signal, never a client
// error: every insert in provisionDeviceIdentity is guarded by a read.
var errDeviceProvisionRace = errors.New("device provisioning lost a race")

type DeviceLoginRequest struct {
	DeviceID string `json:"device_id"`
	// DeviceName is the label other members see. The desktop app sends
	// "<os user>@<hostname>"; empty falls back to the device id's head.
	DeviceName string `json:"device_name"`
}

// deviceAuthWorkspaceSlug returns the effective shared-workspace slug. An
// operator-supplied value passes the same gate as a user-created one: a
// reserved slug would collide with a global route, and a malformed one would
// fail the insert on somebody's first boot instead of at configuration time.
func (c Config) deviceAuthWorkspaceSlug() string {
	slug := strings.ToLower(strings.TrimSpace(c.DeviceAuthWorkspaceSlug))
	if slug == "" || !workspaceSlugPattern.MatchString(slug) || isReservedSlug(slug) {
		return defaultDeviceAuthWorkspaceSlug
	}
	return slug
}

func (c Config) deviceAuthWorkspaceName() string {
	if name := strings.TrimSpace(c.DeviceAuthWorkspaceName); name != "" {
		return name
	}
	return defaultDeviceAuthWorkspaceName
}

// deviceAuthRole is the role for every device except the workspace creator.
// "owner" is not selectable here — it belongs to whoever provisioned the
// workspace — and an unrecognized value must not reach the member.role CHECK
// constraint, where it would surface as a 500 on a first boot.
func (c Config) deviceAuthRole() string {
	if strings.EqualFold(strings.TrimSpace(c.DeviceAuthRole), "admin") {
		return "admin"
	}
	return defaultDeviceAuthRole
}

func deviceUserEmail(deviceID string) string {
	return fmt.Sprintf("device-%s@%s", deviceID, deviceUserEmailDomain)
}

// deviceUserName resolves the display name. It never falls back to the
// synthetic email: that address is an implementation detail of the mapping,
// and it would otherwise show up as a person's name everywhere.
func deviceUserName(requested, deviceID string) string {
	name := deviceNameControlChars.ReplaceAllString(requested, " ")
	name = strings.Join(strings.Fields(name), " ")
	if runes := []rune(name); len(runes) > maxDeviceNameRunes {
		name = strings.TrimSpace(string(runes[:maxDeviceNameRunes]))
	}
	if name == "" {
		return "device-" + deviceID[:8]
	}
	return name
}

// deviceProvisionResult records what one pass actually established, so the
// handler reports signup, workspace creation and daemon refresh on the edges
// where they happened rather than on every login.
type deviceProvisionResult struct {
	user             db.User
	workspaceID      string
	createdUser      bool
	createdWorkspace bool
	createdMember    bool
}

// DeviceLogin issues a session for a device id, provisioning the identity the
// first time that id is seen. Mounted on the public route group; the switch
// below is the only thing standing between it and an unauthenticated caller,
// which is the point of the feature and the reason it defaults to off.
func (h *Handler) DeviceLogin(w http.ResponseWriter, r *http.Request) {
	if !h.cfg.DeviceAuthEnabled {
		writeError(w, http.StatusForbidden, "device auth is not enabled on this instance")
		return
	}

	var req DeviceLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	deviceID := strings.ToLower(strings.TrimSpace(req.DeviceID))
	if !deviceIDPattern.MatchString(deviceID) {
		writeError(w, http.StatusBadRequest, deviceIDFormatError)
		return
	}

	result, err := h.provisionDeviceIdentity(r.Context(), deviceID, req.DeviceName)
	if errors.Is(err, errDeviceProvisionRace) {
		// One more pass finds the row the winner wrote. A second race would
		// mean something other than concurrent first boots is inserting these.
		result, err = h.provisionDeviceIdentity(r.Context(), deviceID, req.DeviceName)
	}
	if err != nil {
		slog.Error("device auth provisioning failed", append(logger.RequestAttrs(r), "error", err)...)
		writeError(w, http.StatusInternalServerError, "failed to provision device identity")
		return
	}

	token, err := h.issueJWT(result.user)
	if err != nil {
		if errors.Is(err, auth.ErrTemporarilyDisabledUser) {
			writeError(w, http.StatusForbidden, auth.TemporarilyDisabledUserError)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to issue token")
		return
	}

	userID := uuidToString(result.user.ID)
	if result.createdUser {
		obsmetrics.RecordEvent(h.Analytics, h.Metrics, analytics.Signup(userID, result.user.Email, deviceAuthSignupSource))
	}
	if result.createdWorkspace {
		obsmetrics.RecordEvent(h.Analytics, h.Metrics, analytics.WorkspaceCreated(userID, result.workspaceID))
	}
	if result.createdMember {
		h.notifyDaemonWorkspacesChanged(userID)
	}

	slog.Info("device auth session issued", append(logger.RequestAttrs(r),
		"user_id", userID,
		"workspace_id", result.workspaceID,
		"new_user", result.createdUser,
		"new_workspace", result.createdWorkspace,
	)...)

	// Same session artifacts the other two login endpoints hand back. A login
	// endpoint that returns a token but none of the cookies would work for the
	// desktop app (bearer token) and silently half-work for a browser on the
	// same self-hosted deployment: authenticated requests via the HttpOnly
	// cookie, and 403 on every CDN-served attachment.
	if err := auth.SetAuthCookies(w, token); err != nil {
		slog.Warn("failed to set auth cookies", "error", err)
	}
	if h.CFSigner != nil {
		for _, cookie := range h.CFSigner.SignedCookies(time.Now().Add(auth.AuthTokenTTL())) {
			http.SetCookie(w, cookie)
		}
	}

	writeJSON(w, http.StatusOK, LoginResponse{
		Token: token,
		User:  h.userToResponse(result.user),
	})
}

// provisionDeviceIdentity resolves the user, the shared workspace and the
// membership for one device id, creating whichever of the three is missing.
// All of it commits together: a user without membership, or a workspace
// without its status catalog, is a state the app cannot render.
func (h *Handler) provisionDeviceIdentity(ctx context.Context, deviceID, deviceName string) (deviceProvisionResult, error) {
	var out deviceProvisionResult

	tx, err := h.TxStarter.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	qtx := h.Queries.WithTx(tx)

	email := deviceUserEmail(deviceID)
	user, err := qtx.GetUserByEmail(ctx, email)
	switch {
	case err == nil:
	case isNotFound(err):
		user, err = qtx.CreateUser(ctx, db.CreateUserParams{
			Name:  deviceUserName(deviceName, deviceID),
			Email: email,
		})
		if err != nil {
			return out, raceOrErr(err)
		}
		out.createdUser = true
	default:
		return out, err
	}

	slug := h.cfg.deviceAuthWorkspaceSlug()
	ws, err := qtx.GetWorkspaceBySlug(ctx, slug)
	switch {
	case err == nil:
	case isNotFound(err):
		ws, err = qtx.CreateWorkspace(ctx, db.CreateWorkspaceParams{
			Name:        h.cfg.deviceAuthWorkspaceName(),
			Slug:        slug,
			IssuePrefix: defaultIssuePrefixFromSlug(slug),
		})
		if err != nil {
			return out, raceOrErr(err)
		}
		out.createdWorkspace = true
	default:
		return out, err
	}

	// Seeded even for a workspace that already exists: the seed is idempotent
	// and concurrency-safe, and a workspace must never be reachable without
	// its status catalog (MUL-6243) — including one an operator created by
	// hand under the configured slug before turning this switch on.
	if err := issuestatus.Ensure(ctx, qtx, ws.ID); err != nil {
		return out, err
	}

	_, err = qtx.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{
		UserID:      user.ID,
		WorkspaceID: ws.ID,
	})
	switch {
	case err == nil:
	case isNotFound(err):
		role := h.cfg.deviceAuthRole()
		if out.createdWorkspace {
			role = "owner"
		}
		if _, err := qtx.CreateMember(ctx, db.CreateMemberParams{
			WorkspaceID: ws.ID,
			UserID:      user.ID,
			Role:        role,
		}); err != nil {
			return out, raceOrErr(err)
		}
		out.createdMember = true
	default:
		return out, err
	}

	// onboarded_at != null is the only path into the dashboard (the desktop
	// shell routes an un-onboarded user to the onboarding overlay), and a
	// device identity has nothing left to onboard — its workspace and
	// membership exist by the time this runs. The query COALESCEs, so a
	// returning device that somehow reaches this branch is unaffected.
	if !user.OnboardedAt.Valid {
		user, err = qtx.MarkUserOnboarded(ctx, user.ID)
		if err != nil {
			return out, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return out, err
	}

	out.user = user
	out.workspaceID = uuidToString(ws.ID)
	return out, nil
}

// raceOrErr classifies an insert failure. Each insert here runs only after a
// read said the row was absent, so a unique violation means a concurrent
// first boot rather than bad input.
func raceOrErr(err error) error {
	if isUniqueViolation(err) {
		return errDeviceProvisionRace
	}
	return err
}
