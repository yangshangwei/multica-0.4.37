package handler

import (
	"bytes"
	_ "embed"
	"net/http"
	"time"
)

//go:embed builtin_agent_avatars/afu-seal-v1.png
var afuSealAvatarPNG []byte

// ServeBuiltinAgentAvatar publishes the versioned artwork embedded in the
// binary. It contains no tenant data and is available without storage or auth.
func (h *Handler) ServeBuiltinAgentAvatar(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, "afu-seal-v1.png", time.Time{}, bytes.NewReader(afuSealAvatarPNG))
}
