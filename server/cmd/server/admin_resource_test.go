package main

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

func TestAdminResourceRoutesEnforceRolesAndStoreCapability(t *testing.T) {
	root := t.TempDir()
	t.Setenv("MULTICA_RESOURCE_PUBLISH_DIR", root)
	f := newPlatformAdminRouterFixture(t)
	super := platformRouterJWT(t, f.user(t, "super_admin"), nil)
	observer := platformRouterJWT(t, f.user(t, "platform_observer"), nil)
	ordinary := platformRouterJWT(t, f.user(t, ""), nil)
	for _, token := range []string{super, observer} {
		platformAssertNoStore(t, f.request(t, token, false, http.MethodGet, "/api/admin/resources?kind=skill", nil, nil).Want(200))
	}
	platformAssertNoStore(t, f.request(t, ordinary, false, http.MethodGet, "/api/admin/resources?kind=skill", nil, nil).Want(403))
	platformAssertNoStore(t, f.request(t, "", false, http.MethodGet, "/api/admin/resources?kind=skill", nil, nil).Want(401))
	for _, path := range []string{"/api/admin/resources/skill/preview", "/api/admin/resources/skill/example/publish", "/api/admin/resources/skill/example/withdraw"} {
		platformAssertNoStore(t, f.request(t, observer, false, http.MethodPost, path, map[string]string{}, nil).Want(403))
		// Missing CSRF on a cookie request is refused before reading the upload.
		platformAssertNoStore(t, f.request(t, super, true, http.MethodPost, path, map[string]string{}, map[string]string{"X-CSRF-Token": ""}).Want(403))
	}
	platformAssertNoStore(t, f.request(t, observer, false, http.MethodGet, "/api/admin/resources/operations/"+uuid.NewString(), nil, nil).Want(404))
	if err := os.WriteFile(filepath.Join(root, "index.json"), []byte(`{"broken":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	response := f.request(t, super, false, http.MethodGet, "/api/admin/resources?kind=skill", nil, nil).Want(503)
	if response.Map()["code"] != "resource_store_unavailable" {
		t.Fatal("a corrupt enabled store was silently disabled")
	}
	platformAssertNoStore(t, response)
}
