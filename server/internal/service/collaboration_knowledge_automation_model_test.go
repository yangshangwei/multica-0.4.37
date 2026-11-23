package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/pkg/llm"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func automationModelFixture(t *testing.T) knowledgeAutomationInput {
	t.Helper()
	result, basis := "Amounts use cents. Preserve the request identity.", "The accepted API contract records both rules."
	canonical, err := knowledgeDeliveryCanonical(result, basis)
	if err != nil {
		t.Fatal(err)
	}
	return knowledgeAutomationInput{MaxCandidates: 3, Sources: []knowledgeAutomationSource{{Ref: "s1", Result: result, Basis: basis, Canonical: canonical,
		Source: protocol.CollaborationKnowledgeSource{Kind: "delivery", IssueID: "00000000-0000-4000-8000-000000000001",
			SubmissionID: "00000000-0000-4000-8000-000000000002", DeliveryEventSeq: "10", AcceptedEventSeq: "11", SHA256: protocol.CollaborationKnowledgeHash(canonical)}}}}
}

func automationGenerationJSON(t *testing.T, refs ...string) string {
	t.Helper()
	candidates := make([]map[string]any, 0, len(refs))
	for _, ref := range refs {
		candidates = append(candidates, map[string]any{"ref": ref, "summary": "Amounts use cents.", "statements": []knowledgeAutomationStatement{{
			Text: "Amounts use cents.", SourceRef: "s1", Field: "result", Quote: "Amounts use cents."}}})
	}
	raw, err := json.Marshal(map[string]any{"candidates": candidates})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func requireAutomationReason(t *testing.T, err error, reason string) {
	t.Helper()
	var failure *knowledgeAutomationModelError
	if !errors.As(err, &failure) || failure.Reason != reason || err.Error() != reason {
		t.Fatalf("error=%v, want fixed reason %q", err, reason)
	}
}

func TestKnowledgeAutomationModelStagesKeepPrivateIdentitiesAndComparisonBoundaries(t *testing.T) {
	input := automationModelFixture(t)
	input.Comparisons = []knowledgeAutomationComparison{{Ref: "c1", Summary: "Comparison summary", Content: "PRIVATE OTHER SOURCE COMPARISON"}}
	generation, err := buildKnowledgeAutomationGenerationRequest(input)
	if err != nil {
		t.Fatal(err)
	}
	if generation.MaxTokens != 6000 || generation.MaxOutputBytes != 24<<10 || strings.Contains(generation.UserPrompt, "PRIVATE OTHER SOURCE") {
		t.Fatalf("wrong generation budget or comparison disclosure: %+v", generation)
	}
	candidates, err := parseKnowledgeAutomationGeneration(automationGenerationJSON(t, "k1"), input)
	if err != nil {
		t.Fatal(err)
	}
	check, err := buildKnowledgeAutomationCheckRequest(input, candidates)
	if err != nil {
		t.Fatal(err)
	}
	if check.MaxTokens != 3000 || check.MaxOutputBytes != 16<<10 || !strings.Contains(check.UserPrompt, "PRIVATE OTHER SOURCE COMPARISON") || !strings.Contains(check.UserPrompt, candidates[0].Summary) {
		t.Fatalf("checker omitted a complete comparison/candidate or changed budget: %+v", check)
	}
	for _, request := range []knowledgeAutomationModelRequest{generation, check} {
		for _, private := range []string{input.Sources[0].Source.IssueID, input.Sources[0].Source.SubmissionID, input.Sources[0].Source.SHA256, "accepted_event_seq", "source_issue_id"} {
			if strings.Contains(request.UserPrompt, private) || strings.Contains(request.SystemPrompt, private) {
				t.Fatalf("private identity %q reached model request", private)
			}
		}
	}
	if len(candidates[0].Sources) != 1 || candidates[0].Sources[0] != input.Sources[0].Source || candidates[0].Intent != "create" {
		t.Fatalf("generated provenance did not map to the frozen source: %+v", candidates)
	}
}

func TestKnowledgeAutomationModelGenerationBoundaries(t *testing.T) {
	input := automationModelFixture(t)
	valid := automationGenerationJSON(t, "k1")
	for name, raw := range map[string]string{
		"duplicate key":     strings.Replace(valid, `"ref":"k1"`, `"ref":"k1","ref":"k2"`, 1),
		"escaped duplicate": strings.Replace(valid, `"ref":"k1"`, `"ref":"k1","\u0072ef":"k2"`, 1),
		"case alias":        strings.Replace(valid, `"ref":"k1"`, `"Ref":"k1"`, 1),
		"extra candidate":   strings.Replace(valid, `"ref":"k1"`, `"ref":"k1","approved":true`, 1),
		"unknown source":    strings.Replace(valid, `"s1"`, `"c1"`, 1),
		"unknown field":     strings.Replace(valid, `"field":"result"`, `"field":"content"`, 1),
		"false quote":       strings.Replace(valid, `"quote":"Amounts use cents."`, `"quote":"Amounts use dollars."`, 1),
		"empty quote":       strings.Replace(valid, `"quote":"Amounts use cents."`, `"quote":""`, 1),
		"missing statement": strings.Replace(valid, `"statements"`, `"statements_absent"`, 1),
		"extra root":        strings.TrimSuffix(valid, "}") + `,"job_id":"forged"}`,
		"trailing JSON":     valid + `{}`,
		"markdown fence":    "```json\n" + valid + "\n```",
		"invalid UTF8":      strings.Replace(valid, "Amounts use cents.", string([]byte{0xff}), 1),
		"unpaired escape":   strings.Replace(valid, "Amounts use cents.", `\ud800`, 1),
		"null collection":   `{"candidates":null}`,
		"unknown ref":       automationGenerationJSON(t, "k4"),
		"duplicate ref":     automationGenerationJSON(t, "k1", "k1"),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := parseKnowledgeAutomationGeneration(raw, input)
			requireAutomationReason(t, err, "model_output_invalid")
		})
	}
	zero, err := parseKnowledgeAutomationGeneration(`{"candidates":[]}`, input)
	if err != nil || zero == nil || len(zero) != 0 {
		t.Fatalf("zero useful conclusions is a valid complete result: %+v %v", zero, err)
	}
	input.MaxCandidates = 1
	_, err = parseKnowledgeAutomationGeneration(automationGenerationJSON(t, "k1", "k2"), input)
	requireAutomationReason(t, err, "model_output_invalid")
	_, err = parseKnowledgeAutomationGeneration(strings.Repeat(" ", (24<<10)+1), input)
	requireAutomationReason(t, err, "model_output_too_large")
}

func TestKnowledgeAutomationModelRevalidationCoversEveryFixedPredecessor(t *testing.T) {
	input := automationModelFixture(t)
	input.MaxCandidates = 1
	input.Predecessors = []knowledgeAutomationPredecessor{
		{Ref: "k1", Summary: "Amounts use cents.", Content: "Amounts use cents."},
		{Ref: "k2", Summary: "Old rule", Content: "Amounts use dollars."},
		{Ref: "k3", Summary: "Amounts use cents.", Content: "Amounts use cents."},
	}
	candidates, err := parseKnowledgeAutomationGeneration(automationGenerationJSON(t, "k3", "k1", "k2"), input)
	if err != nil || len(candidates) != 3 {
		t.Fatalf("review was incorrectly limited by the new-candidate budget: %+v %v", candidates, err)
	}
	if candidates[0].Ref != "k1" || candidates[0].Intent != "revalidate" || candidates[1].Ref != "k2" || candidates[1].Intent != "revise" || candidates[2].Intent != "revalidate" {
		t.Fatalf("review did not retain fixed predecessor order/intents: %+v", candidates)
	}
	for _, raw := range []string{`{"candidates":[]}`, automationGenerationJSON(t, "k1", "k2"), automationGenerationJSON(t, "k1", "k2", "k4")} {
		_, err := parseKnowledgeAutomationGeneration(raw, input)
		requireAutomationReason(t, err, "model_output_invalid")
	}
	input.Predecessors = input.Predecessors[:1]
	input.Predecessors[0].Summary = "A different original claim"
	candidates, err = parseKnowledgeAutomationGeneration(automationGenerationJSON(t, "k1"), input)
	if err != nil || candidates[0].Intent != "revise" {
		t.Fatalf("changed immutable summary must be revised, not restored unchecked: %+v %v", candidates, err)
	}
}

func TestKnowledgeAutomationModelReviewComparisonMustBeExact(t *testing.T) {
	input := automationModelFixture(t)
	input.Predecessors = []knowledgeAutomationPredecessor{{Ref: "k1", Content: "Amounts use cents.", Summary: "Amounts use cents."}}
	candidates, err := parseKnowledgeAutomationGeneration(automationGenerationJSON(t, "k1"), input)
	if err != nil {
		t.Fatal(err)
	}
	_, err = buildKnowledgeAutomationCheckRequest(input, candidates)
	requireAutomationReason(t, err, "comparison_unavailable")
	input.Comparisons = []knowledgeAutomationComparison{{Ref: "c1", Content: "Other content", Summary: "Amounts use cents.", PredecessorFor: "k1"}}
	_, err = buildKnowledgeAutomationCheckRequest(input, candidates)
	requireAutomationReason(t, err, "target_invalid")
	input.Comparisons[0].Content = input.Predecessors[0].Content
	if _, err = buildKnowledgeAutomationCheckRequest(input, candidates); err != nil {
		t.Fatal(err)
	}
}

func TestKnowledgeAutomationModelDecodedAndWireBudgets(t *testing.T) {
	request, err := buildKnowledgeAutomationGenerationRequest(automationModelFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	oversized := request
	raw, err := json.Marshal(map[string]string{"text": strings.Repeat("x", knowledgeAutomationMaxInputBytes)})
	if err != nil {
		t.Fatal(err)
	}
	oversized.UserPrompt = string(raw)
	requireAutomationReason(t, checkKnowledgeAutomationRequestBudget(oversized, "model"), "input_budget_exceeded")
	requireAutomationReason(t, checkKnowledgeAutomationRequestBudget(request, strings.Repeat("\x01", 400000)), "input_budget_exceeded")
	input := automationModelFixture(t)
	text := strings.Repeat("x", 120<<10) + " COMPLETE SOURCE TAIL"
	input.Sources[0] = knowledgeAutomationSource{Ref: "s1", Result: text, Canonical: text, Source: protocol.CollaborationKnowledgeSource{
		Kind: "memory", IssueID: input.Sources[0].Source.IssueID, Version: "1", SHA256: protocol.CollaborationKnowledgeHash(text)}}
	complete, err := buildKnowledgeAutomationGenerationRequest(input)
	if err != nil || !strings.Contains(complete.UserPrompt, "COMPLETE SOURCE TAIL") {
		t.Fatalf("complete source was truncated: %v", err)
	}
}

func TestKnowledgeAutomationModelChecksCannotRewriteOrOmitCandidates(t *testing.T) {
	input := automationModelFixture(t)
	input.Comparisons = []knowledgeAutomationComparison{{Ref: "c1", Content: "Other rule", Summary: "Other rule"}}
	candidates, err := parseKnowledgeAutomationGeneration(automationGenerationJSON(t, "k1"), input)
	if err != nil {
		t.Fatal(err)
	}
	valid := `{"checks":[{"ref":"k1","support":"supported","reuse":"reusable","relation":"novel","related_refs":[],"reason":"The summary and statements are supported."}]}`
	checks, err := parseKnowledgeAutomationChecks(valid, input, candidates)
	if err != nil || len(checks) != 1 || checks[0].Ref != "k1" || candidates[0].Content != "Amounts use cents." {
		t.Fatalf("checker changed the frozen candidate: %+v %+v %v", checks, candidates, err)
	}
	for name, raw := range map[string]string{
		"missing check":            `{"checks":[]}`,
		"null refs":                strings.Replace(valid, `"related_refs":[]`, `"related_refs":null`, 1),
		"unknown ref":              strings.Replace(valid, `"ref":"k1"`, `"ref":"k2"`, 1),
		"unknown relation":         strings.Replace(valid, `"novel"`, `"probably_fine"`, 1),
		"unknown related source":   strings.Replace(valid, `"related_refs":[]`, `"related_refs":["c2"]`, 1),
		"self citation":            strings.Replace(valid, `"related_refs":[]`, `"related_refs":["k1"]`, 1),
		"duplicate related source": strings.Replace(valid, `"related_refs":[]`, `"related_refs":["c1","c1"]`, 1),
		"rewritten content":        strings.Replace(valid, `"ref":"k1"`, `"ref":"k1","content":"New rule"`, 1),
		"duplicate field":          strings.Replace(valid, `"support":"supported"`, `"support":"supported","support":"unsupported"`, 1),
		"large reason":             strings.Replace(valid, "The summary and statements are supported.", strings.Repeat("r", 1001), 1),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := parseKnowledgeAutomationChecks(raw, input, candidates)
			requireAutomationReason(t, err, "model_output_invalid")
		})
	}
	_, err = parseKnowledgeAutomationChecks(strings.Repeat(" ", (16<<10)+1), input, candidates)
	requireAutomationReason(t, err, "model_output_too_large")
}

func TestKnowledgeAutomationModelInputBudgetsAndFrozenHashes(t *testing.T) {
	for name, change := range map[string]func(*knowledgeAutomationInput){
		"changed decoded field": func(in *knowledgeAutomationInput) { in.Sources[0].Result += " Changed." },
		"changed complete hash": func(in *knowledgeAutomationInput) { in.Sources[0].Source.SHA256 = strings.Repeat("0", 64) },
		"missing source":        func(in *knowledgeAutomationInput) { in.Sources = nil },
		"unexpected source ref": func(in *knowledgeAutomationInput) { in.Sources[0].Ref = "s2" },
	} {
		t.Run(name, func(t *testing.T) {
			input := automationModelFixture(t)
			change(&input)
			_, err := buildKnowledgeAutomationGenerationRequest(input)
			requireAutomationReason(t, err, "source_invalid")
		})
	}
	input := automationModelFixture(t)
	for len(input.Sources) < 9 {
		input.Sources = append(input.Sources, input.Sources[0])
	}
	_, err := buildKnowledgeAutomationGenerationRequest(input)
	requireAutomationReason(t, err, "source_budget_exceeded")
	input = automationModelFixture(t)
	text := strings.Repeat("s", 128<<10)
	input.Sources[0] = knowledgeAutomationSource{Ref: "s1", Result: text, Canonical: text,
		Source: protocol.CollaborationKnowledgeSource{Kind: "memory", IssueID: input.Sources[0].Source.IssueID, Version: "1", SHA256: protocol.CollaborationKnowledgeHash(text)}}
	_, err = buildKnowledgeAutomationGenerationRequest(input)
	requireAutomationReason(t, err, "source_budget_exceeded")
	input = automationModelFixture(t)
	for i := 0; i < 17; i++ {
		input.Comparisons = append(input.Comparisons, knowledgeAutomationComparison{Ref: "c" + strconv.Itoa(i+1), Content: "Other rule", Summary: "Other"})
	}
	_, err = buildKnowledgeAutomationGenerationRequest(input)
	requireAutomationReason(t, err, "target_budget_exceeded")
	input.Comparisons = input.Comparisons[:16]
	for i := range input.Comparisons {
		input.Comparisons[i].Content = strings.Repeat("c", 8192)
	}
	_, err = buildKnowledgeAutomationGenerationRequest(input)
	requireAutomationReason(t, err, "target_budget_exceeded")
}

func TestKnowledgeAutomationModelCandidateByteBudgets(t *testing.T) {
	input := automationModelFixture(t)
	for name, change := range map[string]func(map[string]any){
		"summary": func(candidate map[string]any) { candidate["summary"] = strings.Repeat("知", 86) },
		"content": func(candidate map[string]any) {
			candidate["statements"] = []knowledgeAutomationStatement{{Text: strings.Repeat("知", 683), SourceRef: "s1", Field: "result", Quote: "Amounts use cents."}}
		},
		"statement count": func(candidate map[string]any) {
			statements := make([]knowledgeAutomationStatement, 5)
			for i := range statements {
				statements[i] = knowledgeAutomationStatement{Text: "Rule", SourceRef: "s1", Field: "result", Quote: "Amounts use cents."}
			}
			candidate["statements"] = statements
		},
	} {
		t.Run(name, func(t *testing.T) {
			candidate := map[string]any{"ref": "k1", "summary": "Summary", "statements": []knowledgeAutomationStatement{{Text: "Amounts use cents.", SourceRef: "s1", Field: "result", Quote: "Amounts use cents."}}}
			change(candidate)
			raw, err := json.Marshal(map[string]any{"candidates": []any{candidate}})
			if err != nil {
				t.Fatal(err)
			}
			_, err = parseKnowledgeAutomationGeneration(string(raw), input)
			requireAutomationReason(t, err, "model_output_invalid")
		})
	}
}

func TestKnowledgeAutomationModelCallPreservesUsageAndSanitizesFailures(t *testing.T) {
	for _, tc := range []struct {
		name, output, reason string
		status               int
	}{
		{"valid", `{"candidates":[]}`, "", http.StatusOK},
		{"oversized", strings.Repeat("x", (24<<10)+1), "model_output_too_large", http.StatusOK},
		{"invalid", `{"candidates":[],"candidates":[]}`, "model_output_invalid", http.StatusOK},
		{"provider error", "PRIVATE PROVIDER ERROR", "model_request_failed", http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				var response any = map[string]any{"model": "served-model", "choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": tc.output}, "finish_reason": "stop"}}, "usage": map[string]any{"prompt_tokens": 8, "completion_tokens": 2, "total_tokens": 10}}
				if tc.status != http.StatusOK {
					response = map[string]any{"error": map[string]any{"message": tc.output, "type": "invalid_request_error"}}
				}
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Errorf("encode fake response: %v", err)
				}
			}))
			t.Cleanup(srv.Close)
			retries, err := llm.Retries(0)
			if err != nil {
				t.Fatal(err)
			}
			client := llm.New(llm.Config{BaseURL: srv.URL, MaxRetries: retries})
			request, err := buildKnowledgeAutomationGenerationRequest(automationModelFixture(t))
			if err != nil {
				t.Fatal(err)
			}
			result, err := callKnowledgeAutomationModel(context.Background(), client, request)
			if tc.reason == "" {
				if err != nil || result.Text != tc.output {
					t.Fatalf("valid output=%+v err=%v", result, err)
				}
			} else {
				requireAutomationReason(t, err, tc.reason)
			}
			if tc.status == http.StatusOK && (result.Model != "served-model" || result.Usage.Status != llm.UsageKnown || result.Usage.TotalTokens == nil || *result.Usage.TotalTokens != 10) {
				t.Fatalf("lost actual usage on %s: %+v", tc.name, result)
			}
			if err != nil && strings.Contains(err.Error(), "PRIVATE") {
				t.Fatal("private provider failure leaked")
			}
		})
	}
	request, err := buildKnowledgeAutomationGenerationRequest(automationModelFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	result, err := callKnowledgeAutomationModel(context.Background(), llm.New(llm.Config{}), request)
	if !errors.Is(err, llm.ErrNotConfigured) || result.Usage.Status != llm.UsageUnknown {
		t.Fatalf("unconfigured result=%+v err=%v", result, err)
	}
	request.UserPrompt = strings.Repeat("x", (2<<20)+1)
	_, err = callKnowledgeAutomationModel(context.Background(), llm.New(llm.Config{BaseURL: "http://127.0.0.1:1"}), request)
	requireAutomationReason(t, err, "input_budget_exceeded")
}
