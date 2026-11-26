package materialgit

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
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
	f := &remote{t: t, kind: kind, mode: "100644", enforceAdmins: true, approval: "APPROVED", checkState: "SUCCESS", glStatus: "mergeable", canMerge: true, writes: map[string]int{}}
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
	return map[string]any{"number": int64(9), "html_url": "https://github.com/acme/materials/pull/9", "body": f.reviewBody, "state": state, "draft": false, "merged": f.merged, "mergeable": true, "mergeable_state": "clean", "changed_files": count, "merge_commit_sha": mergeSHA, "merged_by": map[string]string{"login": "multica[bot]"}, "head": map[string]any{"ref": candidate().Branch, "sha": f.head(), "repo": map[string]string{"full_name": "acme/materials"}}, "base": map[string]any{"ref": "main", "sha": baseSHA, "repo": map[string]string{"full_name": "acme/materials"}}}
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
	return map[string]any{"iid": 9, "project_id": 1, "source_project_id": 1, "target_project_id": 1, "web_url": strings.Replace(f.server.URL, "http://", "https://", 1) + "/acme/materials/-/merge_requests/9", "description": f.reviewBody, "state": state, "sha": f.head(), "source_branch": candidate().Branch, "target_branch": "main", "draft": false, "detailed_merge_status": f.glStatus, "has_conflicts": false, "changes_count": count, "merge_commit_sha": mergeSHA, "merge_user": map[string]string{"username": "multica"}, "user": map[string]bool{"can_merge": f.canMerge}, "diff_refs": map[string]string{"head_sha": f.head()}, "head_pipeline": map[string]string{"sha": f.head(), "status": strings.ToLower(f.checkState)}}
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

func TestPublishRecoversRealEffectsWithoutRepeatingWrites(t *testing.T) {
	for _, kind := range []string{"github", "gitlab"} {
		losses := []string{"", "branch", "review"}
		if kind == "gitlab" {
			losses = []string{"", "commit", "review"}
		}
		for _, loss := range losses {
			t.Run(kind+"/"+loss, func(t *testing.T) {
				p, f := fixture(t, kind)
				f.loss = loss
				_, err := p.Publish(context.Background(), candidate())
				if loss != "" {
					var remoteError *Error
					if !errors.As(err, &remoteError) || !remoteError.Uncertain {
						t.Fatalf("want uncertain response, got %v", err)
					}
				} else if err != nil {
					t.Fatal(err)
				}
				r, err := p.Publish(context.Background(), candidate())
				if err != nil {
					t.Fatal(err)
				}
				if r.Number != 9 || r.HeadSHA != headSHA || r.State != "open" || !r.GatesPassed || len(r.Changes) != 1 {
					t.Fatalf("bad observed review: %+v", r)
				}
				if f.writes["branch"] != 1 || f.writes["commit"] != 1 || f.writes["review"] != 1 {
					t.Fatalf("duplicate effects: %v", f.writes)
				}
				body, err := p.ReadFile(context.Background(), headSHA, filePath)
				if err != nil || body != candidate().Files[0].Content {
					t.Fatalf("fixed body mismatch %v", err)
				}
			})
		}
	}
}

func TestPublishNeverRecreatesRetargetedOrDeletedReviewBranch(t *testing.T) {
	for _, kind := range []string{"github", "gitlab"} {
		for _, change := range []string{"deleted_branch", "retargeted_review"} {
			t.Run(kind+"/"+change, func(t *testing.T) {
				provider, remote := fixture(t, kind)
				seed(remote)
				if change == "deleted_branch" {
					remote.branch = false
				} else {
					remote.reviewBase = "release"
				}
				if _, err := provider.Publish(context.Background(), candidate()); err == nil || len(remote.writes) != 0 {
					t.Fatalf("existing review should block new writes: err=%v writes=%v", err, remote.writes)
				}
			})
		}
	}
}

func TestGitLabPublishWaitsForDiffMetadata(t *testing.T) {
	provider, remote := fixture(t, "gitlab")
	remote.diffPending = true
	_, err := provider.Publish(context.Background(), candidate())
	var pending *Error
	if !errors.As(err, &pending) || !pending.Retryable {
		t.Fatalf("new MR diff metadata should be retried: %v", err)
	}
	remote.diffPending = false
	review, err := provider.Publish(context.Background(), candidate())
	if err != nil || review.Number != 9 || remote.writes["review"] != 1 || remote.writes["commit"] != 1 || remote.writes["branch"] != 1 {
		t.Fatalf("did not recover original MR: review=%+v err=%v writes=%v", review, err, remote.writes)
	}
}

func TestUnreadableBaseTreePreventsPublication(t *testing.T) {
	for _, kind := range []string{"github", "gitlab"} {
		t.Run(kind, func(t *testing.T) {
			provider, remote := fixture(t, kind)
			remote.treeUnavailable = true
			_, err := provider.Publish(context.Background(), candidate())
			if !isStatus(err, 404) || len(remote.writes) != 0 {
				t.Fatalf("unreadable tree allowed writes: err=%v writes=%v", err, remote.writes)
			}
		})
	}
}

func TestMissingConfirmationAfterMergeIsUncertain(t *testing.T) {
	for _, kind := range []string{"github", "gitlab"} {
		t.Run(kind, func(t *testing.T) {
			provider, remote := fixture(t, kind)
			seed(remote)
			remote.loss = "confirmation"
			_, err := provider.Merge(context.Background(), mergeExpectation(kind))
			var pending *Error
			if !errors.As(err, &pending) || !pending.Uncertain {
				t.Fatalf("unconfirmed merge was not marked uncertain: %v", err)
			}
			review, err := provider.Merge(context.Background(), mergeExpectation(kind))
			if err != nil || review.State != "merged" || remote.writes["merge"] != 1 {
				t.Fatalf("merge recovery failed: review=%+v err=%v writes=%v", review, err, remote.writes)
			}
		})
	}
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

func TestGitLabMergeRejectsStaleSuccessfulPipeline(t *testing.T) {
	provider, remote := fixture(t, "gitlab")
	seed(remote)
	remote.pipelineSHA = baseSHA
	review, err := provider.Merge(context.Background(), mergeExpectation("gitlab"))
	if err == nil || review.GatesPassed || remote.writes["merge"] != 0 {
		t.Fatalf("pipeline from different commit accepted: review=%+v err=%v", review, err)
	}
}

func TestGitLabUnconfirmedBranchOwnershipNeverWritesCommit(t *testing.T) {
	for _, collision := range []bool{false, true} {
		t.Run(strconv.FormatBool(collision), func(t *testing.T) {
			p, f := fixture(t, "gitlab")
			if collision {
				f.branchCollision = true
			} else {
				f.loss = "branch"
			}
			if _, err := p.Publish(context.Background(), candidate()); err == nil {
				t.Fatal("ambiguous creation was accepted")
			}
			if !f.branch || f.writes["branch"] != 1 || f.writes["commit"] != 0 || f.review {
				t.Fatalf("unsafe side effect %v", f.writes)
			}
			_, err := p.Publish(context.Background(), candidate())
			var e *Error
			if !errors.As(err, &e) || e.Code != "candidate_branch_ownership_unknown" {
				t.Fatalf("missing ownership blocker: %v", err)
			}
			if f.writes["branch"] != 1 || f.writes["commit"] != 0 || f.review {
				t.Fatalf("recovery adopted unmarked branch: %v", f.writes)
			}
		})
	}
}

func TestNewRevisionUpdatesExistingMaterialWithFilePrecondition(t *testing.T) {
	for _, kind := range []string{"github", "gitlab"} {
		t.Run(kind, func(t *testing.T) {
			p, f := fixture(t, kind)
			f.existingBase = true
			f.content = "# Previously approved requirement"
			if _, err := p.Publish(context.Background(), candidate()); err != nil {
				t.Fatal(err)
			}
			if f.writes["commit"] != 1 || f.content != candidate().Files[0].Content {
				t.Fatalf("revision update failed: %v", f.writes)
			}
		})
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
			r, err := p.Merge(context.Background(), mergeExpectation("github"))
			if scenario == "absent" {
				if err != nil || r.State != "merged" || f.writes["merge"] != 1 {
					t.Fatalf("ruleset-only branch blocked: review=%+v err=%v", r, err)
				}
			} else if err == nil || f.writes["merge"] != 0 || r.BlockReason != "native_protection_unavailable" {
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
func TestPublishRefusesForeignOrModifiedCandidate(t *testing.T) {
	for _, kind := range []string{"github", "gitlab"} {
		for _, mutation := range []string{"foreign_marker", "content", "extra_file", "symlink", "truncated_diff"} {
			t.Run(kind+"/"+mutation, func(t *testing.T) {
				p, f := fixture(t, kind)
				seed(f)
				switch mutation {
				case "foreign_marker":
					f.commitMessage = "human branch"
				case "content":
					f.content = "human edit"
				case "extra_file":
					f.extraFile = true
				case "symlink":
					f.mode = "120000"
				case "truncated_diff":
					f.truncateDiff = true
				}
				if _, err := p.Publish(context.Background(), candidate()); err == nil {
					t.Fatal("changed candidate was accepted")
				}
				if len(f.writes) != 0 {
					t.Fatalf("changed candidate caused writes: %v", f.writes)
				}
			})
		}
	}
}
func TestConditionalMergeAndLostResponseRecovery(t *testing.T) {
	for _, kind := range []string{"github", "gitlab"} {
		for _, loss := range []bool{false, true} {
			t.Run(kind+"/loss="+strconv.FormatBool(loss), func(t *testing.T) {
				p, f := fixture(t, kind)
				seed(f)
				if loss {
					f.loss = "merge"
				}
				r, err := p.Merge(context.Background(), mergeExpectation(kind))
				if loss {
					var e *Error
					if !errors.As(err, &e) || !e.Uncertain {
						t.Fatalf("expected uncertainty: %v", err)
					}
				} else if err != nil || r.State != "merged" {
					t.Fatalf("merge failed %v %+v", err, r)
				}
				r, err = p.Merge(context.Background(), mergeExpectation(kind))
				if err != nil || r.State != "merged" || r.MergeSHA != mergeSHA || r.MergeActor == "" {
					t.Fatalf("recovery failed %v %+v", err, r)
				}
				if f.writes["merge"] != 1 {
					t.Fatalf("merge repeated %v", f.writes)
				}
			})
		}
	}
}
func TestMergeRefusesChangedHeadAndNativeGates(t *testing.T) {
	for _, kind := range []string{"github", "gitlab"} {
		for _, mutation := range []string{"head", "checks", "approval", "permission", "unknown"} {
			t.Run(kind+"/"+mutation, func(t *testing.T) {
				p, f := fixture(t, kind)
				seed(f)
				switch mutation {
				case "head":
					f.foreignHead = true
				case "checks":
					f.checkState = "PENDING"
				case "approval":
					f.protected = true
					f.requireReviews = true
					f.approval = "REVIEW_REQUIRED"
					f.glStatus = "not_approved"
				case "permission":
					f.policyStatus = 403
					f.canMerge = false
				case "unknown":
					f.policyStatus = 404
					f.glStatus = "new_unknown_state"
				}
				if _, err := p.Merge(context.Background(), mergeExpectation(kind)); err == nil {
					t.Fatal("unsafe merge accepted")
				}
				if f.writes["merge"] != 0 {
					t.Fatal("native checks bypassed")
				}
			})
		}
	}
}

// The merge call must enforce the caller's frozen target and file set both
// before its write and when consuming the final receipt. A changed post-write
// receipt remains a remote fact, but must never authorize a local baseline.
func TestMergeRechecksApprovedTargetAndFiles(t *testing.T) {
	for _, kind := range []string{"github", "gitlab"} {
		for _, after := range []bool{false, true} {
			for _, mutation := range []string{"target", "extra_file"} {
				t.Run(kind+"/after="+strconv.FormatBool(after)+"/"+mutation, func(t *testing.T) {
					p, f := fixture(t, kind)
					seed(f)
					expectedCode := "candidate_files_changed"
					mutate := func(remote *remote) { remote.extraFile = true }
					if mutation == "target" {
						expectedCode = "review_target_changed"
						mutate = func(remote *remote) { remote.reviewBase = "release" }
					}
					if after {
						f.afterMerge = mutate
					} else {
						mutate(f)
					}
					_, err := p.Merge(context.Background(), mergeExpectation(kind))
					var rejected *Error
					if !errors.As(err, &rejected) || rejected.Code != expectedCode {
						t.Fatalf("changed review not rejected: err=%v", err)
					}
					wantWrites := 0
					if after {
						wantWrites = 1
					}
					if f.writes["merge"] != wantWrites || f.merged != after {
						t.Fatalf("unexpected merge effect: writes=%v merged=%v", f.writes, f.merged)
					}
				})
			}
		}
	}
}

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
func TestDecisionSyncRecoversSameNoteAndRejectsConflictingKey(t *testing.T) {
	for _, kind := range []string{"github", "gitlab"} {
		t.Run(kind, func(t *testing.T) {
			p, f := fixture(t, kind)
			seed(f)
			f.loss = "note"
			in := DecisionNote{Key: "decision-1", ActorID: "user-1", RevisionID: "rev-1", HeadSHA: headSHA, Decision: "approve", Reason: "Checked AC-1", URL: "https://multica.example/project/1"}
			if err := p.SyncDecision(context.Background(), 9, in); err == nil {
				t.Fatal("expected lost response")
			}
			if err := p.SyncDecision(context.Background(), 9, in); err != nil {
				t.Fatal(err)
			}
			if f.writes["note"] != 1 || !strings.Contains(f.notes[0], "not a native Git approval") || !strings.Contains(f.notes[0], "user-1") {
				t.Fatalf("wrong audit: %v", f.writes)
			}
			in.Decision = "withdraw"
			if err := p.SyncDecision(context.Background(), 9, in); err == nil {
				t.Fatal("conflicting replay accepted")
			}
		})
	}
}

func TestInvalidCandidateHasNoNetworkEffects(t *testing.T) {
	for _, kind := range []string{"github", "gitlab"} {
		t.Run(kind, func(t *testing.T) {
			p, f := fixture(t, kind)
			for _, name := range []string{"../prd.md", "requirements/a/../../AGENTS.md", "AGENTS.md", "requirements/a/AGENTS.md", "requirements/a/%2e%2e/prd.md", "requirements/a/prd.md\x00", ".git/requirements/a/prd.md"} {
				in := candidate()
				in.Files[0].Path = name
				if _, err := p.Publish(context.Background(), in); err == nil {
					t.Errorf("allowed %q", name)
				}
			}
			in := candidate()
			in.Branch = "main"
			if _, err := p.Publish(context.Background(), in); err == nil {
				t.Error("direct base write allowed")
			}
			if _, err := p.ReadFile(context.Background(), "main", filePath); err == nil {
				t.Error("mutable ref allowed")
			}
			if len(f.requests) != 0 {
				t.Fatalf("invalid request used network: %v", f.requests)
			}
		})
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
					_ = json.NewEncoder(w).Encode([]map[string]string{{"body": body}})
				}
			}))
			defer srv.Close()
			p, err := newClient(Connection{Provider: kind, BaseURL: srv.URL, Repository: "a/b", Token: "x"}, srv.Client(), true)
			if err != nil {
				t.Fatal(err)
			}
			if err = p.SyncDecision(context.Background(), 9, in); err != nil {
				t.Fatal(err)
			}
			if pages != 2 {
				t.Fatalf("read %d pages", pages)
			}
		})
	}
}
