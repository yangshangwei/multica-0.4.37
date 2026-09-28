package handler

import (
	"bytes"
	"image/png"
	"net/http"
	"testing"

	"github.com/multica-ai/multica/server/internal/storage"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestResolveAvatarURL_BuiltinSealDoesNotRequireStorage(t *testing.T) {
	const avatarPath = "/api/avatars/builtin/afu-seal-v1.png"
	for _, store := range []storage.Storage{nil, &mockStorageNoCdn{}} {
		for _, publicURL := range []string{"", "https://api.example.test/"} {
			h := &Handler{Storage: store, cfg: Config{PublicURL: publicURL}}
			want := avatarPath
			if publicURL != "" {
				want = "https://api.example.test" + avatarPath
			}
			if got := h.resolveAvatarURL(avatarPath); got != want {
				t.Errorf("storage=%T publicURL=%q: got %q, want %q", store, publicURL, got, want)
			}
			if got := h.resolveAvatarURL(want); got != want {
				t.Errorf("resolved avatar changed on second read: %q", got)
			}
			for _, raw := range []string{"", "emoji:🦄", "/api/avatars/builtin/unknown.png", "https://example.test/custom.png"} {
				if got := h.resolveAvatarURL(raw); got != raw {
					t.Errorf("unrelated avatar %q changed to %q", raw, got)
				}
			}
		}
	}
}

func TestServeBuiltinAgentAvatar_PublicPNG(t *testing.T) {
	h := &Handler{}
	res := testutil.Call(t, h.ServeBuiltinAgentAvatar,
		testutil.JSONRequest(http.MethodGet, "/api/avatars/builtin/afu-seal-v1.png", nil)).Want(http.StatusOK)
	for header, want := range map[string]string{
		"Content-Type":           "image/png",
		"Cache-Control":          "public, max-age=31536000, immutable",
		"X-Content-Type-Options": "nosniff",
	} {
		if got := res.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
	img, err := png.Decode(bytes.NewReader(res.Body.Bytes()))
	if err != nil {
		t.Fatalf("public avatar is not a decodable PNG: %v", err)
	}
	if img.Bounds().Dx() != 512 || img.Bounds().Dy() != 512 {
		t.Fatalf("avatar dimensions = %v, want 512 x 512", img.Bounds())
	}
}
