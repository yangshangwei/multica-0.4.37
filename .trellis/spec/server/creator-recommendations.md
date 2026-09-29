# Creator recommendation boundary

## 1. Scope / Trigger

Human members explicitly request task-creation assistant advice. This is a read-only LLM consumer; it never creates issues/tasks, dispatches an agent, or changes assignees/defaults. Project execution-squad selection remains a separate contract.

## 2. Signatures

- `POST /api/issues/recommend-creators` handled by `Handler.RecommendIssueCreators`.
- Request `{text:string}` with workspace pinned by authenticated request headers.
- Response `{recommendations:[{actor_type:"agent"|"squad",actor_id:UUID,reason:string}]}`, zero to three items.
- Internal model payload `{text,candidates:[{ref,actor_type,description_excerpt}]}`; output `{recommendations:[{ref,evidence}]}`.
- `loadedInvocationDecision` is the shared pure predicate used by the existing single-object query wrapper and the batch loader; callers establish human attribution and membership.

## 3. Contracts

Reject task-token/agent principals before catalog/provider access, require a human workspace member, then filter active user-kind agents with runtime bindings and current invocation grants. Inventory visibility/admin status does not grant private invocation. Squads require an eligible same-workspace leader and use their own description.

Load invocation targets in one batch; never perform550 individual permission lookups. Scan all eligible saved descriptions. At most40 described actors may be sent: small catalogs all participate, larger catalogs require positive English-word/CJK-bigram evidence with stable type/ID ties. Select a relevant continuous excerpt of at most600 runes, including matches late in long squad descriptions. Agents currently have a255-character database description limit; the larger excerpt budget also supports squads.

Text≤20000 runes, inbound body≤128KiB and serialized user-data JSON≤128KiB. Bound output to2000 tokens/32KiB, use a30-second context deadline and the existing bounded description-assist admission pool. Provider transport retries follow existing LLM configuration within that deadline; clients do not automatically retry.

Only task text, excerpts, actor types and temporary refs go upstream. No actor name/ID fields, instructions, runtime credentials, template claims or attachment file contents. Outbound consumer inventory, .env.example and both environment-variable docs must stay synchronized.

Evidence must be an exact nonblank substring of the actual supplied excerpt,≤240 runes; refs must be unique, known and at most3. Map refs only through the authoritative request map. Before responding, recheck membership and the current eligible catalog; drop objects whose description changed, even if the old quote remains as a substring.

## 4. Validation / Error Matrix

| Condition | Result |
| --- | --- |
| Agent/task-token principal |403 before model/catalog work |
| Nonmember or wrong workspace | Existing member gate401/404 |
| Invalid/extra request field, trailingJSON, blank/oversized text |400 invalid_request |
| Unconfigured LLM |503 ai_unavailable; no upstream call |
| Admission full |429 ai_busy with Retry-After |
| Cancelled context | Stop work and release slot; no suggestion write |
| Deadline |504 ai_timeout |
| Upstream failure |502 ai_generation_failed, sanitized |
| Unknown/duplicate ref, false evidence, malformed/over-limit output |502 ai_invalid_output |
| No eligible/evidence-supported candidates |200 with[]; no arbitrary first-N fallback |
| Permission/archive/runtime/description change during generation | Drop invalid candidates; DB failure is not an empty result |

## 5. Good / Base / Bad Cases

Good: a relevant squad late in a550-item directory enters the bounded shortlist and returns its own saved evidence. Base: an empty result leaves manual selection available. Bad: using admin inventory permission, inferring ability from names, or passing arbitrary model UUIDs through to a client.

Large-catalog lexical retrieval can miss synonyms and cross-language descriptions; do not market the result as a proven global optimum. The model recommendation never grants execution authority; normal submission still applies current permission/readiness/CLI gates.

## 6. Tests Required

`issue_creator_recommendation_test.go` covers membership/actor gates, read-only task-count evidence, private invocation, squad identity, exact/false evidence, post-generation changes, cancellation/admission/timeout, serialized budgets and tail retrieval. Existing TestCanInvokeAgent cases protect the deliberate system workspace exception and member/team restrictions. Use the explicit isolated database and httptest provider; never run real agent CLIs.

`pkg/llm/outbound_contract_test.go` must identify this consumer. E2E uses a deterministic local provider and real API, plus real Electron renderer with native services isolated.

## 7. Wrong / Correct

Wrong: slice the first40 database rows, then ask a model who is best. Correct: filter invocation eligibility, scan all descriptions, bound the evidence shortlist, validate opaque refs and recheck after generation.

Wrong: accept any quote still found in a modified description (a new negation may reverse its meaning). Correct: require unchanged saved description as well as exact evidence membership before returning/adopting.
