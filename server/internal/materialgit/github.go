package materialgit

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"
)

type githubProvider struct{ *client }

type ghCommit struct {
	SHA     string `json:"sha"`
	Message string `json:"message"`
	Tree    struct {
		SHA string `json:"sha"`
	} `json:"tree"`
	Parents []struct {
		SHA string `json:"sha"`
	} `json:"parents"`
}
type ghEntry struct{ Path, Mode, Type, SHA string }
type ghPull struct {
	Number       int64     `json:"number"`
	URL          string    `json:"html_url"`
	Body         string    `json:"body"`
	State        string    `json:"state"`
	Draft        bool      `json:"draft"`
	Merged       bool      `json:"merged"`
	MergeSHA     string    `json:"merge_commit_sha"`
	Mergeable    *bool     `json:"mergeable"`
	MergeState   string    `json:"mergeable_state"`
	ChangedFiles *int      `json:"changed_files"`
	Head         ghPullRef `json:"head"`
	Base         ghPullRef `json:"base"`
	MergedBy     struct {
		Login string `json:"login"`
	} `json:"merged_by"`
}
type ghPullRef struct {
	Ref, SHA string
	Repo     struct {
		FullName string `json:"full_name"`
	} `json:"repo"`
}

func (p *githubProvider) commit(ctx context.Context, sha string) (ghCommit, error) {
	var result ghCommit
	_, err := p.request(ctx, http.MethodGet, p.repo+"/git/commits/"+url.PathEscape(sha), nil, &result)
	if err != nil {
		return result, err
	}
	if !shaPattern.MatchString(result.SHA) || !shaPattern.MatchString(result.Tree.SHA) || result.SHA != sha {
		return result, failure("provider_commit_invalid")
	}
	return result, nil
}
func (p *githubProvider) ref(ctx context.Context, branch string) (string, error) {
	var result struct {
		Ref    string
		Object struct{ Type, SHA string }
	}
	_, err := p.request(ctx, http.MethodGet, p.repo+"/git/ref/heads/"+escapePath(branch), nil, &result)
	if err != nil {
		return "", err
	}
	if result.Ref != "refs/heads/"+branch || result.Object.Type != "commit" || !shaPattern.MatchString(result.Object.SHA) {
		return "", failure("provider_ref_invalid")
	}
	return result.Object.SHA, nil
}
func (p *githubProvider) entry(ctx context.Context, treeSHA, name string) (ghEntry, error) {
	var result struct {
		SHA       string
		Truncated *bool
		Tree      []ghEntry
	}
	_, err := p.request(ctx, http.MethodGet, p.repo+"/git/trees/"+treeSHA, nil, &result)
	if err != nil {
		return ghEntry{}, err
	}
	if result.SHA != treeSHA || result.Truncated == nil || *result.Truncated || result.Tree == nil {
		return ghEntry{}, failure("provider_tree_incomplete")
	}
	var found *ghEntry
	for _, e := range result.Tree {
		if e.Path == name {
			if found != nil {
				return ghEntry{}, failure("provider_tree_invalid")
			}
			v := e
			found = &v
		}
	}
	if found == nil {
		return ghEntry{}, &Error{Code: "provider_tree_entry_missing", Status: 404}
	}
	if !shaPattern.MatchString(found.SHA) {
		return ghEntry{}, failure("provider_tree_invalid")
	}
	return *found, nil
}
func (p *githubProvider) fileEntry(ctx context.Context, commitSHA, name string) (ghEntry, error) {
	commit, err := p.commit(ctx, commitSHA)
	if err != nil {
		return ghEntry{}, err
	}
	tree := commit.Tree.SHA
	parts := strings.Split(name, "/")
	for i, part := range parts {
		entry, err := p.entry(ctx, tree, part)
		if err != nil {
			return ghEntry{}, err
		}
		if i == len(parts)-1 {
			if entry.Type != "blob" || entry.Mode != "100644" {
				return ghEntry{}, failure("material_file_not_regular")
			}
			return entry, nil
		}
		if entry.Type != "tree" || entry.Mode != "040000" {
			return ghEntry{}, failure("material_path_not_directory")
		}
		tree = entry.SHA
	}
	return ghEntry{}, failure("invalid_file_reference")
}
func (p *githubProvider) ReadFile(ctx context.Context, commit, name string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	if err := validateRead(commit, name); err != nil {
		return "", err
	}
	entry, err := p.fileEntry(ctx, commit, name)
	if err != nil {
		return "", err
	}
	var blob struct {
		SHA, Encoding, Content string
		Size                   *int
	}
	_, err = p.request(ctx, http.MethodGet, p.repo+"/git/blobs/"+entry.SHA, nil, &blob)
	if err != nil {
		return "", err
	}
	if blob.SHA != entry.SHA || blob.Encoding != "base64" || blob.Size == nil || *blob.Size > MaxFileBytes || *blob.Size < 0 {
		return "", failure("provider_file_invalid")
	}
	b, err := base64.StdEncoding.DecodeString(blob.Content)
	if err != nil || len(b) != *blob.Size || !utf8.Valid(b) {
		return "", failure("provider_file_incomplete")
	}
	return string(b), nil
}

func (p *githubProvider) Publish(ctx context.Context, in PublishRequest) (Review, error) {
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	if err := validatePublish(in); err != nil {
		return Review{}, err
	}
	q := url.Values{"state": {"all"}, "head": {strings.Split(p.connection.Repository, "/")[0] + ":" + in.Branch}, "per_page": {"100"}}
	var found *ghPull
	for page := 1; ; page++ {
		q.Set("page", strconv.Itoa(page))
		var rows []ghPull
		h, err := p.request(ctx, http.MethodGet, p.repo+"/pulls?"+q.Encode(), nil, &rows)
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
		next, err := nextPage(h, page, len(rows), true)
		if err != nil {
			return Review{}, err
		}
		if !next {
			break
		}
	}
	if found != nil && found.Base.Ref != in.BaseBranch {
		return Review{}, failure("candidate_review_changed")
	}
	head, err := p.ref(ctx, in.Branch)
	if err != nil && !isStatus(err, 404) {
		return Review{}, err
	}
	if isStatus(err, 404) {
		if found != nil {
			return Review{}, failure("candidate_branch_missing")
		}
		base, err := p.ref(ctx, in.BaseBranch)
		if err != nil {
			return Review{}, err
		}
		commit, err := p.commit(ctx, base)
		if err != nil {
			return Review{}, err
		}
		entries := make([]map[string]string, 0, len(in.Files))
		for _, file := range in.Files {
			// Traversal checks every existing ancestor, not just the final blob.
			if _, err := p.fileEntry(ctx, base, file.Path); err != nil && !isMissingTreeEntry(err) {
				return Review{}, err
			}
			entries = append(entries, map[string]string{"path": file.Path, "mode": "100644", "type": "blob", "content": file.Content})
		}
		var tree struct{ SHA string }
		if _, err = p.request(ctx, http.MethodPost, p.repo+"/git/trees", map[string]any{"base_tree": commit.Tree.SHA, "tree": entries}, &tree); err != nil {
			return Review{}, err
		}
		if !shaPattern.MatchString(tree.SHA) {
			return Review{}, failure("provider_tree_invalid")
		}
		var made ghCommit
		if _, err = p.request(ctx, http.MethodPost, p.repo+"/git/commits", map[string]any{"message": in.Title + "\n\n" + candidateMarker(in), "tree": tree.SHA, "parents": []string{base}}, &made); err != nil {
			return Review{}, err
		}
		if !shaPattern.MatchString(made.SHA) {
			return Review{}, failure("provider_commit_invalid")
		}
		// Only create refs. A colliding ref is read and checked, never updated.
		_, err = p.request(ctx, http.MethodPost, p.repo+"/git/refs", map[string]string{"ref": "refs/heads/" + in.Branch, "sha": made.SHA}, nil)
		if err != nil && !isStatus(err, 422) && !isStatus(err, 409) {
			return Review{}, err
		}
		head, err = p.ref(ctx, in.Branch)
		if err != nil {
			return Review{}, err
		}
	}
	commit, err := p.commit(ctx, head)
	if err != nil {
		return Review{}, err
	}
	if !strings.Contains(commit.Message, candidateMarker(in)) {
		return Review{}, failure("candidate_branch_conflict")
	}
	if err = sameFiles(ctx, p, head, in.Files); err != nil {
		return Review{}, err
	}
	if found == nil {
		var created ghPull
		_, err = p.request(ctx, http.MethodPost, p.repo+"/pulls", map[string]any{"title": in.Title, "body": candidateMarker(in) + "\n\n" + in.Body, "head": in.Branch, "base": in.BaseBranch, "maintainer_can_modify": false}, &created)
		if err != nil {
			return Review{}, err
		}
		found = &created
	}
	if found.Number <= 0 {
		return Review{}, failure("provider_review_invalid")
	}
	raw, review, err := p.observe(ctx, found.Number)
	if err != nil {
		return Review{}, err
	}
	if !strings.Contains(raw.Body, candidateMarker(in)) || review.HeadSHA != head || review.Branch != in.Branch || review.BaseBranch != in.BaseBranch {
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

func (p *githubProvider) Observe(ctx context.Context, number int64) (Review, error) {
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	_, r, e := p.observe(ctx, number)
	return r, e
}
func (p *githubProvider) observe(ctx context.Context, number int64) (ghPull, Review, error) {
	var raw ghPull
	if number <= 0 {
		return raw, Review{}, failure("invalid_review")
	}
	endpoint := p.repo + "/pulls/" + strconv.FormatInt(number, 10)
	if _, err := p.request(ctx, http.MethodGet, endpoint, nil, &raw); err != nil {
		return raw, Review{}, err
	}
	if raw.Number != number || !p.validateReviewURL(raw.URL, number) || !shaPattern.MatchString(raw.Head.SHA) || !validRef(raw.Head.Ref) || !validRef(raw.Base.Ref) || raw.Head.Repo.FullName != p.connection.Repository || raw.Base.Repo.FullName != p.connection.Repository || raw.ChangedFiles == nil || *raw.ChangedFiles < 1 || *raw.ChangedFiles > 3000 {
		return raw, Review{}, failure("provider_review_invalid")
	}
	r := Review{Number: number, URL: raw.URL, HeadSHA: raw.Head.SHA, BaseBranch: raw.Base.Ref, Branch: raw.Head.Ref, State: raw.State, MergeSHA: raw.MergeSHA, MergeActor: raw.MergedBy.Login, Changes: []Change{}}
	if raw.Merged {
		r.State = "merged"
		if !shaPattern.MatchString(r.MergeSHA) || r.MergeActor == "" {
			return raw, Review{}, failure("provider_merge_incomplete")
		}
	}
	if r.State != "open" && r.State != "closed" && r.State != "merged" {
		return raw, Review{}, failure("provider_review_invalid")
	}
	seen := map[string]bool{}
	for page := 1; ; page++ {
		var rows []struct {
			Filename, Status string
			PreviousFilename string `json:"previous_filename"`
		}
		h, err := p.request(ctx, http.MethodGet, endpoint+"/files?per_page=100&page="+strconv.Itoa(page), nil, &rows)
		if err != nil {
			return raw, Review{}, err
		}
		for _, f := range rows {
			if !validPath(f.Filename) || seen[f.Filename] {
				return raw, Review{}, failure("provider_diff_invalid")
			}
			seen[f.Filename] = true
			mode := ""
			if f.Status != "removed" {
				entry, err := p.fileEntry(ctx, r.HeadSHA, f.Filename)
				if err != nil {
					return raw, Review{}, err
				}
				mode = entry.Mode
			}
			r.Changes = append(r.Changes, Change{Path: f.Filename, PreviousPath: f.PreviousFilename, Status: f.Status, Mode: mode})
		}
		next, err := nextPage(h, page, len(rows), true)
		if err != nil {
			return raw, Review{}, err
		}
		if !next {
			break
		}
	}
	if len(r.Changes) != *raw.ChangedFiles {
		return raw, Review{}, failure("provider_diff_incomplete")
	}
	if r.State == "merged" {
		return raw, r, nil
	}
	if raw.Draft || raw.Mergeable == nil || !*raw.Mergeable || raw.MergeState != "clean" {
		r.BlockReason = "native_merge_checks_pending"
		return raw, r, nil
	}
	passed, reason := p.gates(ctx, raw)
	r.GatesPassed = passed
	r.BlockReason = reason
	return raw, r, nil
}

// GitHub's ordinary merge endpoint can bypass a protection policy for privileged
// actors. Refuse policies with bypass allowances and require enforcement for
// administrators; unknown rules remain blocked rather than inferred from clean.
func (p *githubProvider) gates(ctx context.Context, pr ghPull) (bool, string) {
	requireReviews, requireChecks := false, false
	var branch struct{ Protected *bool }
	if _, err := p.request(ctx, http.MethodGet, p.repo+"/branches/"+url.PathEscape(pr.Base.Ref), nil, &branch); err != nil || branch.Protected == nil {
		return false, "native_policy_unavailable"
	}
	type branchRule struct {
		Type       string
		RulesetID  int64 `json:"ruleset_id"`
		Parameters json.RawMessage
	}
	var rules []branchRule
	for page := 1; ; page++ {
		var rows []branchRule
		h, err := p.request(ctx, http.MethodGet, p.repo+"/rules/branches/"+url.PathEscape(pr.Base.Ref)+"?per_page=100&page="+strconv.Itoa(page), nil, &rows)
		if err != nil || rows == nil {
			return false, "native_policy_unavailable"
		}
		rules = append(rules, rows...)
		next, err := nextPage(h, page, len(rows), true)
		if err != nil {
			return false, "native_policy_unavailable"
		}
		if !next {
			break
		}
	}
	seen := map[int64]bool{}
	for _, rule := range rules {
		switch rule.Type {
		case "deletion", "non_fast_forward", "pull_request", "required_status_checks", "required_linear_history":
		default:
			return false, "native_policy_unsupported"
		}
		if rule.Type == "pull_request" {
			var params struct {
				Count      *int `json:"required_approving_review_count"`
				CodeOwners bool `json:"require_code_owner_review"`
			}
			if json.Unmarshal(rule.Parameters, &params) != nil || params.Count == nil {
				return false, "native_policy_unavailable"
			}
			requireReviews = requireReviews || *params.Count > 0 || params.CodeOwners
		}
		if rule.Type == "required_status_checks" {
			requireChecks = true
		}
		if rule.RulesetID <= 0 {
			return false, "native_policy_unavailable"
		}
		if seen[rule.RulesetID] {
			continue
		}
		seen[rule.RulesetID] = true
		var rs struct {
			Enforcement  string
			BypassActors json.RawMessage `json:"bypass_actors"`
		}
		if _, err := p.request(ctx, http.MethodGet, p.repo+"/rulesets/"+strconv.FormatInt(rule.RulesetID, 10)+"?includes_parents=true", nil, &rs); err != nil {
			return false, "native_policy_unavailable"
		}
		var bypass []json.RawMessage
		if rs.Enforcement != "active" || len(rs.BypassActors) == 0 || json.Unmarshal(rs.BypassActors, &bypass) != nil || bypass == nil {
			return false, "native_policy_unavailable"
		}
		if len(bypass) != 0 {
			return false, "native_policy_bypass_not_supported"
		}
	}
	if *branch.Protected {
		var protection struct {
			EnforceAdmins *struct{ Enabled bool } `json:"enforce_admins"`
			Checks        json.RawMessage         `json:"required_status_checks"`
			Reviews       *struct {
				Bypass     map[string][]json.RawMessage `json:"bypass_pull_request_allowances"`
				Count      *int                         `json:"required_approving_review_count"`
				CodeOwners bool                         `json:"require_code_owner_reviews"`
			} `json:"required_pull_request_reviews"`
		}
		_, err := p.request(ctx, http.MethodGet, p.repo+"/branches/"+url.PathEscape(pr.Base.Ref)+"/protection", nil, &protection)
		if err != nil {
			// A ruleset alone also sets branch.protected. Its missing classic
			// protection endpoint is not itself proof that no classic rule applies.
			if isStatus(err, http.StatusNotFound) && len(rules) > 0 && p.classicRuleAbsent(ctx, pr.Base.Ref) {
				return p.reviewAndChecks(ctx, pr, requireReviews, requireChecks)
			}
			return false, "native_protection_unavailable"
		}
		if protection.EnforceAdmins == nil || !protection.EnforceAdmins.Enabled {
			return false, "native_policy_bypass_not_supported"
		}
		if protection.Reviews != nil {
			if protection.Reviews.Count == nil {
				return false, "native_policy_unavailable"
			}
			requireReviews = requireReviews || *protection.Reviews.Count > 0 || protection.Reviews.CodeOwners
			for _, actors := range protection.Reviews.Bypass {
				if len(actors) > 0 {
					return false, "native_policy_bypass_not_supported"
				}
			}
		}
		if len(protection.Checks) > 0 && string(protection.Checks) != "null" {
			requireChecks = true
		}
	}
	return p.reviewAndChecks(ctx, pr, requireReviews, requireChecks)
}

const classicRuleQuery = `query($owner:String!,$repo:String!,$ref:String!){
  repository(owner:$owner,name:$repo){ref(qualifiedName:$ref){
    name prefix branchProtectionRule{id}
  }}
}`

func (p *githubProvider) classicRuleAbsent(ctx context.Context, branch string) bool {
	var response struct {
		Errors []json.RawMessage
		Data   struct {
			Repository *struct {
				Ref *struct {
					Name, Prefix string
					Rule         json.RawMessage `json:"branchProtectionRule"`
				}
			}
		}
	}
	parts := strings.Split(p.connection.Repository, "/")
	_, err := p.request(ctx, http.MethodPost, "/graphql", map[string]any{"query": classicRuleQuery, "variables": map[string]any{"owner": parts[0], "repo": parts[1], "ref": "refs/heads/" + branch}}, &response)
	if err != nil || len(response.Errors) > 0 || response.Data.Repository == nil || response.Data.Repository.Ref == nil {
		return false
	}
	ref := response.Data.Repository.Ref
	// Distinguish an explicit null from an omitted field or inaccessible ref.
	return ref.Name == branch && ref.Prefix == "refs/heads/" && strings.TrimSpace(string(ref.Rule)) == "null"
}

const gateQuery = `query($owner:String!,$repo:String!,$number:Int!){
  repository(owner:$owner,name:$repo){pullRequest(number:$number){
    headRefOid baseRefName mergeStateStatus reviewDecision
    commits(last:1){nodes{commit{oid statusCheckRollup{state}}}}
  }}
}`

func (p *githubProvider) reviewAndChecks(ctx context.Context, pr ghPull, requireReviews, requireChecks bool) (bool, string) {
	var response struct {
		Errors []json.RawMessage
		Data   struct {
			Repository *struct {
				PullRequest *struct {
					HeadRefOID     string  `json:"headRefOid"`
					BaseRefName    string  `json:"baseRefName"`
					MergeState     string  `json:"mergeStateStatus"`
					ReviewDecision *string `json:"reviewDecision"`
					Commits        struct {
						Nodes []struct {
							Commit struct {
								OID    string                  `json:"oid"`
								Rollup *struct{ State string } `json:"statusCheckRollup"`
							}
						}
					}
				} `json:"pullRequest"`
			}
		}
	}
	parts := strings.Split(p.connection.Repository, "/")
	_, err := p.request(ctx, http.MethodPost, "/graphql", map[string]any{"query": gateQuery, "variables": map[string]any{"owner": parts[0], "repo": parts[1], "number": pr.Number}}, &response)
	if err != nil || len(response.Errors) > 0 || response.Data.Repository == nil || response.Data.Repository.PullRequest == nil {
		return false, "native_checks_unavailable"
	}
	got := response.Data.Repository.PullRequest
	if got.HeadRefOID != pr.Head.SHA || got.BaseRefName != pr.Base.Ref || got.MergeState != "CLEAN" || len(got.Commits.Nodes) != 1 || got.Commits.Nodes[0].Commit.OID != pr.Head.SHA {
		return false, "native_checks_changed"
	}
	if got.ReviewDecision != nil && *got.ReviewDecision != "APPROVED" {
		return false, "native_approval_pending"
	}
	if requireReviews && (got.ReviewDecision == nil || *got.ReviewDecision != "APPROVED") {
		return false, "native_approval_pending"
	}
	rollup := got.Commits.Nodes[0].Commit.Rollup
	if requireChecks && rollup == nil {
		return false, "native_checks_pending"
	}
	if rollup != nil && rollup.State != "SUCCESS" {
		return false, "native_checks_pending"
	}
	return true, ""
}

func (p *githubProvider) SyncDecision(ctx context.Context, number int64, in DecisionNote) error {
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	if number <= 0 {
		return failure("invalid_review")
	}
	body, err := decisionBody(in)
	if err != nil {
		return err
	}
	endpoint := p.repo + "/issues/" + strconv.FormatInt(number, 10) + "/comments"
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
		next, err := nextPage(h, page, len(rows), true)
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
func (p *githubProvider) Merge(ctx context.Context, expected MergeExpectation) (Review, error) {
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
	var result struct {
		Merged bool
		SHA    string
	}
	_, err = p.request(ctx, http.MethodPut, p.repo+"/pulls/"+strconv.FormatInt(number, 10)+"/merge", map[string]string{"sha": head, "merge_method": "squash"}, &result)
	if err != nil {
		return r, err
	}
	if !result.Merged {
		return r, &Error{Code: "merge_not_confirmed", Retryable: true, Uncertain: true}
	}
	observed, err := p.Observe(ctx, number)
	if err != nil {
		return r, &Error{Code: "merge_not_confirmed", Retryable: true, Uncertain: true}
	}
	if err := expected.Check(observed); err != nil {
		return observed, err
	}
	if observed.State != "merged" || observed.MergeSHA != result.SHA {
		return observed, &Error{Code: "merge_not_confirmed", Retryable: true, Uncertain: true}
	}
	return observed, nil
}

var _ Provider = (*githubProvider)(nil)
