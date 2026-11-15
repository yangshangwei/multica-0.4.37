package service

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

type knowledgeAutomationGeneratedText struct {
	Ref        string                         `json:"ref"`
	Summary    string                         `json:"summary"`
	Statements []knowledgeAutomationStatement `json:"statements"`
}

func parseKnowledgeAutomationGeneration(raw string, input knowledgeAutomationInput) ([]knowledgeAutomationCandidate, error) {
	if len(raw) > knowledgeAutomationGenerationBytes {
		return nil, automationModelError("model_output_too_large")
	}
	if err := validateKnowledgeAutomationInput(input); err != nil {
		return nil, err
	}
	invalid := func() ([]knowledgeAutomationCandidate, error) {
		return nil, automationModelError("model_output_invalid")
	}
	if !validKnowledgeAutomationJSON(raw) {
		return invalid()
	}
	var output struct {
		Candidates []json.RawMessage `json:"candidates"`
	}
	if !decodeKnowledgeAutomationObject([]byte(raw), &output, "candidates") || output.Candidates == nil {
		return invalid()
	}
	count := len(output.Candidates)
	if len(input.Predecessors) > 0 {
		if count != len(input.Predecessors) {
			return invalid()
		}
	} else if count > input.MaxCandidates {
		return invalid()
	}
	sources := make(map[string]knowledgeAutomationSource, len(input.Sources))
	for _, source := range input.Sources {
		sources[source.Ref] = source
	}
	byRef := make(map[string]knowledgeAutomationCandidate, count)
	for _, item := range output.Candidates {
		var generated struct {
			Ref        string            `json:"ref"`
			Summary    string            `json:"summary"`
			Statements []json.RawMessage `json:"statements"`
		}
		if !decodeKnowledgeAutomationObject(item, &generated, "ref", "summary", "statements") || !automationText(generated.Summary, knowledgeAutomationMaxSummaryBytes) ||
			len(generated.Statements) == 0 || len(generated.Statements) > knowledgeAutomationMaxStatements {
			return invalid()
		}
		if _, duplicate := byRef[generated.Ref]; duplicate {
			return invalid()
		}
		candidate := knowledgeAutomationCandidate{Ref: generated.Ref, Summary: generated.Summary, Intent: "create",
			Statements: make([]knowledgeAutomationStatement, 0, len(generated.Statements)), Sources: make([]protocol.CollaborationKnowledgeSource, 0, len(generated.Statements))}
		texts := make([]string, 0, len(generated.Statements))
		for _, statementRaw := range generated.Statements {
			var statement knowledgeAutomationStatement
			if !decodeKnowledgeAutomationObject(statementRaw, &statement, "text", "source_ref", "field", "quote") || !automationText(statement.Text, knowledgeAutomationMaxContentBytes) || !automationText(statement.Quote, knowledgeAutomationGenerationBytes) {
				return invalid()
			}
			source, found := sources[statement.SourceRef]
			if !found {
				return invalid()
			}
			var field string
			switch statement.Field {
			case "result":
				field = source.Result
			case "basis":
				field = source.Basis
			default:
				return invalid()
			}
			if !strings.Contains(field, statement.Quote) {
				return invalid()
			}
			texts = append(texts, statement.Text)
			candidate.Statements = append(candidate.Statements, statement)
			if !slices.Contains(candidate.Sources, source.Source) {
				candidate.Sources = append(candidate.Sources, source.Source)
			}
		}
		candidate.Content = normalizeKnowledgeAutomationContent(strings.Join(texts, "\n"))
		if !automationText(candidate.Content, knowledgeAutomationMaxContentBytes) {
			return invalid()
		}
		byRef[candidate.Ref] = candidate
	}
	result := make([]knowledgeAutomationCandidate, 0, count)
	for i := 0; i < count; i++ {
		ref := fmt.Sprintf("k%d", i+1)
		candidate, found := byRef[ref]
		if !found {
			return invalid()
		}
		if len(input.Predecessors) > 0 {
			prior := input.Predecessors[i]
			candidate.Intent = "revise"
			if candidate.Content == normalizeKnowledgeAutomationContent(prior.Content) && candidate.Summary == prior.Summary {
				candidate.Intent = "revalidate"
			}
		}
		result = append(result, candidate)
	}
	return result, nil
}

func validateKnowledgeAutomationCandidates(input knowledgeAutomationInput, candidates []knowledgeAutomationCandidate) error {
	generated := make([]knowledgeAutomationGeneratedText, 0, len(candidates))
	for _, candidate := range candidates {
		generated = append(generated, knowledgeAutomationGeneratedText{Ref: candidate.Ref, Summary: candidate.Summary, Statements: candidate.Statements})
	}
	raw, err := json.Marshal(struct {
		Candidates []knowledgeAutomationGeneratedText `json:"candidates"`
	}{generated})
	if err != nil {
		return automationModelError("model_output_invalid")
	}
	parsed, err := parseKnowledgeAutomationGeneration(string(raw), input)
	if err != nil {
		return err
	}
	for i, candidate := range candidates {
		actual := parsed[i]
		if candidate.Ref != actual.Ref || candidate.Content != actual.Content || candidate.Summary != actual.Summary || candidate.Intent != actual.Intent ||
			!slices.Equal(candidate.Statements, actual.Statements) || !slices.Equal(candidate.Sources, actual.Sources) {
			return automationModelError("model_output_invalid")
		}
	}
	return nil
}

func parseKnowledgeAutomationChecks(raw string, input knowledgeAutomationInput, candidates []knowledgeAutomationCandidate) ([]knowledgeAutomationCheck, error) {
	if len(raw) > knowledgeAutomationCheckBytes {
		return nil, automationModelError("model_output_too_large")
	}
	if err := validateKnowledgeAutomationCandidates(input, candidates); err != nil {
		return nil, err
	}
	invalid := func() ([]knowledgeAutomationCheck, error) { return nil, automationModelError("model_output_invalid") }
	if !validKnowledgeAutomationJSON(raw) {
		return invalid()
	}
	var output struct {
		Checks []json.RawMessage `json:"checks"`
	}
	if !decodeKnowledgeAutomationObject([]byte(raw), &output, "checks") || output.Checks == nil || len(output.Checks) != len(candidates) {
		return invalid()
	}
	knownRefs := make(map[string]bool, len(candidates)+len(input.Comparisons))
	candidateRefs := make(map[string]bool, len(candidates))
	for _, candidate := range candidates {
		candidateRefs[candidate.Ref], knownRefs[candidate.Ref] = true, true
	}
	for _, comparison := range input.Comparisons {
		knownRefs[comparison.Ref] = true
	}
	checks := make(map[string]knowledgeAutomationCheck, len(candidates))
	for _, item := range output.Checks {
		var check knowledgeAutomationCheck
		if !decodeKnowledgeAutomationObject(item, &check, "ref", "support", "reuse", "relation", "related_refs", "reason") || !candidateRefs[check.Ref] ||
			!slices.Contains([]string{"supported", "unsupported", "uncertain"}, check.Support) || !slices.Contains([]string{"reusable", "transient", "uncertain"}, check.Reuse) ||
			!slices.Contains([]string{"novel", "duplicate", "conflict", "uncertain"}, check.Relation) || !automationText(check.Reason, knowledgeAutomationMaxCheckReasonBytes) || check.RelatedRefs == nil {
			return invalid()
		}
		if _, duplicate := checks[check.Ref]; duplicate {
			return invalid()
		}
		seen := make(map[string]bool, len(check.RelatedRefs))
		for _, ref := range check.RelatedRefs {
			if !knownRefs[ref] || ref == check.Ref || seen[ref] {
				return invalid()
			}
			seen[ref] = true
		}
		if (check.Relation == "duplicate" || check.Relation == "conflict") && len(check.RelatedRefs) == 0 {
			return invalid()
		}
		checks[check.Ref] = check
	}
	result := make([]knowledgeAutomationCheck, 0, len(candidates))
	for _, candidate := range candidates {
		check, found := checks[candidate.Ref]
		if !found {
			return invalid()
		}
		result = append(result, check)
	}
	return result, nil
}

func normalizeKnowledgeAutomationContent(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "\r\n", "\n"), "\r", "\n")
}

func knowledgeAutomationRuleOutcome(candidate knowledgeAutomationCandidate, check knowledgeAutomationCheck, predecessorComparisonRef string) (string, string) {
	if candidate.Ref != check.Ref || !slices.Contains([]string{"create", "revalidate", "revise"}, candidate.Intent) {
		return "review_pending", "model_output_invalid"
	}
	if check.Support != "supported" {
		return "review_pending", "unsupported_content"
	}
	if check.Reuse == "transient" {
		return "transient", "no_reusable_knowledge"
	}
	if check.Reuse != "reusable" {
		return "review_pending", "reuse_uncertain"
	}
	if candidate.Intent != "create" && predecessorComparisonRef == "" {
		return "review_pending", "comparison_unavailable"
	}
	selfOnly := predecessorComparisonRef != "" && len(check.RelatedRefs) == 1 && check.RelatedRefs[0] == predecessorComparisonRef
	switch check.Relation {
	case "novel":
		return "accept", "rules_passed"
	case "duplicate":
		if candidate.Intent == "create" && len(check.RelatedRefs) > 0 {
			return "duplicate", "duplicate"
		}
		if candidate.Intent != "create" && selfOnly {
			return "accept", "rules_passed"
		}
		return "review_pending", "comparison_conflict"
	case "conflict":
		if candidate.Intent == "revise" && selfOnly {
			return "accept", "rules_passed"
		}
		return "review_pending", "conflict"
	default:
		return "review_pending", "comparison_uncertain"
	}
}
