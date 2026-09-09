package handler

import (
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/docs"
)

// docsRevalidate is the cache policy for the two JSON endpoints: a client may
// hold a copy but must revalidate, so upgrading the server publishes new
// content on the next request instead of after a TTL.
const docsRevalidate = "private, no-cache"

// docsAssetMaxAge is how long a client may reuse a docs image. The bytes are
// immutable for a given server build, and an upgrade that changes an image
// changes the manifest that names it, so a long TTL costs nothing.
const docsAssetMaxAge = "public, max-age=604800, immutable"

// docsAssetContentTypes is an allowlist, not a lookup table. Serving an
// embedded file under a type the generator never meant to produce is how a docs
// bundle turns into an injection surface, so an extension that is not named
// here is refused rather than sniffed.
//
// SVG is deliberately absent. It is the one image format that executes script
// when a browser navigates to it directly, and these bytes are served from the
// API origin; the generator emits .webp only, so allowing it would widen the
// surface for content that does not exist. Add it only with a sandboxing CSP.
var docsAssetContentTypes = map[string]string{
	".webp": "image/webp",
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
}

// GetDocsManifest returns the in-app documentation navigation tree.
//
// The manifest is the allowlist the other two endpoints validate against: a
// slug or asset path absent from it does not resolve, so a client can only ask
// for content this build actually published.
func (h *Handler) GetDocsManifest(w http.ResponseWriter, r *http.Request) {
	resource := docs.Manifest(h.cfg.ServerVersion)
	if docsNotModified(w, r, resource.ETag(), docsRevalidate) {
		return
	}
	writeDocsResource(w, resource.Content(), "application/json")
}

// GetDocsPage returns one documentation page by slug.
func (h *Handler) GetDocsPage(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimSpace(r.URL.Query().Get("slug"))
	if slug == "" {
		writeError(w, http.StatusBadRequest, "slug is required")
		return
	}
	// A manifest-membership lookup, not a path join: the slug never becomes a
	// filesystem path here, so there is no traversal to escape and no way to
	// read an embedded file this build did not publish as a page.
	resource, ok := docs.Page(slug)
	if !ok {
		writeError(w, http.StatusNotFound, "documentation page not found")
		return
	}
	if docsNotModified(w, r, resource.ETag(), docsRevalidate) {
		return
	}
	writeDocsResource(w, resource.Content(), "application/json")
}

// ServeDocsAsset serves an image referenced by an embedded documentation page.
//
// Deliberately outside the authenticated route group. The desktop renderer runs
// on an opaque file:// origin (see renderer-web-preferences.ts), so an <img> tag
// it renders sends neither the SameSite=Strict auth cookie nor an Authorization
// header — the same constraint that already puts ServeAvatar and
// DownloadAttachmentWithCapability on public routes. Those two carry a
// credential in the URL because they serve tenant content; these bytes are the
// build's own published screenshots, identical for every caller and already
// public on the docs site, so there is no secret for a signature to protect.
//
// What keeps this narrow is the manifest: only paths this build published
// resolve, and only the extensions allowlisted above are served.
func (h *Handler) ServeDocsAsset(w http.ResponseWriter, r *http.Request) {
	assetPath, err := url.PathUnescape(chi.URLParam(r, "*"))
	if err != nil {
		writeError(w, http.StatusNotFound, "documentation asset not found")
		return
	}
	contentType, ok := docsAssetContentTypes[strings.ToLower(path.Ext(assetPath))]
	if !ok {
		writeError(w, http.StatusNotFound, "documentation asset not found")
		return
	}
	resource, ok := docs.Asset(assetPath)
	if !ok {
		writeError(w, http.StatusNotFound, "documentation asset not found")
		return
	}
	if docsNotModified(w, r, resource.ETag(), docsAssetMaxAge) {
		return
	}
	writeDocsResource(w, resource.Content(), contentType)
}

// docsNotModified sets the validator headers and reports whether the client's
// cached copy is already current, in which case the body is skipped.
func docsNotModified(w http.ResponseWriter, r *http.Request, etag, cacheControl string) bool {
	w.Header().Set("Cache-Control", cacheControl)
	w.Header().Set("ETag", etag)
	if etagMatches(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return true
	}
	return false
}

// etagMatches implements the If-None-Match comparison for the subset of the
// header these endpoints can receive: "*", or a comma-separated list of entity
// tags. Comparison is weak, per RFC 9110 §13.1.2 — a GET revalidation must
// treat W/"x" and "x" as a match, and a plain compare against the whole header
// value misses every client that sends more than one tag.
func etagMatches(header, etag string) bool {
	header = strings.TrimSpace(header)
	if header == "" {
		return false
	}
	if header == "*" {
		return true
	}
	for _, candidate := range strings.Split(header, ",") {
		if strings.TrimPrefix(strings.TrimSpace(candidate), "W/") == etag {
			return true
		}
	}
	return false
}

func writeDocsResource(w http.ResponseWriter, body []byte, contentType string) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}
