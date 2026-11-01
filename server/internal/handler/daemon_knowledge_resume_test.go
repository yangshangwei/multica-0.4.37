package handler

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// H2: a daemon that cannot reach the claimed native session reports
// resume_unavailable on prepare. The handler's strict decoder must accept it,
// and the fresh session must become the next claim's prior session so the
// stale one stops being chosen on every later input.
func TestKnowledgeUnreachableResumeHTTPRebindsAndBreaksLoop(t *testing.T) {
	for _, surface := range []string{"issue", "chat"} {
		t.Run(surface, func(t *testing.T) {
			f := newTaskKnowledgeHTTPFixture(t, surface)
			old := f.recordNative(t, "stale-native")
			f.nextTask(t)
			task := f.claim(t, "single", &f.inv)
			d := task.KnowledgeContext
			if task.PriorSessionID != "stale-native" || d == nil || d.Session.Generation != old.Session.Generation || d.Session.NativeSessionID != "stale-native" {
				t.Fatalf("fixture did not bind the claim to the stale session: prior=%q", task.PriorSessionID)
			}
			fence := &protocol.TaskKnowledgeFence{ClaimGeneration: d.ClaimGeneration, DispatchedAt: d.DispatchedAt}
			f.call(t, testHandler.StartTask, "/start", map[string]any{"knowledge_fence": fence}, "taskId", f.taskID).Want(200)
			req := protocol.TaskKnowledgePrepareRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ClaimGeneration: d.ClaimGeneration, DispatchedAt: d.DispatchedAt, Session: d.Session, InvocationFingerprint: strings.Repeat("a", 64)}
			raw, err := json.Marshal(req)
			if err != nil {
				t.Fatal(err)
			}
			var body map[string]any
			if err := json.Unmarshal(raw, &body); err != nil {
				t.Fatal(err)
			}
			body["resume_unavailable"] = true
			var p protocol.TaskKnowledgePrepareResponse
			f.call(t, testHandler.PrepareTaskKnowledge, "/prepare", body, "taskId", f.taskID).Want(200).JSON(&p)
			if p.Session.NativeSessionID != "" || p.Session.Generation == old.Session.Generation {
				t.Fatalf("prepare kept the stale binding: %+v", p.Session)
			}
			found := false
			for _, c := range p.Contributions {
				found = found || c.CandidateID == f.proposal.CandidateID
			}
			if !found {
				t.Fatal("fresh session lost the current approved contribution")
			}
			auth := protocol.TaskKnowledgeAuthorizeRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), PrepareID: p.PrepareID, ClaimGeneration: d.ClaimGeneration, DispatchedAt: d.DispatchedAt, Session: p.Session, InvocationFingerprint: p.InvocationFingerprint, ManifestSHA256: p.ManifestSHA256, PromptSHA256: strings.Repeat("b", 64), AttemptSequence: 1, AttemptNonce: uuid.NewString()}
			f.call(t, testHandler.AuthorizeTaskKnowledgeLaunch, "/authorize", auth, "taskId", f.taskID).Want(200)
			for _, kind := range []string{"started", "native_session", "terminated"} {
				observation := protocol.TaskKnowledgeObservation{ProtocolVersion: 1, RequestID: uuid.NewString(), PrepareID: p.PrepareID, ClaimGeneration: d.ClaimGeneration, DispatchedAt: d.DispatchedAt, AttemptSequence: 1, AttemptNonce: auth.AttemptNonce, Kind: kind}
				switch kind {
				case "started":
					observation.ProcessID, observation.ProcessInstanceID = 42, "fresh-process"
				case "native_session":
					observation.NativeSessionID, observation.Source = "fresh-native", "system"
				case "terminated":
					observation.TerminationKind, observation.WaitObserved = "managed_launch_ended", true
				}
				f.call(t, testHandler.ObserveTaskKnowledgeLaunch, "/observations", observation, "taskId", f.taskID).Want(200)
			}
			f.call(t, testHandler.CompleteTask, "/complete", TaskCompleteRequest{Output: "Completed fresh turn", SessionID: "fresh-native", WorkDir: "/tmp/knowledge-fresh", KnowledgeFence: fence}, "taskId", f.taskID).Want(200)
			f.nextTask(t)
			claimed := f.claim(t, "single", &f.inv)
			if claimed.PriorSessionID != "fresh-native" || claimed.KnowledgeContext == nil || claimed.KnowledgeContext.Session.Generation != p.Session.Generation {
				t.Fatalf("the next input still chose the unreachable session: prior=%q", claimed.PriorSessionID)
			}
			if surface == "issue" && claimed.PriorWorkDir != "/tmp/knowledge-fresh" {
				t.Fatalf("the next input lost the fresh workdir: %q", claimed.PriorWorkDir)
			}
		})
	}
}
