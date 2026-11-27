package materialgit

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	baseSHA  = "1111111111111111111111111111111111111111"
	headSHA  = "2222222222222222222222222222222222222222"
	mergeSHA = "3333333333333333333333333333333333333333"
	treeSHA  = "4444444444444444444444444444444444444444"
	blobSHA  = "5555555555555555555555555555555555555555"
	filePath = "requirements/req-1/prd.md"
)

func candidate() PublishRequest {
	return PublishRequest{Key: "rev-1", Branch: "multica/material/rev-1", BaseBranch: "main", Title: "Review the agreed requirement", Body: "Material review; no closing intent.", Files: []File{{Path: filePath, Content: "# Requirement\n\nAC-1: verified behavior\n"}}}
}

func mergeExpectation(kind string) MergeExpectation {
	change := Change{Path: filePath, Status: "added", Mode: "100644"}
	if kind == "gitlab" {
		change.PreviousPath = filePath
	}
	return MergeExpectation{Number: 9, HeadSHA: headSHA, Branch: candidate().Branch, BaseBranch: "main", Changes: []Change{change}}
}

// Each test owns its server. This fixture executes real HTTP adapters and keeps
// provider effects separately from replies so response-loss recovery is tested.
type remote struct {
	t                                  *testing.T
	mu                                 sync.Mutex
	kind                               string
	branch, review, merged             bool
	content, commitMessage, reviewBody string
	mode                               string
	mergedAt                           *time.Time
	branchCollision                    bool
	existingBase                       bool
	foreignHead                        bool
	extraFile                          bool
	truncateDiff                       bool
	protected                          bool
	enforceAdmins                      bool
	requireReviews                     bool
	requireChecks                      bool
	approval                           string
	checkState                         string
	policyStatus                       int
	classicStatus                      int
	classicProof                       string
	rules                              []map[string]any
	rulesetBypass                      []any
	glStatus                           string
	diffPending                        bool
	treeUnavailable                    bool
	pipelineSHA                        string
	reviewBase                         string
	canMerge                           bool
	loss                               string
	afterMerge                         func(*remote)
	notes                              []string
	writes                             map[string]int
	requests                           []string
	server                             *httptest.Server
}

func fixture(t *testing.T, kind string) (Provider, *remote) {
	t.Helper()
	mergedAt := time.Date(2026, 11, 27, 8, 0, 0, 0, time.UTC)
	f := &remote{mergedAt: &mergedAt, t: t, kind: kind, mode: "100644", enforceAdmins: true, approval: "APPROVED", checkState: "SUCCESS", glStatus: "mergeable", canMerge: true, writes: map[string]int{}}
	f.server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.server.Close)
	p, err := newClient(Connection{Provider: kind, BaseURL: f.server.URL, Repository: "acme/materials", Token: "private-test-token"}, f.server.Client(), true)
	if err != nil {
		t.Fatal(err)
	}
	return p, f
}
func (f *remote) reply(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		f.t.Error(err)
	}
}
func (f *remote) read(r *http.Request) map[string]any {
	var v map[string]any
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		f.t.Error(err)
	}
	return v
}
func (f *remote) lost(w http.ResponseWriter, phase string) bool {
	if f.loss != phase {
		return false
	}
	f.loss = ""
	f.reply(w, 500, map[string]string{"error": "private-test-token must never leak"})
	return true
}
func (f *remote) head() string {
	if f.foreignHead {
		return baseSHA
	}
	return headSHA
}
func (f *remote) ghPull() map[string]any {
	state := "open"
	if f.merged {
		state = "closed"
	}
	count := 1
	if f.extraFile {
		count++
	}
	return map[string]any{"number": int64(9), "html_url": "https://github.com/acme/materials/pull/9", "body": f.reviewBody, "state": state, "draft": false, "merged": f.merged, "mergeable": true, "mergeable_state": "clean", "changed_files": count, "merge_commit_sha": mergeSHA, "merged_at": f.mergedAt, "merged_by": map[string]string{"login": "multica[bot]"}, "head": map[string]any{"ref": candidate().Branch, "sha": f.head(), "repo": map[string]string{"full_name": "acme/materials"}}, "base": map[string]any{"ref": "main", "sha": baseSHA, "repo": map[string]string{"full_name": "acme/materials"}}}
}
func (f *remote) glMR() map[string]any {
	state := "opened"
	if f.merged {
		state = "merged"
	}
	count := "1"
	if f.extraFile {
		count = "2"
	}
	return map[string]any{"iid": 9, "project_id": 1, "source_project_id": 1, "target_project_id": 1, "web_url": strings.Replace(f.server.URL, "http://", "https://", 1) + "/acme/materials/-/merge_requests/9", "description": f.reviewBody, "state": state, "sha": f.head(), "source_branch": candidate().Branch, "target_branch": "main", "draft": false, "detailed_merge_status": f.glStatus, "has_conflicts": false, "changes_count": count, "merge_commit_sha": mergeSHA, "merged_at": f.mergedAt, "merge_user": map[string]string{"username": "multica"}, "user": map[string]bool{"can_merge": f.canMerge}, "diff_refs": map[string]string{"head_sha": f.head()}, "head_pipeline": map[string]string{"sha": f.head(), "status": strings.ToLower(f.checkState)}}
}
func (f *remote) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, r.Method+" "+r.URL.RequestURI())
	if f.kind == "github" {
		if r.Header.Get("Authorization") != "Bearer private-test-token" {
			f.t.Error("missing GitHub credential")
		}
		f.serveGH(w, r)
	} else {
		if r.Header.Get("PRIVATE-TOKEN") != "private-test-token" {
			f.t.Error("missing GitLab credential")
		}
		f.serveGL(w, r)
	}
}

func (f *remote) serveGH(w http.ResponseWriter, r *http.Request) {
	ep := strings.TrimPrefix(r.URL.Path, "/repos/acme/materials")
	switch {
	case r.Method == "GET" && strings.HasPrefix(ep, "/git/ref/heads/"):
		branch := strings.TrimPrefix(ep, "/git/ref/heads/")
		sha := baseSHA
		if branch != "main" {
			if !f.branch {
				f.reply(w, 404, map[string]string{})
				return
			}
			sha = f.head()
		}
		f.reply(w, 200, map[string]any{"ref": "refs/heads/" + branch, "object": map[string]string{"type": "commit", "sha": sha}})
	case r.Method == "GET" && strings.HasPrefix(ep, "/git/commits/"):
		sha := strings.TrimPrefix(ep, "/git/commits/")
		f.reply(w, 200, map[string]any{"sha": sha, "message": f.commitMessage, "tree": map[string]string{"sha": treeSHA}, "parents": []map[string]string{{"sha": baseSHA}}})
	case r.Method == "GET" && strings.HasPrefix(ep, "/git/trees/"):
		if f.treeUnavailable {
			f.reply(w, 404, map[string]string{})
			return
		}
		entries := []map[string]string{{"path": "requirements", "mode": "040000", "type": "tree", "sha": treeSHA}, {"path": "req-1", "mode": "040000", "type": "tree", "sha": treeSHA}}
		if f.branch || f.existingBase || f.review {
			entries = append(entries, map[string]string{"path": "prd.md", "mode": f.mode, "type": "blob", "sha": blobSHA})
		}
		if f.extraFile {
			entries = append(entries, map[string]string{"path": "secret.md", "mode": "100644", "type": "blob", "sha": blobSHA})
		}
		f.reply(w, 200, map[string]any{"sha": treeSHA, "truncated": false, "tree": entries})
	case r.Method == "GET" && strings.HasPrefix(ep, "/git/blobs/"):
		f.reply(w, 200, map[string]any{"sha": blobSHA, "size": len(f.content), "encoding": "base64", "content": base64.StdEncoding.EncodeToString([]byte(f.content))})
	case r.Method == "POST" && ep == "/git/trees":
		f.writes["tree"]++
		body := f.read(r)
		if body["base_tree"] != treeSHA {
			f.t.Error("tree does not preserve base")
		}
		rows := body["tree"].([]any)
		item := rows[0].(map[string]any)
		if item["path"] != filePath || item["mode"] != "100644" || item["type"] != "blob" {
			f.t.Error("unsafe tree entry")
		}
		f.content = item["content"].(string)
		f.reply(w, 201, map[string]string{"sha": treeSHA})
	case r.Method == "POST" && ep == "/git/commits":
		f.writes["commit"]++
		body := f.read(r)
		f.commitMessage = body["message"].(string)
		if body["parents"].([]any)[0] != baseSHA {
			f.t.Error("commit missing frozen parent")
		}
		f.reply(w, 201, map[string]string{"sha": headSHA})
	case r.Method == "POST" && ep == "/git/refs":
		f.writes["branch"]++
		body := f.read(r)
		if body["ref"] != "refs/heads/"+candidate().Branch || body["sha"] != headSHA {
			f.t.Error("unexpected branch create")
		}
		if _, ok := body["force"]; ok {
			f.t.Error("force must not be sent")
		}
		if f.branch {
			f.reply(w, 422, map[string]string{})
			return
		}
		f.branch = true
		if f.lost(w, "branch") {
			return
		}
		f.reply(w, 201, map[string]string{})
	case ep == "/pulls" && r.Method == "GET":
		if r.URL.Query().Get("head") != "acme:"+candidate().Branch || r.URL.Query().Get("state") != "all" {
			f.t.Error("review recovery missing branch/all filter")
		}
		rows := []any{}
		if f.review && (f.reviewBase == "" || r.URL.Query().Get("base") == "" || r.URL.Query().Get("base") == f.reviewBase) {
			review := f.ghPull()
			if f.reviewBase != "" {
				review["base"].(map[string]any)["ref"] = f.reviewBase
			}
			rows = append(rows, review)
		}
		f.reply(w, 200, rows)
	case ep == "/pulls" && r.Method == "POST":
		f.writes["review"]++
		f.review = true
		body := f.read(r)
		f.reviewBody = body["body"].(string)
		if body["base"] != "main" || body["head"] != candidate().Branch || body["maintainer_can_modify"] != false {
			f.t.Error("review target not frozen")
		}
		if f.lost(w, "review") {
			return
		}
		f.reply(w, 201, f.ghPull())
	case ep == "/pulls/9" && r.Method == "GET":
		if f.merged && f.loss == "confirmation" {
			f.loss = ""
			f.reply(w, 404, map[string]string{})
			return
		}
		review := f.ghPull()
		if f.reviewBase != "" {
			review["base"].(map[string]any)["ref"] = f.reviewBase
		}
		f.reply(w, 200, review)
	case ep == "/pulls/9/files" && r.Method == "GET":
		rows := []map[string]string{{"filename": filePath, "status": "added"}}
		if f.truncateDiff {
			rows = nil
		}
		if f.extraFile {
			rows = append(rows, map[string]string{"filename": "secret.md", "status": "added"})
		}
		f.reply(w, 200, rows)
	case (ep == "/branches/main" || ep == "/branches/release") && r.Method == "GET":
		f.reply(w, 200, map[string]any{"protected": f.protected})
	case (ep == "/rules/branches/main" || ep == "/rules/branches/release") && r.Method == "GET":
		if f.policyStatus != 0 {
			f.reply(w, f.policyStatus, map[string]string{})
			return
		}
		if f.rules == nil {
			f.reply(w, 200, []any{})
		} else {
			f.reply(w, 200, f.rules)
		}
	case ep == "/rulesets/17" && r.Method == "GET":
		bypass := f.rulesetBypass
		if bypass == nil {
			bypass = []any{}
		}
		f.reply(w, 200, map[string]any{"enforcement": "active", "bypass_actors": bypass})
	case ep == "/branches/main/protection" && r.Method == "GET":
		if f.classicStatus != 0 {
			f.reply(w, f.classicStatus, map[string]string{})
			return
		}
		protection := map[string]any{"enforce_admins": map[string]bool{"enabled": f.enforceAdmins}}
		if f.requireReviews {
			protection["required_pull_request_reviews"] = map[string]any{"required_approving_review_count": 1, "bypass_pull_request_allowances": map[string]any{"users": []any{}, "teams": []any{}, "apps": []any{}}}
		}
		if f.requireChecks {
			protection["required_status_checks"] = map[string]any{"contexts": []string{"test"}}
		}
		f.reply(w, 200, protection)
	case r.URL.Path == "/graphql" && r.Method == "POST":
		body := f.read(r)
		variables := body["variables"].(map[string]any)
		if strings.Contains(body["query"].(string), "branchProtectionRule") {
			if variables["ref"] != "refs/heads/main" || variables["owner"] != "acme" || variables["repo"] != "materials" {
				f.t.Error("classic rule proof scope mismatch")
			}
			ref := map[string]any{"name": "main", "prefix": "refs/heads/", "branchProtectionRule": nil}
			response := map[string]any{"data": map[string]any{"repository": map[string]any{"ref": ref}}}
			switch f.classicProof {
			case "absent":
			case "present":
				ref["branchProtectionRule"] = map[string]string{"id": "classic-rule"}
			case "missing":
				delete(ref, "branchProtectionRule")
			case "null_ref":
				response["data"].(map[string]any)["repository"].(map[string]any)["ref"] = nil
			case "wrong_ref":
				ref["name"] = "release"
			case "wrong_prefix":
				ref["prefix"] = "refs/tags/"
			default:
				response["errors"] = []any{map[string]string{"message": "private rule unavailable"}}
			}
			f.reply(w, 200, response)
			return
		}
		if variables["number"] != float64(9) || variables["owner"] != "acme" || variables["repo"] != "materials" {
			f.t.Error("GraphQL scope mismatch")
		}
		var decision any
		if f.approval != "" {
			decision = f.approval
		}
		var rollup any
		if f.checkState != "" {
			rollup = map[string]string{"state": f.checkState}
		}
		baseRef := "main"
		if f.reviewBase != "" {
			baseRef = f.reviewBase
		}
		f.reply(w, 200, map[string]any{"data": map[string]any{"repository": map[string]any{"pullRequest": map[string]any{"headRefOid": f.head(), "baseRefName": baseRef, "mergeStateStatus": "CLEAN", "reviewDecision": decision, "commits": map[string]any{"nodes": []any{map[string]any{"commit": map[string]any{"oid": f.head(), "statusCheckRollup": rollup}}}}}}}})
	case ep == "/pulls/9/merge" && r.Method == "PUT":
		body := f.read(r)
		if body["sha"] != headSHA || body["merge_method"] != "squash" {
			f.t.Error("merge is not bound to expected HEAD")
		}
		f.writes["merge"]++
		f.merged = true
		if f.afterMerge != nil {
			f.afterMerge(f)
		}
		if f.lost(w, "merge") {
			return
		}
		f.reply(w, 200, map[string]any{"merged": true, "sha": mergeSHA})
	case ep == "/issues/9/comments":
		f.serveNotes(w, r)
	default:
		f.t.Errorf("unexpected GitHub request %s %s", r.Method, r.URL)
		f.reply(w, 500, map[string]string{})
	}
}
func (f *remote) serveGL(w http.ResponseWriter, r *http.Request) {
	ep := strings.TrimPrefix(r.URL.Path, "/api/v4/projects/acme/materials")
	switch {
	case r.Method == "GET" && strings.HasPrefix(ep, "/repository/branches/"):
		branch := strings.TrimPrefix(ep, "/repository/branches/")
		sha := baseSHA
		if branch != "main" {
			if !f.branch {
				f.reply(w, 404, map[string]string{})
				return
			}
			sha = f.head()
			if f.commitMessage == "" {
				sha = baseSHA
			}
		}
		f.reply(w, 200, map[string]any{"name": branch, "commit": map[string]any{"id": sha, "message": f.commitMessage, "parent_ids": []string{baseSHA}}})
	case r.Method == "GET" && ep == "/repository/tree":
		if f.treeUnavailable {
			f.reply(w, 404, map[string]string{})
			return
		}
		prefix := r.URL.Query().Get("path")
		rows := []map[string]string{}
		switch prefix {
		case "":
			rows = append(rows, map[string]string{"id": treeSHA, "path": "requirements", "type": "tree", "mode": "040000"})
		case "requirements":
			rows = append(rows, map[string]string{"id": treeSHA, "path": "requirements/req-1", "type": "tree", "mode": "040000"})
		case "requirements/req-1":
			if ((f.branch || f.review) && f.content != "") || f.existingBase {
				rows = append(rows, map[string]string{"id": blobSHA, "path": filePath, "type": "blob", "mode": f.mode})
			}
		}
		f.reply(w, 200, rows)
	case r.Method == "GET" && strings.HasPrefix(ep, "/repository/files/"):
		sha := r.URL.Query().Get("ref")
		f.reply(w, 200, map[string]any{"file_path": filePath, "size": len(f.content), "encoding": "base64", "content": base64.StdEncoding.EncodeToString([]byte(f.content)), "commit_id": sha, "last_commit_id": headSHA, "blob_id": blobSHA})
	case r.Method == "POST" && ep == "/repository/branches":
		f.writes["branch"]++
		body := f.read(r)
		if body["branch"] != candidate().Branch || body["ref"] != baseSHA {
			f.t.Error("branch create not pinned")
		}
		if f.branchCollision {
			f.branch = true
			f.reply(w, 400, map[string]string{})
			return
		}
		f.branch = true
		if f.lost(w, "branch") {
			return
		}
		f.reply(w, 201, map[string]any{"name": candidate().Branch, "commit": map[string]string{"id": baseSHA}})
	case r.Method == "POST" && ep == "/repository/commits":
		f.writes["commit"]++
		body := f.read(r)
		if !f.branch || body["force"] != false || body["start_sha"] != nil || body["branch"] != candidate().Branch {
			f.t.Error("commit is not pinned and non-force")
		}
		action := body["actions"].([]any)[0].(map[string]any)
		expectedAction := "create"
		if f.existingBase {
			expectedAction = "update"
			if action["last_commit_id"] != headSHA {
				f.t.Error("existing file update lost commit precondition")
			}
		}
		if action["action"] != expectedAction || action["file_path"] != filePath {
			f.t.Error("unsafe action")
		}
		f.content = action["content"].(string)
		f.commitMessage = body["commit_message"].(string)
		f.branch = true
		if f.lost(w, "commit") {
			return
		}
		f.reply(w, 201, map[string]string{"id": headSHA, "message": f.commitMessage})
	case ep == "/merge_requests" && r.Method == "GET":
		if r.URL.Query().Get("source_branch") != candidate().Branch || r.URL.Query().Get("state") != "all" {
			f.t.Error("recovery missing branch/all filter")
		}
		rows := []any{}
		if f.review && (f.reviewBase == "" || r.URL.Query().Get("target_branch") == "" || r.URL.Query().Get("target_branch") == f.reviewBase) {
			review := f.glMR()
			if f.reviewBase != "" {
				review["target_branch"] = f.reviewBase
			}
			rows = append(rows, review)
		}
		f.reply(w, 200, rows)
	case ep == "/merge_requests" && r.Method == "POST":
		f.writes["review"]++
		f.review = true
		body := f.read(r)
		f.reviewBody = body["description"].(string)
		if body["source_branch"] != candidate().Branch || body["target_branch"] != "main" || body["allow_collaboration"] != false {
			f.t.Error("MR target not frozen")
		}
		if f.lost(w, "review") {
			return
		}
		f.reply(w, 201, f.glMR())
	case ep == "/merge_requests/9" && r.Method == "GET":
		if f.merged && f.loss == "confirmation" {
			f.loss = ""
			f.reply(w, 404, map[string]string{})
			return
		}
		review := f.glMR()
		if f.reviewBase != "" {
			review["target_branch"] = f.reviewBase
		}
		if f.diffPending {
			review["changes_count"] = ""
			review["diff_refs"] = nil
		}
		if f.pipelineSHA != "" {
			review["head_pipeline"].(map[string]string)["sha"] = f.pipelineSHA
		}
		f.reply(w, 200, review)
	case ep == "/merge_requests/9/diffs" && r.Method == "GET":
		rows := []map[string]any{{"old_path": filePath, "new_path": filePath, "a_mode": "0", "b_mode": f.mode, "new_file": true}}
		if f.truncateDiff {
			rows = nil
		}
		if f.extraFile {
			rows = append(rows, map[string]any{"old_path": "secret.md", "new_path": "secret.md", "b_mode": "100644", "new_file": true})
		}
		f.reply(w, 200, rows)
	case ep == "/merge_requests/9/merge" && r.Method == "PUT":
		body := f.read(r)
		if body["sha"] != headSHA || body["squash"] != true || body["should_remove_source_branch"] != false {
			f.t.Error("merge not pinned")
		}
		f.writes["merge"]++
		f.merged = true
		if f.afterMerge != nil {
			f.afterMerge(f)
		}
		if f.lost(w, "merge") {
			return
		}
		f.reply(w, 200, f.glMR())
	case ep == "/merge_requests/9/notes":
		f.serveNotes(w, r)
	default:
		f.t.Errorf("unexpected GitLab request %s %s", r.Method, r.URL)
		f.reply(w, 500, map[string]string{})
	}
}
func (f *remote) serveNotes(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		rows := []map[string]string{}
		for _, body := range f.notes {
			rows = append(rows, map[string]string{"body": body})
		}
		f.reply(w, 200, rows)
		return
	}
	if r.Method != "POST" {
		f.t.Error("unexpected note mutation")
		return
	}
	f.writes["note"]++
	body := f.read(r)["body"].(string)
	f.notes = append(f.notes, body)
	if f.lost(w, "note") {
		return
	}
	f.reply(w, 201, map[string]string{"body": body})
}
func seed(f *remote) {
	in := candidate()
	f.branch = true
	f.review = true
	f.content = in.Files[0].Content
	f.commitMessage = in.Title + "\n\n" + candidateMarker(in)
	f.reviewBody = candidateMarker(in) + "\n\n" + in.Body
}

func TestReadFileAtMergeCommitPreservesExactBytes(t *testing.T) {
	for _, kind := range []string{"github", "gitlab"} {
		t.Run(kind, func(t *testing.T) {
			provider, remote := fixture(t, kind)
			seed(remote)
			body := "\uFEFF# Requirement\r\n\r\n" + strings.Repeat("AC: \u9700\u6c42\r\n", 10000)
			remote.content = body
			got, err := provider.ReadFile(context.Background(), mergeSHA, filePath)
			if err != nil || got != body {
				t.Fatalf("merge commit contents differ: got %d bytes, err=%v", len(got), err)
			}
			for _, request := range remote.requests {
				if kind == "gitlab" && !strings.Contains(request, "ref="+mergeSHA) ||
					kind == "github" && strings.Contains(request, "/git/commits/") && !strings.HasSuffix(request, "/git/commits/"+mergeSHA) {
					t.Fatalf("read was not pinned: %s", request)
				}
			}
			remote.content = "\xff"
			if _, err := provider.ReadFile(context.Background(), mergeSHA, filePath); err == nil {
				t.Fatal("invalid UTF-8 accepted")
			}
		})
	}
}

func TestReadFileRefusesNonRegularModes(t *testing.T) {
	for _, kind := range []string{"github", "gitlab"} {
		t.Run(kind, func(t *testing.T) {
			provider, remote := fixture(t, kind)
			seed(remote)
			for _, mode := range []string{"100755", "120000", "160000"} {
				remote.mode = mode
				if _, err := provider.ReadFile(t.Context(), mergeSHA, filePath); err == nil {
					t.Fatalf("accepted forbidden file mode %s", mode)
				}
			}
		})
	}
}

func TestGitLabMergeRejectsStaleSuccessfulPipeline(t *testing.T) {
	provider, remote := fixture(t, "gitlab")
	seed(remote)
	remote.pipelineSHA = baseSHA
	review, err := provider.Observe(context.Background(), 9)
	if err != nil || review.GatesPassed || remote.writes["merge"] != 0 {
		t.Fatalf("pipeline from different commit accepted: review=%+v err=%v", review, err)
	}
}

func TestGitHubRulesetNativeApprovalAndBypass(t *testing.T) {
	for _, scenario := range []string{"approved", "review_required", "bypass", "unknown_rule", "missing_parameters"} {
		t.Run(scenario, func(t *testing.T) {
			p, f := fixture(t, "github")
			seed(f)
			f.protected = true
			f.classicStatus = http.StatusNotFound
			f.classicProof = "absent"
			f.rules = []map[string]any{{"type": "pull_request", "ruleset_id": 17, "parameters": map[string]any{"required_approving_review_count": 1}}}
			switch scenario {
			case "review_required":
				f.approval = "REVIEW_REQUIRED"
			case "bypass":
				f.rulesetBypass = []any{map[string]any{"actor_type": "Integration", "actor_id": 1}}
			case "unknown_rule":
				f.rules[0]["type"] = "future_policy"
			case "missing_parameters":
				delete(f.rules[0], "parameters")
			}
			r, err := p.Observe(context.Background(), 9)
			if err != nil {
				t.Fatal(err)
			}
			if r.GatesPassed != (scenario == "approved") {
				t.Fatalf("unsafe policy outcome: %+v", r)
			}
		})
	}
}

func TestGitHubRulesetOnlyRequiresProofOfClassicRuleAbsence(t *testing.T) {
	for _, scenario := range []string{"absent", "present", "missing", "null_ref", "wrong_ref", "wrong_prefix", "permission_error", "rest_forbidden", "no_active_rules"} {
		t.Run(scenario, func(t *testing.T) {
			p, f := fixture(t, "github")
			seed(f)
			f.protected = true
			f.classicStatus = http.StatusNotFound
			f.classicProof = scenario
			f.rules = []map[string]any{{"type": "pull_request", "ruleset_id": 17, "parameters": map[string]any{"required_approving_review_count": 1}}}
			if scenario == "rest_forbidden" {
				f.classicProof = "absent"
				f.classicStatus = http.StatusForbidden
			}
			if scenario == "no_active_rules" {
				f.classicProof = "absent"
				f.rules = nil
			}
			r, err := p.Observe(context.Background(), 9)
			if scenario == "absent" {
				if err != nil || !r.GatesPassed || f.writes["merge"] != 0 {
					t.Fatalf("ruleset-only branch blocked: review=%+v err=%v", r, err)
				}
			} else if err != nil || r.GatesPassed || f.writes["merge"] != 0 || r.BlockReason != "native_protection_unavailable" {
				t.Fatalf("unknown classic rule allowed: review=%+v err=%v writes=%v", r, err, f.writes)
			}
		})
	}
}

func TestIncompleteProviderResponsesCannotAuthorizeOrLeak(t *testing.T) {
	for _, body := range []string{"null", "{", strings.Repeat("x", maxResponseBytes+1)} {
		t.Run(strconv.Itoa(len(body)), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) }))
			defer srv.Close()
			p, err := newClient(Connection{Provider: "github", BaseURL: srv.URL, Repository: "a/b", Token: "private"}, srv.Client(), true)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = p.Observe(context.Background(), 9); err == nil {
				t.Fatal("invalid response accepted")
			}
		})
	}
}

// The merge call must enforce the caller's frozen target and file set both
// before its write and when consuming the final receipt. A changed post-write
// receipt remains a remote fact, but must never authorize a local baseline.

func TestGitHubAdministratorBypassRefused(t *testing.T) {
	p, f := fixture(t, "github")
	seed(f)
	f.protected = true
	f.enforceAdmins = false
	r, err := p.Observe(context.Background(), 9)
	if err != nil || r.GatesPassed || r.BlockReason != "native_policy_bypass_not_supported" {
		t.Fatalf("bypass accepted: %v %+v", err, r)
	}
}
func TestGitHubRequiredChecksCannotBeMissing(t *testing.T) {
	p, f := fixture(t, "github")
	seed(f)
	f.protected = true
	f.requireChecks = true
	f.checkState = ""
	r, err := p.Observe(context.Background(), 9)
	if err != nil || r.GatesPassed || r.BlockReason != "native_checks_pending" {
		t.Fatalf("missing checks accepted: %v %+v", err, r)
	}
}

func TestConnectionValidation(t *testing.T) {
	for _, base := range []string{"http://gitlab.example", "https://user:secret@gitlab.example", "https://gitlab.example?token=secret", "https://gitlab.example/#fragment", "https://gitlab.example/../other"} {
		if _, err := New(Connection{Provider: "gitlab", BaseURL: base, Repository: "acme/materials", Token: "x"}); err == nil {
			t.Errorf("accepted %q", base)
		}
	}
	if _, err := New(Connection{Provider: "github", BaseURL: "https://other.example", Repository: "acme/materials", Token: "x"}); err == nil {
		t.Fatal("unadvertised GitHub Enterprise accepted")
	}
	if _, err := New(Connection{Provider: "gitlab", BaseURL: "https://gitlab.example/git", Repository: "group/nested/materials", Token: "x"}); err != nil {
		t.Fatal(err)
	}
}
func TestTransportRefusesRedirectsAndRedactsProviderFailure(t *testing.T) {
	called := false
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
	defer other.Close()
	for _, status := range []int{http.StatusTemporaryRedirect, http.StatusInternalServerError} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", other.URL)
				w.WriteHeader(status)
				_, _ = w.Write([]byte("private-test-token"))
			}))
			defer srv.Close()
			p, err := newClient(Connection{Provider: "gitlab", BaseURL: srv.URL, Repository: "a/b", Token: "private-test-token"}, srv.Client(), true)
			if err != nil {
				t.Fatal(err)
			}
			_, err = p.Observe(context.Background(), 9)
			if err == nil || strings.Contains(err.Error(), "private-test-token") {
				t.Fatalf("unsafe error %v", err)
			}
		})
	}
	if called {
		t.Fatal("credential redirect followed")
	}
}

func TestPaginationFollowsAllPagesWithoutFollowingLinks(t *testing.T) {
	for _, kind := range []string{"github", "gitlab"} {
		t.Run(kind, func(t *testing.T) {
			pages := 0
			in := DecisionNote{Key: "decision-1", ActorID: "user-1", RevisionID: "rev-1", HeadSHA: headSHA, Decision: "approve"}
			body, _ := decisionBody(in)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" {
					t.Error("existing decision was posted again")
					w.WriteHeader(500)
					return
				}
				pages++
				if r.URL.Query().Get("page") == "1" {
					rows := make([]map[string]string, 100)
					for i := range rows {
						rows[i] = map[string]string{"body": "other"}
					}
					w.Header().Set("Link", `<https://untrusted.invalid/?page=2>; rel="next"`)
					w.Header().Set("X-Next-Page", "2")
					_ = json.NewEncoder(w).Encode(rows)
				} else {
					_ = json.NewEncoder(w).Encode([]map[string]any{{"body": body, "user": map[string]string{"login": "actor"}, "author": map[string]string{"username": "actor"}}})
				}
			}))
			defer srv.Close()
			p, err := newClient(Connection{Provider: kind, BaseURL: srv.URL, Repository: "a/b", Token: "x"}, srv.Client(), true)
			if err != nil {
				t.Fatal(err)
			}
			var c *client
			switch v := p.(type) {
			case *githubProvider:
				c = v.client
			case *gitlabProvider:
				c = v.client
			}
			reader := &observer{reader: p, c: c}
			if _, err = reader.ObserveDecision(context.Background(), 9, marker(in.Key), body); err != nil {
				t.Fatal(err)
			}
			if pages != 2 {
				t.Fatalf("read %d pages", pages)
			}
		})
	}
}

func candidateMarker(in PublishRequest) string {
	files := append([]File(nil), in.Files...)
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	raw, _ := json.Marshal(struct {
		Key, Branch, BaseBranch string
		Files                   []File
	}{in.Key, in.Branch, in.BaseBranch, files})
	digest := sha256.Sum256(raw)
	return marker(in.Key) + "\n<!-- candidate-sha256:" + hex.EncodeToString(digest[:]) + " -->"
}
func decisionBody(in DecisionNote) (string, error) {
	if !keyPattern.MatchString(in.Key) || !keyPattern.MatchString(in.ActorID) || !keyPattern.MatchString(in.RevisionID) || !shaPattern.MatchString(in.HeadSHA) || len(in.Reason) > 16000 {
		return "", failure("invalid_decision_note")
	}
	switch in.Decision {
	case "approve", "request_changes", "withdraw":
	default:
		return "", failure("invalid_decision_note")
	}
	u, err := url.Parse(in.URL)
	if err != nil || u.User != nil || (in.URL != "" && (u.Scheme != "https" && u.Scheme != "http" || u.Host == "")) {
		return "", failure("invalid_decision_note")
	}
	return fmt.Sprintf("%s\nMultica business review (synchronized by the system integration; not a native Git approval).\n\nReviewer: `%s`\nRevision: `%s`\nReviewed HEAD: `%s`\nDecision: `%s`\n\n%s\n\nMultica record: %s", marker(in.Key), in.ActorID, in.RevisionID, in.HeadSHA, in.Decision, in.Reason, in.URL), nil
}
func marker(key string) string { return "<!-- multica-material:" + key + " -->" }

func TestMergedReviewRequiresProviderTimestamp(t *testing.T) {
	for _, kind := range []string{"github", "gitlab"} {
		t.Run(kind, func(t *testing.T) {
			provider, remote := fixture(t, kind)
			seed(remote)
			remote.merged = true
			got, err := provider.Observe(t.Context(), 9)
			if err != nil || got.MergedAt == nil || !got.MergedAt.Equal(time.Date(2026, 11, 27, 8, 0, 0, 0, time.UTC)) {
				t.Fatalf("merge timestamp lost: %v %v", got.MergedAt, err)
			}
			remote.mergedAt = nil
			if _, err := provider.Observe(t.Context(), 9); err == nil {
				t.Fatal("merged review without timestamp accepted")
			}
		})
	}
}
