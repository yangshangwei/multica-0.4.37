package storage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLocalPublicRouteRefusesCollaborationPrivateNamespace(t *testing.T) {
	s := &LocalStorage{uploadDir: t.TempDir()}
	key := "private/collaboration/object.bin"
	if _, err := s.Upload(context.Background(), key, []byte("private ciphertext"), "application/octet-stream", ""); err != nil {
		t.Fatal(err)
	}
	for _, routeKey := range []string{key, "./" + key, "ordinary/../" + key, "private/other/../collaboration/object.bin"} {
		w := httptest.NewRecorder()
		s.ServeFile(w, httptest.NewRequest(http.MethodGet, "/uploads/"+routeKey, nil), routeKey)
		if w.Code != http.StatusNotFound {
			t.Fatalf("public serving reached collaboration storage: path %q status %d", routeKey, w.Code)
		}
	}
	r, err := s.GetReader(context.Background(), key)
	if err != nil {
		t.Fatal("authorized private object reader must remain available:", err)
	}
	if err = r.Close(); err != nil {
		t.Fatal(err)
	}
}
