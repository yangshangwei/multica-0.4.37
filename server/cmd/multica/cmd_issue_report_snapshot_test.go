package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/cli"
	"github.com/multica-ai/multica/server/internal/reportsnapshot"
	"github.com/spf13/cobra"
)

func TestIssueReportSnapshotCLICommandContract(t *testing.T) {
	command := commandChild(issueCmd, "report-snapshot")
	if command == nil {
		t.Fatal("issue report-snapshot command is missing")
	}
	for _, name := range []string{"capture", "show", "withdraw"} {
		if commandChild(command, name) == nil {
			t.Fatalf("issue report-snapshot %s is missing", name)
		}
	}
	capture := commandChild(command, "capture")
	for _, name := range []string{"request-key", "expected-revision", "timezone", "start", "end", "period-mode", "correction-of", "output"} {
		if capture.Flags().Lookup(name) == nil {
			t.Errorf("capture missing --%s", name)
		}
	}
	show := commandChild(command, "show")
	if show.Flags().Lookup("snapshot") == nil || show.Flags().Lookup("output") == nil {
		t.Fatal("show requires snapshot selector and output flags")
	}
	if show.Flags().Lookup("request-key") != nil || show.Flags().Lookup("refresh") != nil {
		t.Fatal("show must not expose a capture or refresh action")
	}
	withdraw := commandChild(command, "withdraw")
	for _, flag := range []string{"request-key", "expected-revision", "current-snapshot", "reason", "accept", "output"} {
		if withdraw.Flags().Lookup(flag) == nil {
			t.Errorf("withdraw missing --%s", flag)
		}
	}
}

const (
	reportSnapshotTestReport    = "10000000-0000-4000-8000-000000000001"
	reportSnapshotTestSnapshot  = "10000000-0000-4000-8000-000000000002"
	reportSnapshotTestWorkspace = "10000000-0000-4000-8000-000000000003"
	reportSnapshotTestUser      = "10000000-0000-4000-8000-000000000004"
	reportSnapshotTestKey       = "10000000-0000-4000-8000-000000000005"
	reportSnapshotTestOther     = "10000000-0000-4000-8000-000000000006"
	reportSnapshotTestStart     = "2026-11-11T00:00:00.000000Z"
	reportSnapshotTestEnd       = "2026-11-12T00:00:00.000000Z"
)

func reportSnapshotTestTime(value string) time.Time {
	t, err := time.Parse(reportSnapshotTimeFormat, value)
	if err != nil {
		panic(err)
	}
	return t
}

func reportSnapshotTestInput() reportsnapshot.Input {
	return reportsnapshot.Input{
		Identity: reportsnapshot.Identity{SnapshotID: reportSnapshotTestSnapshot, WorkspaceID: reportSnapshotTestWorkspace, ReportIssueID: reportSnapshotTestReport, ReportRevision: 7, Sequence: 1, CreatedBy: reportSnapshotTestUser, RequestKey: reportSnapshotTestKey},
		Window:   reportsnapshot.Window{Start: reportSnapshotTestTime(reportSnapshotTestStart), End: reportSnapshotTestTime(reportSnapshotTestEnd), Timezone: "UTC", PeriodMode: "rolling_24h"},
		AsOf:     reportSnapshotTestTime("2026-11-13T00:00:00.000000Z"), ReadCut: "test-cut-1",
		Facts: reportsnapshot.Facts{
			Issues:     []reportsnapshot.IssueFact{{ID: reportSnapshotTestReport, Revision: 7, Status: "todo", AdmissionStatus: "not_required", CreatedAt: reportSnapshotTestTime("2026-11-10T00:00:00.000000Z"), OriginType: "quick_create", Classification: "report", ClassificationReason: "current_report"}},
			Activities: []reportsnapshot.ActivityFact{},
			Sources:    []reportsnapshot.SourceFact{},
			Gaps:       []reportsnapshot.Gap{},
		},
	}
}

func reportSnapshotTestBody(t *testing.T, input reportsnapshot.Input, replayed bool) []byte {
	t.Helper()
	return reportSnapshotPublicTestBody(t, input, replayed, false)
}

func reportSnapshotPublicTestBody(t *testing.T, input reportsnapshot.Input, replayed, capture bool) []byte {
	t.Helper()
	internal, err := reportsnapshot.Build(input)
	if err != nil {
		t.Fatalf("build internal fixture: %v", err)
	}
	public, err := reportsnapshot.ProjectPublic(internal)
	if err != nil {
		t.Fatalf("project public fixture: %v", err)
	}
	body := map[string]any{"protocol_version": 1, "snapshot": public, "replayed": replayed}
	if capture {
		// This fixture spells out the server's typed commitment independently of
		// the CLI helper, including canonical time.Time JSON rather than flag text.
		intent := map[string]any{
			"report_issue_id": input.ReportIssueID, "request_key": input.RequestKey,
			"request": map[string]any{"protocol_version": 1, "expected_issue_revision": input.ReportRevision,
				"timezone": input.Window.Timezone, "start": input.Window.Start, "end": input.Window.End,
				"period_mode": input.Window.PeriodMode, "correction_of": input.CorrectionOf},
		}
		canonical, err := reportsnapshot.CanonicalJSON(intent)
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(canonical)
		body["capture_receipt"] = map[string]any{"request_key": input.RequestKey, "report_issue_id": input.ReportIssueID,
			"snapshot_id": input.SnapshotID, "request_digest": hex.EncodeToString(digest[:])}
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestIssueReportSnapshotPublicProjectionAndHumanReceipt(t *testing.T) {
	for _, action := range []string{"show", "capture"} {
		t.Run(action, func(t *testing.T) {
			input := reportSnapshotTestInput()
			raw := reportSnapshotPublicTestBody(t, input, false, action == "capture")
			requests := 0
			reportSnapshotFake(t, func(w http.ResponseWriter, r *http.Request) {
				requests++
				if action == "capture" {
					w.WriteHeader(http.StatusCreated)
				}
				_, _ = w.Write(raw)
			})
			args := []string{"show", reportSnapshotTestReport}
			if action == "capture" {
				args = reportSnapshotCaptureArgs()
			}
			out, err := runReportSnapshotCLI(t, args...)
			if err != nil || out == "" || requests != 1 {
				t.Fatalf("public %s output=%q err=%v requests=%d", action, out, err, requests)
			}
			if strings.Contains(out, `"created_by"`) || strings.Contains(out, `"read_cut"`) || strings.Contains(out, `"snapshot_digest"`) {
				t.Fatal("CLI returned internal snapshot fields")
			}
			if action == "show" && strings.Contains(out, `"request_key"`) {
				t.Fatal("GET exposed the human request key")
			}
		})
	}
}

func reportSnapshotFake(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	t.Setenv("MULTICA_SERVER_URL", server.URL)
	t.Setenv("MULTICA_WORKSPACE_ID", reportSnapshotTestWorkspace)
	t.Setenv("MULTICA_TOKEN", "mul_fake_human")
	t.Setenv("MULTICA_AGENT_ID", "")
	t.Setenv("MULTICA_TASK_ID", "")
	t.Setenv("MULTICA_DAEMON_PORT", "")
	t.Setenv("MULTICA_TASK_CONFIG_ROOT", "")
	return server
}

func runReportSnapshotCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := &cobra.Command{Use: "multica", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().String("profile", "", "")
	root.PersistentFlags().String("server-url", "", "")
	root.PersistentFlags().String("workspace-id", "", "")
	issue := &cobra.Command{Use: "issue"}
	issue.AddCommand(newIssueReportSnapshotCommand())
	root.AddCommand(issue)
	output := &bytes.Buffer{}
	root.SetOut(output)
	root.SetErr(io.Discard)
	root.SetArgs(append([]string{"issue", "report-snapshot"}, args...))
	err := root.Execute()
	return output.String(), err
}

func reportSnapshotCaptureArgs() []string {
	return []string{"capture", reportSnapshotTestReport, "--request-key", reportSnapshotTestKey, "--expected-revision", "7", "--timezone", "UTC", "--start", reportSnapshotTestStart, "--end", reportSnapshotTestEnd, "--period-mode", "rolling_24h"}
}

func reportSnapshotReplaceFlag(args []string, flag, value string) []string {
	out := append([]string{}, args...)
	for i := range out {
		if out[i] == flag {
			out[i+1] = value
			return out
		}
	}
	return append(out, flag, value)
}

func TestIssueReportSnapshotCaptureAndExplicitReplay(t *testing.T) {
	posts := 0
	initialBody := reportSnapshotPublicTestBody(t, reportSnapshotTestInput(), false, true)
	replayBody := reportSnapshotPublicTestBody(t, reportSnapshotTestInput(), true, true)
	reportSnapshotFake(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/issues/"+reportSnapshotTestReport+"/report-snapshots" || r.URL.RawQuery != "" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		}
		posts++
		if r.Header.Get("Idempotency-Key") != reportSnapshotTestKey || r.Header.Get("X-Workspace-ID") != reportSnapshotTestWorkspace {
			t.Error("capture lost original key/workspace")
		}
		var request map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		if len(request) != 7 || string(request["protocol_version"]) != "1" || string(request["expected_issue_revision"]) != "7" || string(request["correction_of"]) != "null" {
			t.Errorf("capture request fields: %v", request)
		}
		status := http.StatusCreated
		if posts > 1 {
			status = http.StatusOK
		}
		w.WriteHeader(status)
		body := initialBody
		if posts > 1 {
			body = replayBody
		}
		_, _ = w.Write(body)
	})
	first, err := runReportSnapshotCLI(t, reportSnapshotCaptureArgs()...)
	if err != nil {
		t.Fatal(err)
	}
	var initial reportSnapshotEnvelope
	if err := json.Unmarshal([]byte(first), &initial); err != nil {
		t.Fatal(err)
	}
	if initial.Replayed || initial.Snapshot.SnapshotID != reportSnapshotTestSnapshot || posts != 1 {
		t.Fatalf("capture = %s, posts=%d", first, posts)
	}
	second, err := runReportSnapshotCLI(t, reportSnapshotCaptureArgs()...)
	if err != nil {
		t.Fatal(err)
	}
	var replay reportSnapshotEnvelope
	if err := json.Unmarshal([]byte(second), &replay); err != nil {
		t.Fatal(err)
	}
	if !replay.Replayed || replay.Snapshot.ProjectionDigest != initial.Snapshot.ProjectionDigest || posts != 2 {
		t.Fatalf("explicit original-intent replay = %s, posts=%d", second, posts)
	}
}

func TestIssueReportSnapshotReporterReadsOnlyExactSnapshot(t *testing.T) {
	paths := []string{}
	body := reportSnapshotTestBody(t, reportSnapshotTestInput(), false)
	reportSnapshotFake(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.RequestURI())
		if r.Method != http.MethodGet || r.URL.RawQuery != "" || r.Header.Get("Authorization") != "Bearer mat_test_reporter" || r.Header.Get("Idempotency-Key") != "" {
			t.Errorf("unexpected Reporter request: %s %s", r.Method, r.URL)
		}
		_, _ = w.Write(body)
	})
	t.Setenv("MULTICA_TOKEN", "mat_test_reporter")
	t.Setenv("MULTICA_AGENT_ID", reportSnapshotTestOther)
	t.Setenv("MULTICA_TASK_ID", reportSnapshotTestKey)
	for _, args := range [][]string{
		{"show", reportSnapshotTestReport},
		{"show", reportSnapshotTestReport, "--snapshot", reportSnapshotTestSnapshot, "--output", "table"},
	} {
		output, err := runReportSnapshotCLI(t, args...)
		if err != nil || !strings.Contains(output, reportSnapshotTestSnapshot) {
			t.Fatalf("show=%q err=%v", output, err)
		}
	}
	prefix := "/api/issues/" + reportSnapshotTestReport + "/report-snapshots/"
	if len(paths) != 2 || paths[0] != prefix+"current" || paths[1] != prefix+reportSnapshotTestSnapshot {
		t.Fatalf("only exact GETs expected, got %v", paths)
	}
}

func TestIssueReportSnapshotInvalidInputsNeverRequest(t *testing.T) {
	requests := 0
	reportSnapshotFake(t, func(w http.ResponseWriter, _ *http.Request) { requests++; w.WriteHeader(500) })
	base := reportSnapshotCaptureArgs()
	tests := map[string][]string{
		"identifier":             {"show", "MUL-1"},
		"zero report":            {"show", "00000000-0000-0000-0000-000000000000"},
		"wrong snapshot":         {"show", reportSnapshotTestReport, "--snapshot", "latest"},
		"missing key":            reportSnapshotReplaceFlag(base, "--request-key", ""),
		"zero key":               reportSnapshotReplaceFlag(base, "--request-key", "00000000-0000-0000-0000-000000000000"),
		"bad revision":           reportSnapshotReplaceFlag(base, "--expected-revision", "0"),
		"unknown timezone":       reportSnapshotReplaceFlag(base, "--timezone", "Local"),
		"unknown period":         reportSnapshotReplaceFlag(base, "--period-mode", "daily"),
		"offset":                 reportSnapshotReplaceFlag(base, "--start", "2026-11-11T00:00:00.000000+00:00"),
		"nanoseconds":            reportSnapshotReplaceFlag(base, "--start", "2026-11-11T00:00:00.0000001Z"),
		"wrong rolling duration": reportSnapshotReplaceFlag(base, "--start", "2026-11-11T01:00:00.000000Z"),
		"empty interval":         reportSnapshotReplaceFlag(base, "--start", reportSnapshotTestEnd),
		"invalid correction":     reportSnapshotReplaceFlag(base, "--correction-of", "other"),
		"output":                 reportSnapshotReplaceFlag(base, "--output", "yaml"),
	}
	for name, args := range tests {
		t.Run(name, func(t *testing.T) {
			output, err := runReportSnapshotCLI(t, args...)
			if err == nil || output != "" {
				t.Fatalf("invalid args accepted output=%q err=%v", output, err)
			}
		})
	}
	if requests != 0 {
		t.Fatalf("invalid inputs made %d requests", requests)
	}
}

func TestIssueReportSnapshotCaptureRejectsTaskCredentialsLocally(t *testing.T) {
	requests := 0
	reportSnapshotFake(t, func(w http.ResponseWriter, _ *http.Request) { requests++; w.WriteHeader(500) })
	for _, token := range []string{"mat_reporter", "mdt_daemon"} {
		t.Setenv("MULTICA_TOKEN", token)
		output, err := runReportSnapshotCLI(t, reportSnapshotCaptureArgs()...)
		if err == nil || output != "" || requests != 0 {
			t.Fatalf("task capture: %q %v requests=%d", output, err, requests)
		}
	}
}

func TestIssueReportSnapshotCloudPATWritesStayLocal(t *testing.T) {
	for _, source := range []string{"environment", "profile"} {
		for _, action := range []string{"capture", "withdraw"} {
			t.Run(source+"/"+action, func(t *testing.T) {
				requests := 0
				server := reportSnapshotFake(t, func(w http.ResponseWriter, _ *http.Request) { requests++; w.WriteHeader(403) })
				t.Setenv("HOME", t.TempDir())
				token := auth.CloudPATPrefix + "fake_cloud_node"
				args := reportSnapshotCaptureArgs()
				if action == "withdraw" {
					args = reportSnapshotWithdrawArgs()
				}
				if source == "profile" {
					for _, key := range []string{"MULTICA_TOKEN", "MULTICA_SERVER_URL", "MULTICA_WORKSPACE_ID"} {
						t.Setenv(key, "")
					}
					if err := cli.SaveCLIConfigForProfile(cli.CLIConfig{Token: token, ServerURL: server.URL, WorkspaceID: reportSnapshotTestWorkspace}, "f2-cloud-node"); err != nil {
						t.Fatal(err)
					}
					args = append(args, "--profile", "f2-cloud-node")
				} else {
					t.Setenv("MULTICA_TOKEN", token)
				}
				output, err := runReportSnapshotCLI(t, args...)
				if err == nil || output != "" || requests != 0 || !strings.Contains(err.Error(), "human credential") {
					t.Fatalf("Cloud PAT %s output=%q err=%v requests=%d", action, output, err, requests)
				}
			})
		}
	}
}

func TestIssueReportSnapshotCloudPATShowPreservesReadTransport(t *testing.T) {
	requests := 0
	body := reportSnapshotTestBody(t, reportSnapshotTestInput(), false)
	server := reportSnapshotFake(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodGet || r.Header.Get("Authorization") != "Bearer "+auth.CloudPATPrefix+"fake_cloud_node" {
			t.Errorf("unexpected read %s %s", r.Method, r.URL)
		}
		_, _ = w.Write(body)
	})
	t.Setenv("HOME", t.TempDir())
	for _, key := range []string{"MULTICA_TOKEN", "MULTICA_SERVER_URL", "MULTICA_WORKSPACE_ID"} {
		t.Setenv(key, "")
	}
	if err := cli.SaveCLIConfigForProfile(cli.CLIConfig{Token: auth.CloudPATPrefix + "fake_cloud_node", ServerURL: server.URL, WorkspaceID: reportSnapshotTestWorkspace}, "f2-cloud-node"); err != nil {
		t.Fatal(err)
	}
	output, err := runReportSnapshotCLI(t, "show", reportSnapshotTestReport, "--profile", "f2-cloud-node")
	if err != nil || output == "" || requests != 1 {
		t.Fatalf("read transport changed output=%q err=%v requests=%d", output, err, requests)
	}
}

func TestIssueReportSnapshotCaptureResponseLossNeverRetries(t *testing.T) {
	requests := 0
	reportSnapshotFake(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		connection, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		_ = connection.Close()
	})
	output, err := runReportSnapshotCLI(t, reportSnapshotCaptureArgs()...)
	if err == nil || output != "" || requests != 1 || !strings.Contains(err.Error(), "original --request-key") {
		t.Fatalf("uncertain capture output=%q err=%v requests=%d", output, err, requests)
	}
}

func TestIssueReportSnapshotDoesNotFollowRedirects(t *testing.T) {
	for _, action := range []string{"capture", "show", "withdraw"} {
		t.Run(action, func(t *testing.T) {
			targetHits := 0
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { targetHits++; w.WriteHeader(200) }))
			defer target.Close()
			requests := 0
			reportSnapshotFake(t, func(w http.ResponseWriter, r *http.Request) {
				requests++
				http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
			})
			args := reportSnapshotCaptureArgs()
			if action == "show" {
				args = []string{"show", reportSnapshotTestReport}
			} else if action == "withdraw" {
				args = reportSnapshotWithdrawArgs()
			}
			output, err := runReportSnapshotCLI(t, args...)
			if err == nil || output != "" || requests != 1 || targetHits != 0 {
				t.Fatalf("redirect output=%q err=%v source=%d target=%d", output, err, requests, targetHits)
			}
		})
	}
}

func TestIssueReportSnapshotMalformedEnvelopeHasNoOutput(t *testing.T) {
	valid := reportSnapshotTestBody(t, reportSnapshotTestInput(), false)
	var base map[string]json.RawMessage
	if err := json.Unmarshal(valid, &base); err != nil {
		t.Fatal(err)
	}
	without := func(field string) []byte {
		fields := map[string]json.RawMessage{}
		for key, value := range base {
			if key != field {
				fields[key] = value
			}
		}
		raw, err := json.Marshal(fields)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	tests := map[string][]byte{
		"missing replayed":     without("replayed"),
		"missing protocol":     without("protocol_version"),
		"missing snapshot":     without("snapshot"),
		"null replayed":        bytes.Replace(valid, []byte(`"replayed":false`), []byte(`"replayed":null`), 1),
		"duplicate replayed":   append([]byte(`{"replayed":true,`), valid[1:]...),
		"case alias":           bytes.Replace(valid, []byte(`"replayed"`), []byte(`"Replayed"`), 1),
		"unsupported envelope": bytes.Replace(valid, []byte(`"protocol_version":1`), []byte(`"protocol_version":2`), 1),
		"trailing":             append(append([]byte{}, valid...), []byte(` {}`)...),
		"truncated":            valid[:len(valid)-1],
		"snapshot tamper":      bytes.Replace(valid, []byte(`"report_revision_at_capture":7`), []byte(`"report_revision_at_capture":8`), 1),
	}
	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			requests := 0
			reportSnapshotFake(t, func(w http.ResponseWriter, _ *http.Request) { requests++; _, _ = w.Write(raw) })
			output, err := runReportSnapshotCLI(t, "show", reportSnapshotTestReport)
			if err == nil || output != "" || requests != 1 {
				t.Fatalf("malformed response output=%q err=%v requests=%d", output, err, requests)
			}
		})
	}
}

func TestIssueReportSnapshotBindsSuccessfulResponseToIntent(t *testing.T) {
	for _, test := range []struct {
		name    string
		capture bool
		mutate  func(*reportsnapshot.Input)
		replay  bool
		status  int
	}{
		{name: "show wrong snapshot", mutate: func(in *reportsnapshot.Input) { in.SnapshotID = reportSnapshotTestOther }, status: 200},
		{name: "show wrong report", mutate: func(in *reportsnapshot.Input) {
			in.ReportIssueID = reportSnapshotTestOther
			in.Facts.Issues[0].ID = reportSnapshotTestOther
		}, status: 200},
		{name: "show wrong workspace", mutate: func(in *reportsnapshot.Input) { in.WorkspaceID = reportSnapshotTestOther }, status: 200},
		{name: "show replayed", replay: true, status: 200},
		{name: "capture wrong request", capture: true, mutate: func(in *reportsnapshot.Input) { in.RequestKey = reportSnapshotTestOther }, status: 201},
		{name: "capture wrong revision", capture: true, mutate: func(in *reportsnapshot.Input) { in.ReportRevision = 8; in.Facts.Issues[0].Revision = 8 }, status: 201},
		{name: "capture wrong window", capture: true, mutate: func(in *reportsnapshot.Input) {
			in.Window.Start = in.Window.Start.Add(-time.Hour)
			in.Window.End = in.Window.End.Add(-time.Hour)
		}, status: 201},
		{name: "capture wrong correction", capture: true, mutate: func(in *reportsnapshot.Input) {
			correction := reportSnapshotTestOther
			in.CorrectionOf = &correction
		}, status: 201},
		{name: "capture false replay status", capture: true, status: 200},
		{name: "capture created replay status", capture: true, status: 201, replay: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := reportSnapshotTestInput()
			if test.mutate != nil {
				test.mutate(&input)
			}
			raw := reportSnapshotPublicTestBody(t, input, test.replay, test.capture)
			requests := 0
			reportSnapshotFake(t, func(w http.ResponseWriter, _ *http.Request) {
				requests++
				w.WriteHeader(test.status)
				_, _ = w.Write(raw)
			})
			args := []string{"show", reportSnapshotTestReport, "--snapshot", reportSnapshotTestSnapshot}
			if test.capture {
				args = reportSnapshotCaptureArgs()
			}
			output, err := runReportSnapshotCLI(t, args...)
			if err == nil || output != "" || requests != 1 {
				t.Fatalf("mismatched response output=%q err=%v requests=%d", output, err, requests)
			}
		})
	}
}

func TestIssueReportSnapshotMissingOrRefusedReadDoesNotGenerate(t *testing.T) {
	for _, status := range []int{204, 401, 403, 404, 409, 503} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			requests := 0
			reportSnapshotFake(t, func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.Method != http.MethodGet {
					t.Errorf("unexpected write %s", r.Method)
				}
				w.WriteHeader(status)
			})
			output, err := runReportSnapshotCLI(t, "show", reportSnapshotTestReport)
			if err == nil || output != "" || requests != 1 {
				t.Fatalf("refused read output=%q err=%v requests=%d", output, err, requests)
			}
		})
	}
}

func TestIssueReportSnapshotShowPreservesUnknownMetrics(t *testing.T) {
	input := reportSnapshotTestInput()
	project := reportSnapshotTestOther
	input.ProjectID = &project
	input.Facts.Issues[0].ProjectID = &project
	input.Facts.Health = &reportsnapshot.HealthFacts{Project: reportsnapshot.ProjectFact{WorkspaceID: input.WorkspaceID, ID: project, Revision: 1, Status: "in_progress"}}
	input.Facts.Issues = append(input.Facts.Issues, reportsnapshot.IssueFact{
		ID: "10000000-0000-4000-8000-000000000007", Revision: 1, ProjectID: &project,
		Status: "todo", AdmissionStatus: "not_required", CreatedAt: input.Window.Start,
		Classification: "unknown", ClassificationReason: "legacy_report_provenance_unknown",
	})
	count := int64(2)
	for _, kind := range []string{"issues", "status_catalog", "references", "project", "updates", "activities"} {
		input.Facts.Sources = append(input.Facts.Sources, reportsnapshot.SourceFact{Kind: kind, Version: "fixture-v1", Ordering: "id", ReadComplete: true, ProducerComplete: true})
	}
	input.Facts.Sources[0].Count = &count
	body := reportSnapshotTestBody(t, input, false)
	reportSnapshotFake(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(body) })
	output, err := runReportSnapshotCLI(t, "show", reportSnapshotTestReport)
	if err != nil {
		t.Fatal(err)
	}
	var out reportSnapshotEnvelope
	if err := json.Unmarshal([]byte(output), &out); err != nil {
		t.Fatal(err)
	}
	if out.Snapshot.Complete {
		t.Fatal("unknown business classification became complete")
	}
	for _, metric := range out.Snapshot.Metrics {
		if metric.ID == "stock.total" {
			if string(metric.Value) != "null" || metric.Quality == "exact" || metric.Coverage.Unknown.Count == nil || *metric.Coverage.Unknown.Count != 1 {
				t.Fatalf("unknown business metric changed: %+v", metric)
			}
			return
		}
	}
	t.Fatal("missing stock.total metric")
}

func TestIssueReportSnapshotResponseSizeIsBounded(t *testing.T) {
	requests := 0
	reportSnapshotFake(t, func(w http.ResponseWriter, _ *http.Request) {
		requests++
		_, _ = io.CopyN(w, strings.NewReader(strings.Repeat(" ", reportsnapshot.MaxSnapshotBytes+2049)), reportsnapshot.MaxSnapshotBytes+2049)
	})
	output, err := runReportSnapshotCLI(t, "show", reportSnapshotTestReport)
	if err == nil || output != "" || requests != 1 || !strings.Contains(err.Error(), "size") {
		t.Fatalf("oversized response output=%q err=%v requests=%d", output, err, requests)
	}
}

func reportSnapshotWithdrawArgs() []string {
	return []string{"withdraw", reportSnapshotTestReport, "--request-key", reportSnapshotTestKey, "--expected-revision", "7", "--current-snapshot", reportSnapshotTestSnapshot, "--reason", "Withdraw this report content", "--accept"}
}

func reportSnapshotWithdrawalTestBody(t *testing.T, replayed bool) []byte {
	t.Helper()
	request := map[string]any{"protocol_version": 1, "scope": "report", "expected_issue_revision": 7,
		"expected_current_snapshot_id": reportSnapshotTestSnapshot, "reason": "Withdraw this report content"}
	intent, err := reportsnapshot.CanonicalJSON(map[string]any{"report_issue_id": reportSnapshotTestReport,
		"request_key": reportSnapshotTestKey, "request": request})
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(intent)
	raw, err := json.Marshal(map[string]any{"protocol_version": 1, "replayed": replayed,
		"withdrawal_receipt": map[string]any{"withdrawal_id": reportSnapshotTestOther, "request_key": reportSnapshotTestKey,
			"report_issue_id": reportSnapshotTestReport, "expected_current_snapshot_id": reportSnapshotTestSnapshot,
			"request_digest": hex.EncodeToString(digest[:]), "withdrawn_at": "2026-11-13T00:00:00Z"}})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestIssueReportSnapshotWithdrawAndOriginalReceiptReplay(t *testing.T) {
	requests := 0
	reportSnapshotFake(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodPost || r.URL.Path != "/api/issues/"+reportSnapshotTestReport+"/report-snapshots/withdrawals" || r.URL.RawQuery != "" || r.Header.Get("Idempotency-Key") != reportSnapshotTestKey {
			t.Errorf("unexpected withdrawal request %s %s", r.Method, r.URL)
		}
		var request map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil || len(request) != 5 || string(request["scope"]) != `"report"` || string(request["expected_issue_revision"]) != "7" || string(request["expected_current_snapshot_id"]) != `"`+reportSnapshotTestSnapshot+`"` {
			t.Errorf("withdrawal binding=%v err=%v", request, err)
		}
		if requests == 1 {
			w.WriteHeader(http.StatusCreated)
		}
		_, _ = w.Write(reportSnapshotWithdrawalTestBody(t, requests > 1))
	})
	for i := 1; i <= 2; i++ {
		output, err := runReportSnapshotCLI(t, reportSnapshotWithdrawArgs()...)
		if err != nil || requests != i || !strings.Contains(output, reportSnapshotTestOther) || strings.Contains(output, `"snapshot":`) {
			t.Fatalf("withdrawal receipt=%q err=%v requests=%d", output, err, requests)
		}
	}
}

func reportSnapshotMutateObject(t *testing.T, raw []byte, mutate func(map[string]json.RawMessage)) []byte {
	t.Helper()
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	mutate(fields)
	out, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestIssueReportSnapshotCaptureReceiptMustBindOriginalIntent(t *testing.T) {
	valid := reportSnapshotPublicTestBody(t, reportSnapshotTestInput(), false, true)
	for name, mutate := range map[string]func(map[string]json.RawMessage){
		"missing": func(fields map[string]json.RawMessage) { delete(fields, "capture_receipt") },
		"null":    func(fields map[string]json.RawMessage) { fields["capture_receipt"] = json.RawMessage("null") },
		"wrong key": func(fields map[string]json.RawMessage) {
			fields["capture_receipt"] = reportSnapshotMutateObject(t, fields["capture_receipt"], func(receipt map[string]json.RawMessage) {
				receipt["request_key"] = json.RawMessage(`"` + reportSnapshotTestOther + `"`)
			})
		},
		"wrong report": func(fields map[string]json.RawMessage) {
			fields["capture_receipt"] = reportSnapshotMutateObject(t, fields["capture_receipt"], func(receipt map[string]json.RawMessage) {
				receipt["report_issue_id"] = json.RawMessage(`"` + reportSnapshotTestOther + `"`)
			})
		},
		"wrong snapshot": func(fields map[string]json.RawMessage) {
			fields["capture_receipt"] = reportSnapshotMutateObject(t, fields["capture_receipt"], func(receipt map[string]json.RawMessage) {
				receipt["snapshot_id"] = json.RawMessage(`"` + reportSnapshotTestOther + `"`)
			})
		},
		"wrong commitment": func(fields map[string]json.RawMessage) {
			fields["capture_receipt"] = reportSnapshotMutateObject(t, fields["capture_receipt"], func(receipt map[string]json.RawMessage) {
				receipt["request_digest"] = json.RawMessage(`"` + strings.Repeat("0", 64) + `"`)
			})
		},
		"duplicate key": func(fields map[string]json.RawMessage) {
			fields["capture_receipt"] = append([]byte(`{"request_key":"`+reportSnapshotTestKey+`",`), fields["capture_receipt"][1:]...)
		},
		"case alias": func(fields map[string]json.RawMessage) {
			fields["capture_receipt"] = bytes.Replace(fields["capture_receipt"], []byte(`"request_key"`), []byte(`"Request_Key"`), 1)
		},
		"missing field": func(fields map[string]json.RawMessage) {
			fields["capture_receipt"] = reportSnapshotMutateObject(t, fields["capture_receipt"], func(receipt map[string]json.RawMessage) { delete(receipt, "request_digest") })
		},
		"private receipt field": func(fields map[string]json.RawMessage) {
			fields["capture_receipt"] = reportSnapshotMutateObject(t, fields["capture_receipt"], func(receipt map[string]json.RawMessage) { receipt["created_by"] = json.RawMessage(`"private-human"`) })
		},
	} {
		t.Run(name, func(t *testing.T) {
			raw := reportSnapshotMutateObject(t, valid, mutate)
			requests := 0
			reportSnapshotFake(t, func(w http.ResponseWriter, _ *http.Request) { requests++; w.WriteHeader(201); _, _ = w.Write(raw) })
			output, err := runReportSnapshotCLI(t, reportSnapshotCaptureArgs()...)
			if err == nil || output != "" || requests != 1 {
				t.Fatalf("invalid receipt output=%q err=%v requests=%d", output, err, requests)
			}
		})
	}
}

func TestIssueReportSnapshotShowRefusesPrivateFieldsAndHumanReceipt(t *testing.T) {
	valid := reportSnapshotTestBody(t, reportSnapshotTestInput(), false)
	internal, err := reportsnapshot.Build(reportSnapshotTestInput())
	if err != nil {
		t.Fatal(err)
	}
	private, err := json.Marshal(internal)
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(map[string]json.RawMessage){
		"internal snapshot": func(fields map[string]json.RawMessage) { fields["snapshot"] = private },
		"private root": func(fields map[string]json.RawMessage) {
			fields["read_cut"] = json.RawMessage(`"private-cut"`)
		},
		"private snapshot field": func(fields map[string]json.RawMessage) {
			fields["snapshot"] = reportSnapshotMutateObject(t, fields["snapshot"], func(snapshot map[string]json.RawMessage) { snapshot["created_by"] = json.RawMessage(`"private-human"`) })
		},
		"null human receipt": func(fields map[string]json.RawMessage) { fields["capture_receipt"] = json.RawMessage("null") },
		"human receipt": func(fields map[string]json.RawMessage) {
			var capture map[string]json.RawMessage
			if err := json.Unmarshal(reportSnapshotPublicTestBody(t, reportSnapshotTestInput(), false, true), &capture); err != nil {
				t.Fatal(err)
			}
			fields["capture_receipt"] = capture["capture_receipt"]
		},
	} {
		t.Run(name, func(t *testing.T) {
			raw := reportSnapshotMutateObject(t, valid, mutate)
			requests := 0
			reportSnapshotFake(t, func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.Method != http.MethodGet {
					t.Error("private response caused a write")
				}
				_, _ = w.Write(raw)
			})
			output, err := runReportSnapshotCLI(t, "show", reportSnapshotTestReport)
			if err == nil || output != "" || requests != 1 {
				t.Fatalf("private response output=%q err=%v requests=%d", output, err, requests)
			}
		})
	}
}

func TestIssueReportSnapshotWithdrawRequiresExplicitHumanIntent(t *testing.T) {
	requests := 0
	reportSnapshotFake(t, func(w http.ResponseWriter, _ *http.Request) { requests++; w.WriteHeader(500) })
	base := reportSnapshotWithdrawArgs()
	for name, args := range map[string][]string{
		"no accept":        base[:len(base)-1],
		"false accept":     append(append([]string{}, base[:len(base)-1]...), "--accept=false"),
		"no key":           reportSnapshotReplaceFlag(base, "--request-key", ""),
		"wrong key":        reportSnapshotReplaceFlag(base, "--request-key", "not-a-key"),
		"zero current":     reportSnapshotReplaceFlag(base, "--current-snapshot", "00000000-0000-0000-0000-000000000000"),
		"invalid revision": reportSnapshotReplaceFlag(base, "--expected-revision", "0"),
		"blank reason":     reportSnapshotReplaceFlag(base, "--reason", " \n\t"),
		"overlong reason":  reportSnapshotReplaceFlag(base, "--reason", strings.Repeat("中", 1366)),
		"NUL reason":       reportSnapshotReplaceFlag(base, "--reason", "withdraw\x00content"),
		"invalid output":   reportSnapshotReplaceFlag(base, "--output", "yaml"),
	} {
		t.Run(name, func(t *testing.T) {
			out, err := runReportSnapshotCLI(t, args...)
			if err == nil || out != "" || requests != 0 {
				t.Fatalf("invalid withdrawal out=%q err=%v requests=%d", out, err, requests)
			}
		})
	}
	for _, token := range []string{"mat_reporter", "mdt_daemon"} {
		t.Setenv("MULTICA_TOKEN", token)
		out, err := runReportSnapshotCLI(t, base...)
		if err == nil || out != "" || requests != 0 {
			t.Fatalf("machine withdrawal out=%q err=%v requests=%d", out, err, requests)
		}
	}
}

func TestIssueReportSnapshotWithdrawRefusesWrongOrPrivateReceipt(t *testing.T) {
	valid := reportSnapshotWithdrawalTestBody(t, false)
	for name, replacement := range map[string]string{
		"withdrawal_id": "00000000-0000-0000-0000-000000000000", "request_key": reportSnapshotTestOther,
		"report_issue_id": reportSnapshotTestOther, "expected_current_snapshot_id": reportSnapshotTestOther,
		"request_digest": strings.Repeat("0", 64), "withdrawn_at": "2026-11-13T00:00:00.0000001Z",
	} {
		t.Run(name, func(t *testing.T) {
			raw := reportSnapshotMutateObject(t, valid, func(fields map[string]json.RawMessage) {
				fields["withdrawal_receipt"] = reportSnapshotMutateObject(t, fields["withdrawal_receipt"], func(receipt map[string]json.RawMessage) {
					value, _ := json.Marshal(replacement)
					receipt[name] = value
				})
			})
			reportSnapshotFake(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(201); _, _ = w.Write(raw) })
			output, err := runReportSnapshotCLI(t, reportSnapshotWithdrawArgs()...)
			if err == nil || output != "" {
				t.Fatalf("wrong withdrawal receipt output=%q err=%v", output, err)
			}
		})
	}
	for name, mutate := range map[string]func(map[string]json.RawMessage){
		"content": func(fields map[string]json.RawMessage) {
			fields["snapshot"] = json.RawMessage(`{"private":"retained body"}`)
		},
		"missing": func(fields map[string]json.RawMessage) { delete(fields, "withdrawal_receipt") },
		"null":    func(fields map[string]json.RawMessage) { fields["withdrawal_receipt"] = json.RawMessage("null") },
		"case": func(fields map[string]json.RawMessage) {
			fields["withdrawal_receipt"] = bytes.Replace(fields["withdrawal_receipt"], []byte(`"withdrawal_id"`), []byte(`"Withdrawal_ID"`), 1)
		},
		"duplicate": func(fields map[string]json.RawMessage) {
			fields["withdrawal_receipt"] = append([]byte(`{"request_key":"`+reportSnapshotTestKey+`",`), fields["withdrawal_receipt"][1:]...)
		},
		"replayed created": func(fields map[string]json.RawMessage) { fields["replayed"] = json.RawMessage("true") },
	} {
		t.Run(name, func(t *testing.T) {
			raw := reportSnapshotMutateObject(t, valid, mutate)
			reportSnapshotFake(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(201); _, _ = w.Write(raw) })
			output, err := runReportSnapshotCLI(t, reportSnapshotWithdrawArgs()...)
			if err == nil || output != "" {
				t.Fatalf("malformed withdrawal output=%q err=%v", output, err)
			}
		})
	}
}

func TestIssueReportSnapshotWithdrawResponseLossNeverRetries(t *testing.T) {
	requests := 0
	reportSnapshotFake(t, func(w http.ResponseWriter, _ *http.Request) {
		requests++
		connection, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		_ = connection.Close()
	})
	output, err := runReportSnapshotCLI(t, reportSnapshotWithdrawArgs()...)
	if err == nil || output != "" || requests != 1 || !strings.Contains(err.Error(), "original --request-key") {
		t.Fatalf("uncertain withdrawal output=%q err=%v requests=%d", output, err, requests)
	}
}

func TestIssueReportSnapshotWithdrawPreservesExactReasonAndRefusesContentFallback(t *testing.T) {
	reason := " \n撤回这一份报告\t "
	posts, gets := 0, 0
	reportSnapshotFake(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			gets++
			w.WriteHeader(404)
			return
		}
		posts++
		var request struct {
			ProtocolVersion           int    `json:"protocol_version"`
			Scope                     string `json:"scope"`
			ExpectedIssueRevision     int64  `json:"expected_issue_revision"`
			ExpectedCurrentSnapshotID string `json:"expected_current_snapshot_id"`
			Reason                    string `json:"reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.Reason != reason {
			t.Errorf("withdrawal reason changed: %q err=%v", request.Reason, err)
		}
		intent, err := reportsnapshot.CanonicalJSON(map[string]any{"report_issue_id": reportSnapshotTestReport, "request_key": reportSnapshotTestKey, "request": request})
		if err != nil {
			t.Error(err)
		}
		digest := sha256.Sum256(intent)
		raw := reportSnapshotMutateObject(t, reportSnapshotWithdrawalTestBody(t, false), func(fields map[string]json.RawMessage) {
			fields["withdrawal_receipt"] = reportSnapshotMutateObject(t, fields["withdrawal_receipt"], func(receipt map[string]json.RawMessage) {
				receipt["request_digest"] = json.RawMessage(`"` + hex.EncodeToString(digest[:]) + `"`)
			})
		})
		w.WriteHeader(201)
		_, _ = w.Write(raw)
	})
	args := reportSnapshotReplaceFlag(reportSnapshotWithdrawArgs(), "--reason", reason)
	args = reportSnapshotReplaceFlag(args, "--output", "table")
	output, err := runReportSnapshotCLI(t, args...)
	if err != nil || posts != 1 || !strings.Contains(output, "withdrawn") || strings.Contains(output, "METRIC") {
		t.Fatalf("withdrawal table=%q err=%v posts=%d", output, err, posts)
	}
	for _, args := range [][]string{{"show", reportSnapshotTestReport}, {"show", reportSnapshotTestReport, "--snapshot", reportSnapshotTestSnapshot}} {
		output, err = runReportSnapshotCLI(t, args...)
		if err == nil || output != "" || posts != 1 {
			t.Fatalf("withdrawn read fell back: %q %v posts=%d", output, err, posts)
		}
	}
	if gets != 2 {
		t.Fatalf("expected two explicit GETs, got %d", gets)
	}
}
