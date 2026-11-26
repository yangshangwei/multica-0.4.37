package materialgit

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"unicode/utf8"
)

type gitlabProvider struct{ *client }
type glCommit struct {
	ID        string   `json:"id"`
	Message   string   `json:"message"`
	ParentIDs []string `json:"parent_ids"`
}
type glMR struct {
	IID                 int64                     `json:"iid"`
	ProjectID           int64                     `json:"project_id"`
	SourceProjectID     int64                     `json:"source_project_id"`
	TargetProjectID     int64                     `json:"target_project_id"`
	URL                 string                    `json:"web_url"`
	Description         string                    `json:"description"`
	SHA                 string                    `json:"sha"`
	SourceBranch        string                    `json:"source_branch"`
	TargetBranch        string                    `json:"target_branch"`
	State               string                    `json:"state"`
	Draft               bool                      `json:"draft"`
	WorkInProgress      bool                      `json:"work_in_progress"`
	DetailedMergeStatus string                    `json:"detailed_merge_status"`
	HasConflicts        *bool                     `json:"has_conflicts"`
	ChangesCount        string                    `json:"changes_count"`
	MergeSHA            string                    `json:"merge_commit_sha"`
	SquashSHA           string                    `json:"squash_commit_sha"`
	MergedBy            struct{ Username string } `json:"merge_user"`
	LegacyMergedBy      struct{ Username string } `json:"merged_by"`
	User                struct {
		CanMerge *bool `json:"can_merge"`
	} `json:"user"`
	DiffRefs struct {
		HeadSHA string `json:"head_sha"`
	} `json:"diff_refs"`
	HeadPipeline *struct{ SHA, Status string } `json:"head_pipeline"`
}
type glFile struct {
	Path              string `json:"file_path"`
	Size              *int   `json:"size"`
	Encoding, Content string
	CommitID          string `json:"commit_id"`
	LastCommitID      string `json:"last_commit_id"`
	BlobID            string `json:"blob_id"`
}
type glTreeEntry struct{ ID, Name, Type, Path, Mode string }

func (p *gitlabProvider) branch(ctx context.Context, name string) (glCommit, error) {
	var result struct {
		Name   string
		Commit glCommit
	}
	_, err := p.request(ctx, http.MethodGet, p.repo+"/repository/branches/"+url.PathEscape(name), nil, &result)
	if err != nil {
		return result.Commit, err
	}
	if result.Name != name || !shaPattern.MatchString(result.Commit.ID) {
		return result.Commit, failure("provider_ref_invalid")
	}
	return result.Commit, nil
}
func (p *gitlabProvider) treeEntry(ctx context.Context, sha, name string) (glTreeEntry, error) {
	q := url.Values{"ref": {sha}, "per_page": {"100"}, "path": {path.Dir(name)}}
	if path.Dir(name) == "." {
		q.Del("path")
	}
	var found *glTreeEntry
	for page := 1; ; page++ {
		q.Set("page", strconv.Itoa(page))
		var rows []glTreeEntry
		h, err := p.request(ctx, http.MethodGet, p.repo+"/repository/tree?"+q.Encode(), nil, &rows)
		if err != nil {
			return glTreeEntry{}, err
		}
		for _, row := range rows {
			if row.Path == name {
				if found != nil {
					return glTreeEntry{}, failure("provider_tree_invalid")
				}
				v := row
				found = &v
			}
		}
		next, err := nextPage(h, page, len(rows), false)
		if err != nil {
			return glTreeEntry{}, err
		}
		if !next {
			break
		}
	}
	if found == nil {
		return glTreeEntry{}, &Error{Code: "provider_tree_entry_missing", Status: 404}
	}
	if !shaPattern.MatchString(found.ID) {
		return glTreeEntry{}, failure("provider_tree_invalid")
	}
	return *found, nil
}
func (p *gitlabProvider) fileEntry(ctx context.Context, sha, name string) (glTreeEntry, error) {
	parts := strings.Split(name, "/")
	for i := range parts {
		entry, err := p.treeEntry(ctx, sha, strings.Join(parts[:i+1], "/"))
		if err != nil {
			return glTreeEntry{}, err
		}
		if i == len(parts)-1 {
			if entry.Type != "blob" || entry.Mode != "100644" {
				return glTreeEntry{}, failure("material_file_not_regular")
			}
			return entry, nil
		}
		if entry.Type != "tree" || entry.Mode != "040000" {
			return glTreeEntry{}, failure("material_path_not_directory")
		}
	}
	return glTreeEntry{}, failure("invalid_file_reference")
}
func (p *gitlabProvider) file(ctx context.Context, sha, name string) (glFile, error) {
	var file glFile
	_, err := p.request(ctx, http.MethodGet, p.repo+"/repository/files/"+url.PathEscape(name)+"?ref="+url.QueryEscape(sha), nil, &file)
	if err != nil {
		return file, err
	}
	if file.Path != name || file.CommitID != sha || !shaPattern.MatchString(file.LastCommitID) || !shaPattern.MatchString(file.BlobID) || file.Size == nil || *file.Size < 0 || *file.Size > MaxFileBytes || file.Encoding != "base64" {
		return file, failure("provider_file_invalid")
	}
	return file, nil
}
func (p *gitlabProvider) ReadFile(ctx context.Context, sha, name string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	if err := validateRead(sha, name); err != nil {
		return "", err
	}
	entry, err := p.fileEntry(ctx, sha, name)
	if err != nil {
		return "", err
	}
	file, err := p.file(ctx, sha, name)
	if err != nil {
		return "", err
	}
	if entry.ID != file.BlobID {
		return "", failure("provider_file_invalid")
	}
	b, err := base64.StdEncoding.DecodeString(file.Content)
	if err != nil || len(b) != *file.Size || !utf8.Valid(b) {
		return "", failure("provider_file_incomplete")
	}
	return string(b), nil
}

func (p *gitlabProvider) Publish(ctx context.Context, in PublishRequest) (Review, error) {
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	if err := validatePublish(in); err != nil {
		return Review{}, err
	}
	q := url.Values{"state": {"all"}, "source_branch": {in.Branch}, "scope": {"all"}, "per_page": {"100"}}
	var found *glMR
	for page := 1; ; page++ {
		q.Set("page", strconv.Itoa(page))
		var rows []glMR
		h, err := p.request(ctx, http.MethodGet, p.repo+"/merge_requests?"+q.Encode(), nil, &rows)
		if err != nil {
			return Review{}, err
		}
		for _, row := range rows {
			if found != nil {
				return Review{}, failure("candidate_review_ambiguous")
			}
			v := row
			found = &v
		}
		next, err := nextPage(h, page, len(rows), false)
		if err != nil {
			return Review{}, err
		}
		if !next {
			break
		}
	}
	if found != nil && found.TargetBranch != in.BaseBranch {
		return Review{}, failure("candidate_review_changed")
	}
	commit, err := p.branch(ctx, in.Branch)
	if err != nil && !isStatus(err, 404) {
		return Review{}, err
	}
	if isStatus(err, 404) {
		if found != nil {
			return Review{}, failure("candidate_branch_missing")
		}
		base, err := p.branch(ctx, in.BaseBranch)
		if err != nil {
			return Review{}, err
		}
		actions := make([]map[string]any, 0, len(in.Files))
		for _, file := range in.Files {
			action := map[string]any{"action": "create", "file_path": file.Path, "content": file.Content, "encoding": "text"}
			_, err := p.fileEntry(ctx, base.ID, file.Path)
			if err == nil {
				old, err := p.file(ctx, base.ID, file.Path)
				if err != nil {
					return Review{}, err
				}
				action["action"] = "update"
				action["last_commit_id"] = old.LastCommitID
			} else if !isMissingTreeEntry(err) {
				return Review{}, err
			}
			actions = append(actions, action)
		}
		// GitLab's commits API can append to an existing branch even when
		// start_sha is supplied. Claim the name through the create-only branches
		// endpoint first. An ambiguous creation cannot authorize a commit.
		var created struct {
			Name   string
			Commit glCommit
		}
		_, err = p.request(ctx, http.MethodPost, p.repo+"/repository/branches", map[string]string{"branch": in.Branch, "ref": base.ID}, &created)
		if err != nil {
			return Review{}, err
		}
		if created.Name != in.Branch || created.Commit.ID != base.ID {
			return Review{}, &Error{Code: "candidate_branch_ownership_unknown", Uncertain: true}
		}
		current, err := p.branch(ctx, in.Branch)
		if err != nil {
			return Review{}, err
		}
		if current.ID != base.ID {
			return Review{}, failure("candidate_branch_conflict")
		}
		// This is not a branch-head CAS. File last_commit_id guards updates;
		// create actions reject an intervening file. Unrelated concurrent edits
		// are preserved and the full diff is checked before returning a review.
		_, err = p.request(ctx, http.MethodPost, p.repo+"/repository/commits", map[string]any{"branch": in.Branch, "commit_message": in.Title + "\n\n" + candidateMarker(in), "actions": actions, "force": false}, &commit)
		if err != nil {
			return Review{}, err
		}
	}
	if !shaPattern.MatchString(commit.ID) || !strings.Contains(commit.Message, candidateMarker(in)) {
		return Review{}, failure("candidate_branch_ownership_unknown")
	}
	if err = sameFiles(ctx, p, commit.ID, in.Files); err != nil {
		return Review{}, err
	}
	if found == nil {
		var made glMR
		_, err = p.request(ctx, http.MethodPost, p.repo+"/merge_requests", map[string]any{"source_branch": in.Branch, "target_branch": in.BaseBranch, "title": in.Title, "description": candidateMarker(in) + "\n\n" + in.Body, "remove_source_branch": false, "allow_collaboration": false}, &made)
		if err != nil {
			return Review{}, err
		}
		found = &made
	}
	raw, review, err := p.observe(ctx, found.IID)
	if err != nil {
		return Review{}, err
	}
	if !strings.Contains(raw.Description, candidateMarker(in)) || review.HeadSHA != commit.ID || review.Branch != in.Branch || review.BaseBranch != in.BaseBranch {
		return Review{}, failure("candidate_review_changed")
	}
	if err = checkChanges(review.Changes, in.Files); err != nil {
		return Review{}, err
	}
	if review.State == "closed" {
		return Review{}, failure("candidate_review_closed")
	}
	return review, nil
}

func (p *gitlabProvider) Observe(ctx context.Context, number int64) (Review, error) {
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	_, r, e := p.observe(ctx, number)
	return r, e
}
func (p *gitlabProvider) observe(ctx context.Context, number int64) (glMR, Review, error) {
	var raw glMR
	if number <= 0 {
		return raw, Review{}, failure("invalid_review")
	}
	endpoint := p.repo + "/merge_requests/" + strconv.FormatInt(number, 10)
	_, err := p.request(ctx, http.MethodGet, endpoint+"?with_merge_status_recheck=true", nil, &raw)
	if err != nil {
		return raw, Review{}, err
	}
	if raw.IID != number || !p.validateReviewURL(raw.URL, number) || raw.ProjectID <= 0 || raw.SourceProjectID != raw.ProjectID || raw.TargetProjectID != raw.ProjectID || !validRef(raw.SourceBranch) || !validRef(raw.TargetBranch) {
		return raw, Review{}, failure("provider_review_invalid")
	}
	if raw.State == "opened" && (raw.ChangesCount == "" || raw.DiffRefs.HeadSHA == "") {
		return raw, Review{}, &Error{Code: "provider_diff_pending", Retryable: true}
	}
	count, err := strconv.Atoi(raw.ChangesCount)
	if err != nil || count < 1 || count > 1000 || !shaPattern.MatchString(raw.SHA) || raw.DiffRefs.HeadSHA != raw.SHA {
		return raw, Review{}, failure("provider_review_invalid")
	}
	r := Review{Number: number, URL: raw.URL, HeadSHA: raw.SHA, BaseBranch: raw.TargetBranch, Branch: raw.SourceBranch, State: raw.State, MergeSHA: raw.MergeSHA, MergeActor: raw.MergedBy.Username, Changes: []Change{}}
	if r.State == "opened" {
		r.State = "open"
	}
	if r.MergeSHA == "" {
		r.MergeSHA = raw.SquashSHA
	}
	if r.MergeActor == "" {
		r.MergeActor = raw.LegacyMergedBy.Username
	}
	if r.State != "open" && r.State != "closed" && r.State != "merged" {
		return raw, Review{}, failure("provider_review_invalid")
	}
	if r.State == "merged" && (!shaPattern.MatchString(r.MergeSHA) || r.MergeActor == "") {
		return raw, Review{}, failure("provider_merge_incomplete")
	}
	seen := map[string]bool{}
	for page := 1; ; page++ {
		var rows []struct {
			OldPath     string `json:"old_path"`
			NewPath     string `json:"new_path"`
			AMode       string `json:"a_mode"`
			BMode       string `json:"b_mode"`
			NewFile     bool   `json:"new_file"`
			RenamedFile bool   `json:"renamed_file"`
			DeletedFile bool   `json:"deleted_file"`
			TooLarge    bool   `json:"too_large"`
			Collapsed   bool   `json:"collapsed"`
		}
		h, err := p.request(ctx, http.MethodGet, endpoint+"/diffs?per_page=100&page="+strconv.Itoa(page), nil, &rows)
		if err != nil {
			return raw, Review{}, err
		}
		for _, f := range rows {
			if !validPath(f.NewPath) || !validPath(f.OldPath) || seen[f.NewPath] || f.TooLarge {
				return raw, Review{}, failure("provider_diff_incomplete")
			}
			seen[f.NewPath] = true
			status := "modified"
			if f.NewFile {
				status = "added"
			}
			if f.DeletedFile {
				status = "removed"
			}
			if f.RenamedFile {
				status = "renamed"
			}
			r.Changes = append(r.Changes, Change{Path: f.NewPath, PreviousPath: f.OldPath, Status: status, Mode: f.BMode})
		}
		next, err := nextPage(h, page, len(rows), false)
		if err != nil {
			return raw, Review{}, err
		}
		if !next {
			break
		}
	}
	if len(r.Changes) != count {
		return raw, Review{}, failure("provider_diff_incomplete")
	}
	if r.State == "merged" {
		return raw, r, nil
	}
	if r.State != "open" || raw.Draft || raw.WorkInProgress || raw.HasConflicts == nil || *raw.HasConflicts || raw.DetailedMergeStatus != "mergeable" {
		r.BlockReason = "native_merge_checks_pending"
		if keyPattern.MatchString(raw.DetailedMergeStatus) {
			r.BlockReason = "gitlab_" + raw.DetailedMergeStatus
		}
		return raw, r, nil
	}
	if raw.User.CanMerge == nil || !*raw.User.CanMerge {
		r.BlockReason = "native_merge_permission_denied"
		return raw, r, nil
	}
	// detailed_merge_status includes approvals, pipeline policy, discussions,
	// security checks and conflicts. A present pipeline must also be successful;
	// pending/unknown fields must never be interpreted as completed checks.
	if raw.HeadPipeline != nil && (raw.HeadPipeline.Status != "success" || raw.HeadPipeline.SHA != raw.SHA) {
		r.BlockReason = "native_pipeline_pending"
		return raw, r, nil
	}
	r.GatesPassed = true
	return raw, r, nil
}
func (p *gitlabProvider) SyncDecision(ctx context.Context, number int64, in DecisionNote) error {
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	if number <= 0 {
		return failure("invalid_review")
	}
	body, err := decisionBody(in)
	if err != nil {
		return err
	}
	endpoint := p.repo + "/merge_requests/" + strconv.FormatInt(number, 10) + "/notes"
	found := false
	for page := 1; ; page++ {
		var rows []struct{ Body string }
		h, err := p.request(ctx, http.MethodGet, endpoint+"?per_page=100&page="+strconv.Itoa(page), nil, &rows)
		if err != nil {
			return err
		}
		for _, row := range rows {
			if strings.Contains(row.Body, marker(in.Key)) {
				if row.Body != body {
					return failure("decision_note_conflict")
				}
				found = true
			}
		}
		next, err := nextPage(h, page, len(rows), false)
		if err != nil {
			return err
		}
		if !next {
			break
		}
	}
	if found {
		return nil
	}
	_, err = p.request(ctx, http.MethodPost, endpoint, map[string]string{"body": body}, nil)
	return err
}
func (p *gitlabProvider) Merge(ctx context.Context, expected MergeExpectation) (Review, error) {
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	number, head := expected.Number, expected.HeadSHA
	if !shaPattern.MatchString(head) {
		return Review{}, failure("invalid_head")
	}
	r, err := p.Observe(ctx, number)
	if err != nil {
		return Review{}, err
	}
	if err := expected.Check(r); err != nil {
		return r, err
	}
	if r.State == "merged" {
		return r, nil
	}
	if r.State != "open" || !r.GatesPassed {
		return r, failure("native_merge_blocked")
	}
	_, err = p.request(ctx, http.MethodPut, p.repo+"/merge_requests/"+strconv.FormatInt(number, 10)+"/merge", map[string]any{"sha": head, "should_remove_source_branch": false, "squash": true}, nil)
	if err != nil {
		return r, err
	}
	observed, err := p.Observe(ctx, number)
	if err != nil {
		return r, &Error{Code: "merge_not_confirmed", Retryable: true, Uncertain: true}
	}
	if err := expected.Check(observed); err != nil {
		return observed, err
	}
	if observed.State != "merged" {
		return observed, &Error{Code: "merge_not_confirmed", Retryable: true, Uncertain: true}
	}
	return observed, nil
}

var _ Provider = (*gitlabProvider)(nil)
