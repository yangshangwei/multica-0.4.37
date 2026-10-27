package collaborationrun

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

// hostMemory reads the run's shared-context base after its permit and before
// any prompt is assembled. A run whose claim did not declare support (claimed
// before this daemon or server understood shared context) gets nil and runs
// without it. Any other failure keeps the provider unlaunched.
func (r *Runner) hostMemory(ctx context.Context, identity protocol.CollaborationRunIdentity, token string) (*protocol.CollaborationRunMemory, error) {
	attempts, backoff := r.PermitAttempts, r.PermitBackoff
	if attempts <= 0 {
		attempts = defaultPermitAttempts
	}
	if backoff <= 0 {
		backoff = defaultPermitBackoff
	}
	var err error
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			timer := time.NewTimer(backoff << (attempt - 1))
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				return nil, errors.Join(err, ctx.Err())
			}
		}
		var memory protocol.CollaborationRunMemory
		memory, err = r.Client.MemoryBase(ctx, identity, token)
		if err == nil {
			return &memory, nil
		}
		var refusal *HTTPError
		if errors.As(err, &refusal) && refusal.Status == http.StatusConflict && refusal.Code == "memory_unsupported" {
			return nil, nil
		}
		if !errors.Is(err, ErrTransport) && !(errors.As(err, &refusal) &&
			(refusal.Status == http.StatusRequestTimeout || refusal.Status == http.StatusTooManyRequests || refusal.Status >= 500)) {
			return nil, err
		}
	}
	return nil, err
}

// hostPromptMemory is the document a run starts from. It is only part of the
// provider prompt; it is never written to the brief file or the local ledger.
type hostPromptMemory struct {
	Version int64  `json:"version"`
	Content string `json:"content"`
}

func newHostPromptMemory(memory *protocol.CollaborationRunMemory) (*hostPromptMemory, error) {
	if memory == nil {
		return nil, nil
	}
	version, err := strconv.ParseInt(memory.Version, 10, 64)
	if err != nil || version < 0 {
		return nil, ErrProtocol
	}
	return &hostPromptMemory{Version: version, Content: memory.Content}, nil
}

// hostMemoryBrief is fixed platform guidance. Writing memory is expected but
// never required for the run to end; the document text stays out of this file.
func hostMemoryBrief(issueID string) string {
	return "Collaboration memory is this issue's single shared working document: its current state, maintained by every collaborating agent and human. Your input's `memory` field is the version this run starts from; version 0 is an empty document. Read it before acting, and refresh with `multica collaboration memory show " + issueID + "` when another channel may have written since.\n" +
		"Before you finish, rewrite the whole document to the current state (replace stale conclusions and remove finished items; do not append a log) and save it with `multica collaboration memory update " + issueID + " --base-version <version you merged from> --request-id <uuid> --content-stdin` (or `--content-file <file in the working directory>`). Reuse the same request id when retrying an uncertain result.\n" +
		"Suggested sections: Current state, Key decisions and why, Code findings, Pitfalls, Next steps. Keep only text and pointers (commits, branches, issue attachments, file paths in inline code, comment IDs); put full investigation details in a comment. Never write credentials or secrets.\n" +
		"If the update reports stale_memory_version, run `memory show`, merge your changes into the newer version, and update again with that version as --base-version.\n"
}
