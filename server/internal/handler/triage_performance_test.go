package handler

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

// Opt-in fixture scale comes from TRI-FR-04/TRI-AC performance baselines. This
// records local handler+PostgreSQL latency; it does not claim browser/network P95.
func TestTriageQueueScaleBaseline(t *testing.T) {
	if os.Getenv("MULTICA_TRIAGE_PERF") != "1" {
		t.Skip("set MULTICA_TRIAGE_PERF=1 to measure 10k formal/2k pending")
	}
	ws := dbfx.Workspace(t, "Triage scale baseline", "triage-scale-"+uuidToString(dbid.NewV7()), testutil.Cols{"issue_prefix": "TSB"})
	dbfx.Member(t, ws, testUserID, "owner")
	dbfx.Cleanup(t, `DELETE FROM issue WHERE workspace_id=$1`, ws)
	dbfx.Cleanup(t, `DELETE FROM triage_notification WHERE workspace_id=$1`, ws)
	dbfx.Cleanup(t, `DELETE FROM triage_action WHERE workspace_id=$1`, ws)
	dbfx.Cleanup(t, `DELETE FROM issue_triage WHERE workspace_id=$1`, ws)
	dbfx.InsertNoID(t, "workspace_triage_settings", testutil.Cols{"workspace_id": ws, "enabled": true}, "workspace_id=$1", ws)
	// A set-based fixture avoids timing12000 API creates in a queue benchmark.
	// Fixture-owned cleanup remains scoped to this newly created workspace.
	dbfx.Exec(t, `INSERT INTO issue(id,workspace_id,number,title,status,priority,creator_type,creator_id,admission_status)
 SELECT gen_random_uuid(),$1,n,'Triage baseline '||n,CASE WHEN n<=10000 THEN 'todo' ELSE 'backlog' END,CASE WHEN n%5=0 THEN 'high' ELSE 'none' END,'member',$2,CASE WHEN n<=10000 THEN 'not_required' ELSE 'pending' END FROM generate_series(1,12000) n`, ws, testUserID)
	dbfx.Exec(t, `UPDATE workspace SET issue_counter=12000 WHERE id=$1`, ws)
	dbfx.Exec(t, `INSERT INTO issue_triage(issue_id,workspace_id,entered_at) SELECT id,workspace_id,now()-(number*interval '1 second') FROM issue WHERE workspace_id=$1 AND admission_status='pending'`, ws)
	samples := map[string][]float64{"list": {}, "filter": {}, "action": {}}
	var selection []TriageItem
	for name, path := range map[string]string{"list": "/api/triage/items?limit=50", "filter": "/api/triage/items?limit=50&priority=high&q=Triage&source=manual"} {
		for range 10 {
			req := newRequest("GET", path, nil)
			req.Header.Set("X-Workspace-ID", ws)
			start := time.Now()
			var out struct {
				Items []TriageItem `json:"items"`
				Total int          `json:"total"`
			}
			testutil.Call(t, testHandler.ListTriageItems, req).Want(200).JSON(&out)
			samples[name] = append(samples[name], float64(time.Since(start).Microseconds())/1000)
			expected := 2000
			if name == "filter" {
				expected = 400
			}
			if out.Total != expected || len(out.Items) != 50 {
				t.Fatalf("%s baseline returned total%d/page%d", name, out.Total, len(out.Items))
			}
			if name == "list" {
				selection = out.Items
			}
		}
	}
	for _, item := range selection[:10] {
		req := withURLParam(newRequest("POST", "/api/triage/items/actions", map[string]any{"request_id": uuidToString(dbid.NewV7()), "expected_revision": item.Issue.Revision, "action": "accept"}), "id", item.Issue.ID)
		req.Header.Set("X-Workspace-ID", ws)
		start := time.Now()
		testutil.Call(t, testHandler.ActOnTriageItem, req).Want(200)
		samples["action"] = append(samples["action"], float64(time.Since(start).Microseconds())/1000)
	}
	p95 := map[string]float64{}
	for name, values := range samples {
		sorted := append([]float64(nil), values...)
		sort.Float64s(sorted)
		p95[name] = sorted[int(math.Ceil(float64(len(sorted))*.95))-1]
		if p95[name] > 2000 {
			t.Errorf("local%s P95 %.3fms exceeds2000ms budget", name, p95[name])
		}
	}
	report := map[string]any{"formal_issues": 10000, "pending_issues": 2000, "page_size": 50, "samples_per_operation": 10, "units": "milliseconds", "scope": "local Go handler plus isolated PostgreSQL; excludes browser and HTTP network", "p95_ms": p95, "samples_ms": samples, "measured_at": time.Now().UTC().Format(time.RFC3339)}
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(string(raw))
	if path := os.Getenv("TRIAGE_PERF_REPORT"); path != "" {
		if err = os.WriteFile(path, append(raw, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Log(fmt.Sprintf("Triage local P95 list=%.3fms filter=%.3fms action=%.3fms", p95["list"], p95["filter"], p95["action"]))
}
