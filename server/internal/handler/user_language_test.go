package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
)

func newLanguageTestUser(t *testing.T, email string) string {
	t.Helper()
	ctx := context.Background()

	var userID string
	if err := testPool.QueryRow(ctx,
		`INSERT INTO "user" (name, email) VALUES ($1, $2) RETURNING id`,
		"Language Test", email,
	).Scan(&userID); err != nil {
		t.Fatalf("insert test user: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM "user" WHERE id = $1`, userID)
	})
	return userID
}

func newPatchMeRequest(userID, body string) *http.Request {
	req := httptest.NewRequest("PATCH", "/api/me", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", userID)
	return req
}

func TestUpdateMeAcceptsLanguage(t *testing.T) {
	userID := newLanguageTestUser(t, "lang-set@multica.ai")

	w := httptest.NewRecorder()
	req := newPatchMeRequest(userID, `{"language":"zh-Hans"}`)
	testHandler.UpdateMe(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var lang *string
	if err := testPool.QueryRow(context.Background(),
		`SELECT language FROM "user" WHERE id = $1`, userID,
	).Scan(&lang); err != nil {
		t.Fatalf("lookup user: %v", err)
	}
	if lang == nil || *lang != "zh-Hans" {
		t.Fatalf("expected language=zh-Hans, got %v", lang)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got, _ := resp["language"].(string); got != "zh-Hans" {
		t.Fatalf("expected response language=zh-Hans, got %v", resp["language"])
	}
}

func TestUpdateMeAcceptsKoreanLanguage(t *testing.T) {
	userID := newLanguageTestUser(t, "lang-ko@multica.ai")

	w := httptest.NewRecorder()
	req := newPatchMeRequest(userID, `{"language":"ko"}`)
	testHandler.UpdateMe(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got, _ := resp["language"].(string); got != "en" {
		t.Fatalf("expected response language=en, got %v", resp["language"])
	}
}

func TestUpdateMeAcceptsJapaneseLanguage(t *testing.T) {
	userID := newLanguageTestUser(t, "lang-ja@multica.ai")

	w := httptest.NewRecorder()
	req := newPatchMeRequest(userID, `{"language":"ja"}`)
	testHandler.UpdateMe(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got, _ := resp["language"].(string); got != "en" {
		t.Fatalf("expected response language=en, got %v", resp["language"])
	}
}

func TestUserLanguageLegacyReadAndWriteCompatibility(t *testing.T) {
	for _, language := range []string{"ja", "ko"} {
		t.Run(language, func(t *testing.T) {
			userID := dbfx.User(t, "Language Test", "language-legacy@multica.ai", testutil.Cols{"language": language})
			var before string
			dbfx.QueryRow(t, "SELECT updated_at::text FROM \"user\" WHERE id = $1", userID).Scan(&before)
			var out UserResponse
			req := testutil.WithHeaders(httptest.NewRequest("GET", "/api/me", nil), "X-User-ID", userID)
			testutil.Call(t, testHandler.GetMe, req).Want(http.StatusOK).JSON(&out)
			if out.Language == nil || *out.Language != "en" {
				t.Errorf("legacy GET language = %v, want en", out.Language)
			}
			var persisted, after string
			dbfx.QueryRow(t, "SELECT language, updated_at::text FROM \"user\" WHERE id = $1", userID).Scan(&persisted, &after)
			if persisted != language || after != before {
				t.Fatalf("GET mutated stored preference: %q/%q -> %q/%q", language, before, persisted, after)
			}
			for _, body := range []map[string]any{{"name": "Updated Name"}, {"language": nil}} {
				req := testutil.WithHeaders(testutil.JSONRequest("PATCH", "/api/me", body), "X-User-ID", userID)
				testutil.Call(t, testHandler.UpdateMe, req).Want(http.StatusOK).JSON(&out)
				dbfx.QueryRow(t, "SELECT language FROM \"user\" WHERE id = $1", userID).Scan(&persisted)
				if persisted != language || out.Language == nil || *out.Language != "en" {
					t.Errorf("omitted/null update changed preference or returned retired value: stored=%q response=%v", persisted, out.Language)
				}
			}
			req = testutil.WithHeaders(testutil.JSONRequest("PATCH", "/api/me", map[string]any{"language": " " + language + " "}), "X-User-ID", userID)
			testutil.Call(t, testHandler.UpdateMe, req).Want(http.StatusOK).JSON(&out)
			dbfx.QueryRow(t, "SELECT language FROM \"user\" WHERE id = $1", userID).Scan(&persisted)
			if persisted != "en" || out.Language == nil || *out.Language != "en" {
				t.Fatalf("explicit legacy write must save English: stored=%q response=%v", persisted, out.Language)
			}
		})
	}
}

func TestUserLanguagePreservesUnsetAndRejectsOtherTags(t *testing.T) {
	userID := dbfx.User(t, "No Preference", "language-unset@multica.ai")
	req := testutil.WithHeaders(httptest.NewRequest("GET", "/api/me", nil), "X-User-ID", userID)
	var out UserResponse
	testutil.Call(t, testHandler.GetMe, req).Want(http.StatusOK).JSON(&out)
	if out.Language != nil {
		t.Fatalf("unset preference must remain null, got %q", *out.Language)
	}
	for _, language := range []any{nil, "zh", "ja-JP", "ko-KR", ""} {
		want := http.StatusBadRequest
		if language == nil {
			want = http.StatusOK
		}
		req = testutil.WithHeaders(testutil.JSONRequest("PATCH", "/api/me", map[string]any{"language": language}), "X-User-ID", userID)
		testutil.Call(t, testHandler.UpdateMe, req).Want(want)
		var stored *string
		dbfx.QueryRow(t, "SELECT language FROM \"user\" WHERE id = $1", userID).Scan(&stored)
		if stored != nil {
			t.Fatalf("null/invalid input %v invented preference %q", language, *stored)
		}
	}
}

func TestUpdateMeRejectsUnsupportedLanguage(t *testing.T) {
	userID := newLanguageTestUser(t, "lang-reject@multica.ai")

	w := httptest.NewRecorder()
	req := newPatchMeRequest(userID, `{"language":"<script>"}`)
	testHandler.UpdateMe(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}

	var lang *string
	if err := testPool.QueryRow(context.Background(),
		`SELECT language FROM "user" WHERE id = $1`, userID,
	).Scan(&lang); err != nil {
		t.Fatalf("lookup user: %v", err)
	}
	if lang != nil {
		t.Fatalf("expected language unchanged (NULL), got %v", *lang)
	}
}

// COALESCE semantics: omitting language must NOT clear an existing value.
func TestUpdateMePreservesLanguageWhenNotProvided(t *testing.T) {
	userID := newLanguageTestUser(t, "lang-preserve@multica.ai")

	if _, err := testPool.Exec(context.Background(),
		`UPDATE "user" SET language = 'en' WHERE id = $1`, userID,
	); err != nil {
		t.Fatalf("preset language: %v", err)
	}

	w := httptest.NewRecorder()
	req := newPatchMeRequest(userID, `{"name":"Updated Name"}`)
	testHandler.UpdateMe(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var lang *string
	if err := testPool.QueryRow(context.Background(),
		`SELECT language FROM "user" WHERE id = $1`, userID,
	).Scan(&lang); err != nil {
		t.Fatalf("lookup user: %v", err)
	}
	if lang == nil || *lang != "en" {
		t.Fatalf("expected language=en preserved, got %v", lang)
	}
}
