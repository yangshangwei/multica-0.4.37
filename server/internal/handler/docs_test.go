package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
)

// The docs endpoints read nothing but the embedded bundle and cfg.ServerVersion,
// so they are exercised against a bare Handler rather than the package fixture.
// (The package TestMain still gates the whole file on a reachable DATABASE_URL;
// that is a property of internal/handler, not of these endpoints.)
func docsTestHandler(version string) *Handler {
	return &Handler{cfg: Config{ServerVersion: version}}
}

// A slug and an asset path that the committed bundle really publishes. Picked
// from manifest.json; if the generator ever stops emitting them the tests below
// fail loudly rather than passing against nothing.
const (
	docsTestSlug  = "index"
	docsTestAsset = "images/docs/agent-access-settings.webp"
)

func TestGetDocsManifestReportsServerVersion(t *testing.T) {
	h := docsTestHandler("1.2.3")
	res := testutil.Call(t, h.GetDocsManifest, testutil.JSONRequest(http.MethodGet, "/api/docs/manifest", nil)).
		Want(http.StatusOK)

	var manifest struct {
		Title  string `json:"title"`
		Groups []struct {
			Label string `json:"label"`
			Items []struct {
				Slug  string `json:"slug"`
				Title string `json:"title"`
			} `json:"items"`
		} `json:"groups"`
		Assets        []string `json:"assets"`
		ServerVersion string   `json:"serverVersion"`
	}
	res.JSON(&manifest)

	if manifest.ServerVersion != "1.2.3" {
		t.Errorf("serverVersion = %q, want 1.2.3", manifest.ServerVersion)
	}
	if manifest.Title == "" || len(manifest.Groups) == 0 {
		t.Fatalf("manifest is empty: %+v", manifest)
	}
	if got := res.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	if res.Header().Get("ETag") == "" {
		t.Error("manifest response has no ETag")
	}

	// Every group carries a label and at least one item. An empty group is the
	// shape a nav-builder bug produces (a separator that opened a section
	// nothing landed in), and it renders as a heading with nothing under it.
	for _, group := range manifest.Groups {
		if group.Label == "" {
			t.Errorf("group has no label: %+v", group)
		}
		if len(group.Items) == 0 {
			t.Errorf("group %q has no items", group.Label)
		}
	}
}

// The manifest is the allowlist for the other two endpoints, so anything it
// names has to actually resolve. This is the coverage-completeness check: a slug
// in the nav with no page behind it is a dead sidebar row.
func TestGetDocsManifestEntriesAllResolve(t *testing.T) {
	h := docsTestHandler("")
	var manifest struct {
		Groups []struct {
			Items []struct {
				Slug string `json:"slug"`
			} `json:"items"`
		} `json:"groups"`
		Assets []string `json:"assets"`
	}
	testutil.Call(t, h.GetDocsManifest, testutil.JSONRequest(http.MethodGet, "/api/docs/manifest", nil)).
		Want(http.StatusOK).JSON(&manifest)

	pages := 0
	for _, group := range manifest.Groups {
		for _, item := range group.Items {
			pages++
			req := testutil.JSONRequest(http.MethodGet, "/api/docs/page?slug="+item.Slug, nil)
			testutil.Call(t, h.GetDocsPage, req).Want(http.StatusOK)
		}
	}
	if pages == 0 {
		t.Fatal("manifest published no pages")
	}

	for _, asset := range manifest.Assets {
		req := testutil.WithURLParams(
			testutil.JSONRequest(http.MethodGet, "/api/docs/assets/"+asset, nil), "*", asset)
		testutil.Call(t, h.ServeDocsAsset, req).Want(http.StatusOK)
	}
	if len(manifest.Assets) == 0 {
		t.Fatal("manifest published no assets")
	}
}

func TestGetDocsPageReturnsRequestedSlug(t *testing.T) {
	h := docsTestHandler("")
	req := testutil.JSONRequest(http.MethodGet, "/api/docs/page?slug="+docsTestSlug, nil)
	res := testutil.Call(t, h.GetDocsPage, req).Want(http.StatusOK)

	var page struct {
		Slug  string `json:"slug"`
		Title string `json:"title"`
		Body  string `json:"body"`
		TOC   []struct {
			Depth int    `json:"depth"`
			Title string `json:"title"`
			ID    string `json:"id"`
		} `json:"toc"`
	}
	res.JSON(&page)

	if page.Slug != docsTestSlug {
		t.Errorf("slug = %q, want %q", page.Slug, docsTestSlug)
	}
	if page.Title == "" || page.Body == "" {
		t.Errorf("page has no title or body: %+v", page)
	}
	if got := res.Header().Get("Content-Length"); got != strconv.Itoa(res.Body.Len()) {
		t.Errorf("Content-Length = %q, want %d", got, res.Body.Len())
	}
}

func TestGetDocsPageRejectsMissingSlug(t *testing.T) {
	h := docsTestHandler("")
	for _, query := range []string{"", "?slug=", "?slug=%20%20"} {
		req := testutil.JSONRequest(http.MethodGet, "/api/docs/page"+query, nil)
		testutil.Call(t, h.GetDocsPage, req).Want(http.StatusBadRequest)
	}
}

// The slug is looked up in the manifest, never joined onto a path. These inputs
// therefore have to 404 rather than reaching another embedded file — the
// embedded FS stops an escape out of itself, not an over-read within it.
func TestGetDocsPageRefusesUnpublishedSlugs(t *testing.T) {
	h := docsTestHandler("")
	for _, slug := range []string{
		"nope",
		"../manifest",
		"../../go.mod",
		"index/../../manifest",
		"manifest",    // real embedded file, not a published page
		"pages/index", // the on-disk layout, not the slug
		"index.json",  // ditto
		"/etc/passwd",
		`..\manifest`,
		"developers", // a nav directory, which has no page of its own
	} {
		req := testutil.JSONRequest(http.MethodGet, "/api/docs/page?slug="+slug, nil)
		testutil.Call(t, h.GetDocsPage, req).Want(http.StatusNotFound)
	}
}

func TestServeDocsAssetServesPublishedImage(t *testing.T) {
	h := docsTestHandler("")
	req := testutil.WithURLParams(
		testutil.JSONRequest(http.MethodGet, "/api/docs/assets/"+docsTestAsset, nil), "*", docsTestAsset)
	res := testutil.Call(t, h.ServeDocsAsset, req).Want(http.StatusOK)

	if got := res.Header().Get("Content-Type"); got != "image/webp" {
		t.Errorf("Content-Type = %q, want image/webp", got)
	}
	if got := res.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if res.Body.Len() == 0 {
		t.Error("asset body is empty")
	}
}

func TestServeDocsAssetRefusesUnpublishedPaths(t *testing.T) {
	h := docsTestHandler("")
	for _, assetPath := range []string{
		"images/docs/missing.webp",
		"../manifest.json",
		"../pages/index.json",
		"images/docs/../../manifest.json",
		"manifest.json",                         // disallowed extension and not an asset
		"images/docs/notes.md",                  // ditto
		"images/docs/agent-access-settings.svg", // allowlist excludes SVG
		"",
	} {
		req := testutil.WithURLParams(
			testutil.JSONRequest(http.MethodGet, "/api/docs/assets/"+assetPath, nil), "*", assetPath)
		testutil.Call(t, h.ServeDocsAsset, req).Want(http.StatusNotFound)
	}
}

// A percent-encoded traversal must not decode into a readable path either.
func TestServeDocsAssetRefusesEncodedTraversal(t *testing.T) {
	h := docsTestHandler("")
	encoded := "..%2Fpages%2Findex.json"
	req := testutil.WithURLParams(
		testutil.JSONRequest(http.MethodGet, "/api/docs/assets/"+encoded, nil), "*", encoded)
	testutil.Call(t, h.ServeDocsAsset, req).Want(http.StatusNotFound)
}

func TestDocsEndpointsHonorIfNoneMatch(t *testing.T) {
	h := docsTestHandler("1.2.3")

	manifestETag := testutil.Call(t, h.GetDocsManifest,
		testutil.JSONRequest(http.MethodGet, "/api/docs/manifest", nil)).
		Want(http.StatusOK).Header().Get("ETag")
	if manifestETag == "" {
		t.Fatal("manifest served no ETag to revalidate against")
	}

	pagePath := "/api/docs/page?slug=" + docsTestSlug
	pageETag := testutil.Call(t, h.GetDocsPage, testutil.JSONRequest(http.MethodGet, pagePath, nil)).
		Want(http.StatusOK).Header().Get("ETag")

	assetReq := func() *http.Request {
		return testutil.WithURLParams(
			testutil.JSONRequest(http.MethodGet, "/api/docs/assets/"+docsTestAsset, nil), "*", docsTestAsset)
	}
	assetETag := testutil.Call(t, h.ServeDocsAsset, assetReq()).Want(http.StatusOK).Header().Get("ETag")

	// A GET revalidation compares weakly (RFC 9110 §13.1.2), and clients may
	// send several tags at once, so all four of these forms have to hit.
	for _, header := range []string{
		manifestETag,
		"W/" + manifestETag,
		`"stale", ` + manifestETag,
		"*",
	} {
		req := testutil.WithHeaders(
			testutil.JSONRequest(http.MethodGet, "/api/docs/manifest", nil), "If-None-Match", header)
		res := testutil.Call(t, h.GetDocsManifest, req).Want(http.StatusNotModified)
		if res.Body.Len() != 0 {
			t.Errorf("304 for If-None-Match %q carried a body", header)
		}
	}

	pageReq := testutil.WithHeaders(
		testutil.JSONRequest(http.MethodGet, pagePath, nil), "If-None-Match", pageETag)
	testutil.Call(t, h.GetDocsPage, pageReq).Want(http.StatusNotModified)

	testutil.Call(t, h.ServeDocsAsset,
		testutil.WithHeaders(assetReq(), "If-None-Match", assetETag)).Want(http.StatusNotModified)

	// A stale tag must still serve the body.
	stale := testutil.WithHeaders(
		testutil.JSONRequest(http.MethodGet, "/api/docs/manifest", nil), "If-None-Match", `"stale"`)
	testutil.Call(t, h.GetDocsManifest, stale).Want(http.StatusOK)
}

// The manifest ETag has to move when the reported version does, or a client that
// upgraded its server keeps revalidating into a 304 and shows the old version.
func TestDocsManifestETagTracksServerVersion(t *testing.T) {
	first := testutil.Call(t, docsTestHandler("1.2.3").GetDocsManifest,
		testutil.JSONRequest(http.MethodGet, "/api/docs/manifest", nil)).
		Want(http.StatusOK).Header().Get("ETag")
	second := testutil.Call(t, docsTestHandler("9.9.9").GetDocsManifest,
		testutil.JSONRequest(http.MethodGet, "/api/docs/manifest", nil)).
		Want(http.StatusOK).Header().Get("ETag")

	if first == second {
		t.Errorf("manifest ETag %q did not change with the server version", first)
	}
}

// Cache policy differs by resource on purpose: JSON must revalidate so an
// upgrade publishes immediately, images are immutable for a build.
func TestDocsCacheControlByResource(t *testing.T) {
	h := docsTestHandler("")

	manifest := testutil.Call(t, h.GetDocsManifest,
		testutil.JSONRequest(http.MethodGet, "/api/docs/manifest", nil)).Want(http.StatusOK)
	if got := manifest.Header().Get("Cache-Control"); got != docsRevalidate {
		t.Errorf("manifest Cache-Control = %q, want %q", got, docsRevalidate)
	}

	asset := testutil.WithURLParams(
		testutil.JSONRequest(http.MethodGet, "/api/docs/assets/"+docsTestAsset, nil), "*", docsTestAsset)
	res := testutil.Call(t, h.ServeDocsAsset, asset).Want(http.StatusOK)
	if got := res.Header().Get("Cache-Control"); got != docsAssetMaxAge {
		t.Errorf("asset Cache-Control = %q, want %q", got, docsAssetMaxAge)
	}
}

// Product code deep-links into headings (#自定义运行时配置 from runtime-docs.ts).
// Those anchors only work if the ids the generator wrote are in the page's toc,
// so the shape has to survive the round trip through the endpoint.
func TestGetDocsPageTOCCarriesHeadingIDs(t *testing.T) {
	h := docsTestHandler("")
	req := testutil.JSONRequest(http.MethodGet, "/api/docs/page?slug=daemon-runtimes", nil)
	body := testutil.Call(t, h.GetDocsPage, req).Want(http.StatusOK).Text()

	var page struct {
		TOC []struct {
			Depth int    `json:"depth"`
			Title string `json:"title"`
			ID    string `json:"id"`
		} `json:"toc"`
	}
	if err := json.Unmarshal([]byte(body), &page); err != nil {
		t.Fatalf("decode page: %v", err)
	}
	if len(page.TOC) == 0 {
		t.Fatal("daemon-runtimes published an empty toc")
	}
	for _, entry := range page.TOC {
		if entry.ID == "" || entry.Title == "" {
			t.Errorf("toc entry missing id or title: %+v", entry)
		}
		if entry.Depth < 2 || entry.Depth > 3 {
			t.Errorf("toc entry depth %d outside h2/h3", entry.Depth)
		}
	}
}
