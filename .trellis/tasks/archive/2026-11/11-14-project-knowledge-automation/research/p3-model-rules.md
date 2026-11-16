# P3 bounded model requests and pure rules

2026-11-14; native `trellis-implement` child `p0_core_contracts`.

## Interfaces delivered

`server/internal/service/collaboration_knowledge_automation_model.go` and
`collaboration_knowledge_automation_rules.go` now provide the DTOs and functions
agreed with the worker owner:

- `buildKnowledgeAutomationGenerationRequest(input)` and
  `buildKnowledgeAutomationCheckRequest(input, candidates)`;
- `callKnowledgeAutomationModel(ctx, client, request)`;
- `parseKnowledgeAutomationGeneration(raw, input)` and
  `parseKnowledgeAutomationChecks(raw, input, candidates)`;
- `knowledgeAutomationRuleOutcome(candidate, check, predecessorComparisonRef)`;
- `normalizeKnowledgeAutomationContent(value)`.

Sources use `s1..s8`, candidates/fixed predecessors `k1..k3`, comparisons
`c1..c16`. Explicit model DTOs omit internal source IDs/hashes. Generation sees
complete sources and same-source predecessors only; the checker additionally
receives complete comparisons and frozen candidate summaries/statements. Private
model reasons remain private; their `related_refs` never replace the worker's
full input-source authorization closure.

Source fields must reconstruct their exact full canonical hash. Legacy material
uses `Result=Canonical`, empty `Basis`; delivery uses the existing complete
result/basis canonicalizer. Duplicate/unknown/aliased JSON fields, malformed
UTF-8/unpaired Unicode escapes, trailing data, incomplete review/check results,
unknown refs and non-exact field quotes are rejected without repair/truncation.
Input and output budgets follow the approved design and reuse
`protocol.CollaborationKnowledgeSizes` for decoded UTF-8 plus framing.

## Rule and coordination details

- Create requires supported, reusable, novel; duplicates skip.
- Revalidation requires unchanged normalized body **and summary**. Main approved
  this refinement because both remain immutable when the old version is restored.
  A changed summary uses exact-lineage revision. Applicability remains the
  worker's exact/fixed-scope responsibility.
- A comparison tagged as a predecessor must match its fixed body and summary.
  Only a sole exact predecessor reference can receive the duplicate/change
  exception; additional or unrelated conflicts remain pending.
- Errors return fixed `knowledgeAutomationModelError.Reason` codes. Transport
  causes are retained only for `errors.Is` classification; generic error text is
  content-free. Actual model/usage survives output errors. Parsed candidate
  structure is validated again before checker construction.
- Helpers perform no database/authentication/target selection. The worker owns
  complete target metadata/ACL acquisition, stage persistence, exact duplicates,
  atomic capacity/current-policy checks and final settlement.

## Validation and changed documentation

- Focused service tests: **11 top-level tests and 51 subtests passed**, no
  failures/skips; `DATABASE_URL` and model-account environment variables removed.
  Raw evidence: `p3-model-rules-run2.jsonl`.
- Earlier compile attempts were blocked by concurrent public predicate duplication
  and then the worker type being written. Their original logs remain in
  `p3-model-rules-first.log` and `p3-model-rules-tests.jsonl`; they are not reported
  as product test failures or passing runs.
- Full `pkg/llm` tests, including consumer and SDK import guards: passed
  (`p3-model-rules-llm.jsonl`). `go vet ./internal/service ./pkg/llm`,
  `go build ./...` and scoped `git diff --check`: passed.
- Added the actual service consumer to `pkg/llm/outbound_contract_test.go` and the
  package documentation. Updated only its corresponding outbound inventory in
  `.env.example` and both environment-variable documentation locales. P5 still
  owns full upgrade/user docs and generated help synchronization.

All model tests used local fake HTTP responses; no real model/account, database,
commit or deployment operation occurred. These tests do not prove model semantic
reliability or the later transaction/UI integration.
