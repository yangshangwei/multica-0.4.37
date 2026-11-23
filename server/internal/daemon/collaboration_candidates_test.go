package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/daemon/collaborationrun"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// The fixture proves claim/permit admission without resolving a provider. Start
// models an uncertain launch so production capacity and update fences are tested.
type collaborationCandidateLauncher struct{ starts atomic.Int64 }

func (*collaborationCandidateLauncher) Probe(context.Context) error { return nil }
func (l *collaborationCandidateLauncher) Start(context.Context, collaborationrun.ProviderInput) (*collaborationrun.Process, error) {
	l.starts.Add(1)
	return nil, errors.New("test launch acknowledgement unavailable")
}

func TestCollaborationCandidatePagingAcrossRestart(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("private POSIX ledger requires Linux or macOS")
	}
	for _, mode := range []string{"legacy order", "single candidate pages"} {
		t.Run(mode, func(t *testing.T) {
			workspaceID, runtimeID := uuid.NewString(), uuid.NewString()
			first, second := uuid.NewString(), uuid.NewString()
			candidateIDs := []string{first, second}
			if mode == "single candidate pages" {
				candidateIDs = []string{first}
				for range 16 {
					candidateIDs = append(candidateIDs, uuid.NewString())
				}
				candidateIDs = append(candidateIDs, second)
			}
			refusedClaims := make(map[string]string)
			for _, id := range candidateIDs[:len(candidateIDs)-1] {
				refusedClaims["/api/collaboration/runtime/runs/"+id+"/claim"] = id
			}
			var mu sync.Mutex
			var claims []string
			var identity protocol.CollaborationRunIdentity
			scanPosition := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				if id, refused := refusedClaims[r.URL.Path]; refused {
					claims = append(claims, id)
					w.WriteHeader(http.StatusConflict)
					return
				}
				switch r.URL.Path {
				case "/api/daemon/register":
					_ = json.NewEncoder(w).Encode(RegisterResponse{Runtimes: []Runtime{{ID: runtimeID, Provider: "fake"}}})
				case "/api/collaboration/runtime/runs":
					if r.URL.Query().Get("runtime_id") != runtimeID || r.URL.Query().Has("candidate_metadata") {
						t.Error("candidate scan changed the original runtime-only list contract")
					}
					out := protocol.CollaborationRunListResponse{RunIDs: candidateIDs}
					if mode == "single candidate pages" {
						// The server advances its own keyset on List, independently
						// of the reconstructed poller or whether Claim succeeds.
						// Mirror the candidate query's LIMIT 1: a 16-item page here
						// alternates claims 1/17 forever and never attempts run 18.
						end := min(scanPosition+1, len(candidateIDs))
						out.RunIDs = slices.Clone(candidateIDs[scanPosition:end])
						scanPosition = end % len(candidateIDs)
					}
					_ = json.NewEncoder(w).Encode(out)
				case "/api/collaboration/runtime/runs/" + second + "/claim":
					claims = append(claims, second)
					var request protocol.CollaborationRunClaimRequest
					if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
						t.Error(err)
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					identity = protocol.CollaborationRunIdentity{DeploymentID: request.DeploymentID, InstallationID: request.InstallationID,
						IssueID: uuid.NewString(), SessionID: uuid.NewString(), TargetID: uuid.NewString(), ChannelID: uuid.NewString(),
						BatchID: uuid.NewString(), GroupID: uuid.NewString(), RunID: second, LaunchNonce: uuid.NewString(), Fence: "1", ClaimGeneration: "1"}
					_ = json.NewEncoder(w).Encode(protocol.CollaborationRunClaimResponse{Identity: identity,
						AgentID: runtimeID, ResourceOwnerID: uuid.NewString(), ExecutionWorkspaceID: workspaceID, TerminationToken: "private-candidate-termination",
						Inputs: []protocol.CollaborationRunInput{{CommandID: uuid.NewString(), AuthorUserID: uuid.NewString(), ParticipantEpoch: "1", AccountEpoch: "1", LifecycleEpoch: "1"}}})
				case "/api/collaboration/runtime/runs/" + second + "/permit":
					var request protocol.CollaborationRunPermitRequest
					if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
						t.Error(err)
					}
					_ = json.NewEncoder(w).Encode(protocol.CollaborationRunPermitResponse{Identity: request.Identity, AttemptSeq: request.AttemptSeq, AttemptNonce: request.AttemptNonce, BusinessToken: "mcb_test_candidate"})
				case "/api/collaboration/runtime/runs/" + second + "/stop-intent":
					_ = json.NewEncoder(w).Encode(protocol.CollaborationRunStopIntent{Identity: identity, StopRequested: false})
				default:
					t.Errorf("unexpected candidate recovery action: %s", r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()
			d, _ := collaborationManagedPollerFixture(t, server.URL, workspaceID, runtimeID)
			if _, err := d.client.Register(t.Context(), map[string]any{"workspace_id": workspaceID}); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(d.cfg.CollaborationRunner.LedgerDirectory), 0700); err != nil {
				t.Fatal(err)
			}
			launcher := &collaborationCandidateLauncher{}
			sem, wakeup := newTaskSlotSemaphore(1), make(chan struct{}, 1)
			var wg sync.WaitGroup
			var poller *collaborationPoller
			rounds := 2
			if mode == "single candidate pages" {
				rounds = 2 * len(candidateIDs)
			}
			for range rounds {
				ledger, err := collaborationrun.OpenLedger(d.cfg.CollaborationRunner.LedgerDirectory)
				if err != nil {
					t.Fatal(err)
				}
				poller = &collaborationPoller{daemon: d, config: d.cfg.CollaborationRunner, ledger: ledger, controller: launcher, enabled: true}
				var slot int
				select {
				case slot = <-sem:
				default:
					t.Fatal("a refused claim retained machine capacity")
				}
				if !poller.dispatch(t.Context(), t.Context(), slot, sem, wakeup, &wg) {
					t.Fatal("candidate dispatch did not start")
				}
				wg.Wait()
				if mode == "single candidate pages" && launcher.starts.Load() > 0 {
					break
				}
			}
			mu.Lock()
			actual := slices.Clone(claims)
			mu.Unlock()
			want := candidateIDs
			if mode == "legacy order" {
				want = []string{first, first}
			}
			if !slices.Equal(actual, want) {
				t.Fatalf("server paging skipped candidates or changed legacy order: claims=%v want=%v", actual, want)
			}
			if mode == "legacy order" {
				if launcher.starts.Load() != 0 || len(sem) != 1 || d.activeTasks.Load() != 0 {
					t.Fatal("legacy refusal changed capacity or launched work")
				}
				return
			}
			if launcher.starts.Load() != 1 || len(sem) != 0 || d.activeTasks.Load() != 1 {
				t.Fatal("uncertain successor launch lost the original slot or forked again")
			}
			if _, blocked := poller.blocked.Load(second); !blocked {
				t.Fatal("uncertain successor was not blocked")
			}
			if d.trySetClaimBarrier() {
				t.Fatal("uncertain successor allowed an automatic update")
			}
			retained, err := poller.ledger.Retained()
			if err != nil || len(retained) != 1 || retained[0].Identity.RunID != second || !retained[0].Unconfirmed || retained[0].CanProve {
				t.Fatalf("uncertain successor lost its durable nonce barrier: %+v, %v", retained, err)
			}
		})
	}
}
