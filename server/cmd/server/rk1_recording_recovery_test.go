//go:build agentintegration && !windows

package main

import (
	"bytes"
	"github.com/multica-ai/multica/server/internal/daemon/collaborationrun"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// The ordinary daemon recovers retained recording receipts on a later poll.
// A slow outcome must not turn that pending seal into an incomplete experiment.
func TestRK1FixtureRecoversRecordingWithoutLaunchingAgain(t *testing.T) {
	rk1RecordingRecovery(t, false)
}

func TestRK1FixtureRecoversLostSealResponseWithoutLaunchingAgain(t *testing.T) {
	rk1RecordingRecovery(t, true)
}

type rk1LostSealTransport struct{ dropped atomic.Bool }

func (x *rk1LostSealTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	response, err := http.DefaultTransport.RoundTrip(r)
	if err == nil && strings.HasSuffix(r.URL.Path, "/recording/seal") && x.dropped.CompareAndSwap(false, true) {
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
		return nil, io.ErrUnexpectedEOF
	}
	return response, err
}

func rk1RecordingRecovery(t *testing.T, loseSeal bool) {
	rk1RequireDatabase(t)
	env := rk1BuildEnv(t, "", rk1FakeVersion, "rk1-requested-model")
	f := newRK1Fixture(t, env, filepath.Join(t.TempDir(), "fixture"))
	f.agentTarget("agent")
	f.submit(f.owner, "agent", "Read this issue and reply HOST_SMOKE_OK."+rk1NoWrites)
	f.control(false)
	run := f.nextRun()
	var delayed atomic.Bool
	loss := &rk1LostSealTransport{}
	if loseSeal {
		var err error
		f.client, err = collaborationrun.NewClient(f.base.server.URL, f.base.binding.DaemonToken, loss)
		if err != nil {
			t.Fatal(err)
		}
	}
	original := f.trace.next
	f.trace.next = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !loseSeal && strings.HasSuffix(r.URL.Path, "/outcome") && delayed.CompareAndSwap(false, true) {
			time.Sleep(1200 * time.Millisecond)
		}
		original.ServeHTTP(w, r)
	})
	runErr, duringErr := f.execute(run, f.plan(rk1Mode{}, "smoke", map[string]string{"answer": "HOST_SMOKE_OK"}), nil)
	if runErr != nil || duringErr != nil {
		t.Fatalf("retained receipt recovery failed: %v / %v", runErr, duringErr)
	}
	o := f.observe(run, runErr)
	if (!loseSeal && !delayed.Load()) || (loseSeal && !loss.dropped.Load()) || len(o.Launches) != 1 || !o.Launches[0].ResultSeen || o.recordingFault() != "" || o.Proof != "host_exit_observed" || o.Progress != "settled" {
		t.Fatalf("recovery lost a single completed launch or its recording: launches=%d recording=%s proof=%s progress=%s", len(o.Launches), o.recordingFault(), o.Proof, o.Progress)
	}
	counts := map[string]int{}
	var permits, seals [][]byte
	var proofSeen bool
	for _, call := range o.Calls {
		if strings.HasSuffix(call.Path, "/recording/seal") {
			seals = append(seals, call.Request)
		}
		if strings.HasSuffix(call.Path, "/permit") {
			if proofSeen {
				t.Fatal("receipt recovery requested a new launch permit")
			}
			permits = append(permits, call.Request)
		}
		if strings.HasSuffix(call.Path, "/proof") {
			proofSeen = true
		}
		for _, suffix := range []string{"claim", "permit", "outcome", "proof", "recording/seal"} {
			if call.Method == http.MethodPost && call.Path == "/api/collaboration/runtime/runs/"+run+"/"+suffix {
				counts[suffix]++
			}
		}
	}
	// Initial authorization and BeforeStart replay the same permit identity.
	if len(permits) != 2 || !bytes.Equal(permits[0], permits[1]) {
		t.Fatal("launch permit replay changed its original identity")
	}
	for _, suffix := range []string{"claim", "outcome", "proof", "recording/seal"} {
		want := 1
		if suffix == "recording/seal" && loseSeal {
			want = 2
		}
		if counts[suffix] != want {
			t.Errorf("%s requests=%d, want %d original operation attempts", suffix, counts[suffix], want)
		}
	}
	if loseSeal && (len(seals) != 2 || !bytes.Equal(seals[0], seals[1])) {
		t.Fatal("seal recovery changed original receipt bytes")
	}
}
