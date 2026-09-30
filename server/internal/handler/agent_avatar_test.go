package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIconAvatarValidation(t *testing.T) {
	h := &Handler{}
	for _, tc := range []struct {
		value string
		valid bool
	}{
		{"icon:bot", true}, {"icon:users", true}, {"icon:siren", true}, {"icon:telescope", true},
		{" icon:rocket ", true}, {"emoji:🐙", true},
		{"icon:", false}, {"icon:unknown", false}, {"icon:../bot", false}, {"icon:<svg>", false}, {"icon:Bot", false},
	} {
		t.Run(tc.value, func(t *testing.T) {
			w := httptest.NewRecorder()
			got, ok := h.acceptAvatarURL(w, httptest.NewRequest(http.MethodPost, "/", nil), tc.value, tc.value)
			if ok != tc.valid {
				t.Fatalf("valid = %v, want %v", ok, tc.valid)
			}
			if ok && got != strings.TrimSpace(tc.value) {
				t.Errorf("value = %q", got)
			}
			if !ok && w.Code != http.StatusBadRequest {
				t.Errorf("status = %d", w.Code)
			}
		})
	}
}
