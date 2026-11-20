# Diagnostic r03 cause and minimum repair review

**Confirmed P1: the client omits six frozen2c blocker/refusal codes, hiding the legal first Code review action.** Product repair remains with the client owner. The reviewer changed no repository/frozen source and ran no DB/API/browser/E2E case.

## Findings (fixed)

None. The enum extensions used for the cause test exist only in the private Node process; they were not written to product source.

## Findings (not fixed)

- **S2-R4-C01, P1** — `packages/core/collaboration/stage-schema.ts:160` and `:570`, `packages/views/collaboration/stage-labels.ts`, and English/Chinese `stage.refusals` omit the six wire codes documented at `wire-contract.md:1227`. The raw browser R4 returns200 with current candidate and passed CI. Its release/rollback entries contain two `stage_execution_unavailable` blockers and one `stage_rollback_observation_unavailable` blocker. The parser changes all three to `unknown`. `stage-code-presentation.ts:37` checks blockers across every signable entry and returns at38, before the legal development business-owner advance at71.
- Repair exactly the documented six enum values and stored409 status entries, the six explicit label cases and both locales. Keep the derive known guard and future-unknown behavior. Add one canonical Core composed R4 parse-to-action regression plus the unknown/future control; schema/status vectors belong in the existing Core canonical layer. No API/server/decoder, forced UI action, timeout, actor or dependency change is needed.
- The implementation owner must provide the actual source patch and fresh red/green/static/build/full-runtime evidence. This lane is read-only for product files.

## Causal verification

A single pure Node command completed18 checks, exit0, using the original browser CI-passed GET body SHA256 `9a96cfa00964a9b8f7f68ada672c57022d2c45dfc15a8b5a62e17343aedd9923` (2614 bytes), captured before the original failed click. Its raw text hash and decoded object match. The additional explicit candidate GET `2929f0f63d7ddfb3e165192b433997388bc95b552f7c87f7c8bd25caca922106` reproduces the same cause.

| Control | Result |
| --- | --- |
| Current source parser + unchanged derive | Writable/readable;3 unknown blockers; advances empty |
| Only recognize exact wire six codes in memory | Exactly one advance to code_review; no approval/return |
| Future unknown blocker or stage | Actions remain closed |
| Failed/expired CI, stale candidate, wrong actor, source denial, revision mismatch | Actions remain closed |
| Release/verification stages across five roles with absent execution/approvals | No release/rollback approval or advance |
| Six documented stored codes with fixed409 map |409 accepted as refusal; contradictory502 refused |

The pure issue context is the retained actual creation response with only the current revision/lifecycle observations from captured stage GET. It passed the strict issue parser and in-sync/managed checks. This explicitly controlled context isolates the parser defect; it is not a later captured collaboration-issue GET or proof of a successful browser action. The derive source is unchanged.

## Verification

- Lint/TypeCheck: existing diagnostic owner static PASS evidence was independently checked earlier; the proposed product repair is not yet implemented or newly checked.
- Tests: **18 pure cause/negative controls PASS**. These are diagnostic assertions about old and in-memory hypothetical behavior, not a production regression-green claim. New DB/API/browser/E2E runs by reviewer:0.
- All3358 frozen files and1961 links remain unchanged after replay. No hypothesis product source was written. Every loaded Core source dependency digest matches.
- Spec: after owner repair, record the enum/status-map/explicit-label/both-locale/composed-R4 touch points in existing frontend stage guidance. Preserve peer spec content.

The single diagnostic remains0PASS/1FAIL (denominator1), increment to four-case acceptance0. Both original full runs remain0PASS/1FAIL/3NOT_RUN (denominator4). **A11/A12 remain OPEN.** Entry DB checks still await the main session’s resource permit.

Detailed report: `diagnostic-r03-cause-independent-check-r01.json`, SHA256 `7b4e3e049300250951d94ecc1faffab03186d009f37513e95a7489a706c1ceb0`. Replay result SHA256 `4af4bf61945ed4899e9158e67ea19308022c5db83efbc9e27c41773efabc0116`; retained stdout SHA256 `60ca717f620b4a53770c3b3149d13855a20866497b0a645c9e16d7555907bbfb`; stderr is empty. References checked:24.
