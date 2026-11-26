// Package materialgit publishes immutable engineering material revisions through
// narrowly scoped Git provider APIs. It never owns credentials or business approval.
package materialgit

import "context"

type Connection struct {
	Provider, BaseURL, Repository, Token string
}

type File struct {
	Path, Content string
}

type Change struct {
	Path, PreviousPath, Status, Mode string
}

type PublishRequest struct {
	Key, Branch, BaseBranch, Title, Body string
	Files                                []File
}

type Review struct {
	Number                                                                     int64
	URL, HeadSHA, BaseBranch, Branch, State, MergeSHA, MergeActor, BlockReason string
	Changes                                                                    []Change
	GatesPassed                                                                bool
}

type DecisionNote struct {
	Key, ActorID, RevisionID, HeadSHA, Decision, Reason, URL string
}

type MergeExpectation struct {
	Number                      int64
	HeadSHA, Branch, BaseBranch string
	Changes                     []Change
}

func (expected MergeExpectation) Check(review Review) error {
	if expected.Number <= 0 || review.Number != expected.Number || !validRef(expected.Branch) || !validRef(expected.BaseBranch) || expected.BaseBranch == expected.Branch || review.Branch != expected.Branch || review.BaseBranch != expected.BaseBranch {
		return failure("review_target_changed")
	}
	if !shaPattern.MatchString(expected.HeadSHA) || review.HeadSHA != expected.HeadSHA {
		return failure("review_head_changed")
	}
	if len(expected.Changes) < 1 || len(expected.Changes) > 3 || len(review.Changes) != len(expected.Changes) {
		return failure("candidate_files_changed")
	}
	remaining := make(map[string]Change, len(expected.Changes))
	for _, change := range expected.Changes {
		if !materialPath(change.Path) || change.Mode != "100644" || (change.Status != "added" && change.Status != "modified") || (change.PreviousPath != "" && change.PreviousPath != change.Path) {
			return failure("candidate_files_changed")
		}
		remaining[change.Path] = change
	}
	for _, change := range review.Changes {
		if saved, ok := remaining[change.Path]; !ok || saved != change {
			return failure("candidate_files_changed")
		}
		delete(remaining, change.Path)
	}
	return nil
}

type Provider interface {
	Publish(context.Context, PublishRequest) (Review, error)
	Observe(context.Context, int64) (Review, error)
	SyncDecision(context.Context, int64, DecisionNote) error
	Merge(context.Context, MergeExpectation) (Review, error)
	ReadFile(context.Context, string, string) (string, error)
}
