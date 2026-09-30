package handler

import (
	"crypto/rand"
	"math/big"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
)

const avatarIconPrefix = "icon:"

var agentIconAvatars = []string{
	"bot", "brain", "code", "compass", "flask-conical", "globe", "lightbulb", "rocket",
	"search", "sparkles", "terminal", "wrench",
}

func randomAgentIconAvatar() string {
	index, err := rand.Int(rand.Reader, big.NewInt(int64(len(agentIconAvatars))))
	if err != nil {
		return avatarIconPrefix + agentIconAvatars[0]
	}
	return avatarIconPrefix + agentIconAvatars[index.Int64()]
}

// templateIconAvatar preserves the legacy catalog emoji field while storing
// an explicit Lucide icon for newly materialized agents and squads.
func templateIconAvatar(emoji string) string {
	icons := map[string]string{
		"🚀": "rocket", "🐞": "bug", "🐛": "bug", "🚑": "bug",
		"🚧": "shield-check", "🛡️": "shield-check", "🔭": "telescope",
		"📚": "book-open", "🧹": "wrench", "📦": "package", "🚨": "siren",
		"🔍": "search", "🔎": "search", "📐": "landmark", "🛠️": "code",
		"🧪": "test-tube", "🩺": "microscope", "🔬": "microscope",
		"🚦": "clipboard-check", "📝": "pen-line", "📊": "bar-chart",
		"📡": "server", "🤖": "bot", "🖥️": "globe", "🗃️": "database",
		"🧭": "compass", "🔀": "git-pull-request", "📄": "file-text",
	}
	if icon, ok := icons[emoji]; ok {
		return avatarIconPrefix + icon
	}
	return avatarIconPrefix + "bot"
}

// newAgentAvatar resolves the avatar to persist for a newly created agent. An
// explicit value is validated through acceptAvatarURL — a create path is still
// a way to publish a storage object, so it carries the same authorization
// boundary as an update. ok=false means the error response is already written
// and the caller must abort.
func (h *Handler) newAgentAvatar(w http.ResponseWriter, r *http.Request, avatarURL *string) (pgtype.Text, bool) {
	if avatarURL != nil && strings.TrimSpace(*avatarURL) != "" {
		accepted, ok := h.acceptAvatarURL(w, r, *avatarURL, "")
		if !ok {
			return pgtype.Text{}, false
		}
		return pgtype.Text{String: accepted, Valid: true}, true
	}
	return pgtype.Text{String: randomAgentIconAvatar(), Valid: true}, true
}
