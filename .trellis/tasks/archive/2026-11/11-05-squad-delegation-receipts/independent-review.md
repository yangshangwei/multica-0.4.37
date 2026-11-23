# Independent implementation review — M1-C1

Verdict: **PASS for immediate CLI and leader receipt consumption after two local fixes.** No remaining blocking finding in this slice. Persistent receipts, server-derived evaluation and real-model behavior remain outside this acceptance.

Reviewer: native `trellis-check` / receipts_review. Review performed in the isolated M1 checkout. Only the two receipt helper/test files and the owned mentioning skill were changed; no capability, queue, database, CI or other parallel source was edited.

## Findings fixed

1. `server/cmd/multica/cmd_comment_trigger_outcomes.go`: an identifiable malformed receipt was counted separately from a valid receipt for the same target, so one target could appear both handled and unconfirmed. A missing reason after queued produced “1 handled, 0 blocked, 1 unconfirmed”. The helper now records malformed identified targets in the same map with an unconfirmed sentinel; later receipts cannot restore a success. Regression covers malformed-before/after handled, malformed-after blocked, and a third receipt after contradictory statuses, through both actual add/update command functions.
2. `server/internal/service/builtin_skills/multica-mentioning/SKILL.md`: generic skill guidance required every partial-delegation caller to record squad activity, although that endpoint authorizes only the current squad leader. Qualified the instruction to a squad leader's delegation-only turn and explicitly excluded other agents. This preserves existing authority.

The exact failing regression precedes the helper fix: `receipts-independent-cli-red-01` retained44 named passes and9 failures (six leaf failures plus three parents), with no compile or environment failure. Original implementer failures and source manifests remain retained.

## Review findings and preserved boundaries

- Add/update call the helper only after a successful ordinary HTTP response, before table-mode return. The full decoded JSON remains unchanged; the helper issues no request and does not change the saved-write result.
- Missing legacy field and empty array keep current compatibility. Null/wrong container, malformed entry/field, unsupported target type, malformed canonical UUID, unknown status, missing reason and contradictory duplicates warn without panic. Target UUID casing is normalized for duplicate detection. Unknown reason strings with a recognized handled status remain compatible.
- Identical valid duplicates count once. Identified malformed or conflicting receipts poison that target for the whole response. Unidentifiable malformed entries remain unknown; the helper cannot infer their intended target.
- stderr output contains counts and fixed guidance only, so arbitrary reason text is not emitted as terminal controls. Exact data stays in JSON. No automatic POST/PUT retry occurs after a lost connection or invalid full response; existing transport/decode errors remain errors.
- Owner and guest leader briefs both require exact `(target_type, target_id)` matching against intended live-roster targets. They distinguish admission/coverage from started/completed work, keep partial/unknown outcomes visible, and do not claim saved comment readback can recover the original receipt.
- Existing single-comment, evaluation fallback, owner/guest status, human/integration done and double-trigger guards remain intact. Dynamic brief wording no longer promises a member will run or automatically wake the leader merely because the comment was saved.
- `.trellis/spec/server/squad-delegation-receipts.md` and its index entry correctly describe this limited scope. The spec's duplicate-target and unknown-response rules match the corrected implementation. Source maps point to the actual CLI, handler and test contracts. No schema/API/queue or dependency change belongs to D1.

## Verification

All commands use private HOME, preserved Go cache/module paths, GOMAXPROCS=2 and -p1. No real model executable/account, remote or production operation was invoked. Named counts include subtests; they are not whole-repository totals.

| Check | Result | Evidence |
| --- | --- | --- |
| CLI actual command receipt matrix and existing comment write tests |70 pass,0 fail,0 skip|`receipts-independent-cli-green-01.json`|
| Owner/guest protocol and roster on fresh owned DB `rnd_m1_receipts_review01` |10 pass,0 fail,0 skip|`receipts-independent-handler-01.json`|
| Injected runtime mention contract |4 pass,0 fail,0 skip|`receipts-independent-execenv-01.json`|
| Skill YAML/format and mention/issue contracts |28 pass,0 fail,0 skip|`receipts-independent-skills-01.json`|
| Go build for CLI, handler, execenv |exit0|`receipts-independent-build-01.json`|
| Go vet for CLI, handler, execenv |exit0|`receipts-independent-vet-01.json`|
| gofmt and owned diff whitespace |clean|`receipts-independent-format.json`|

Current owned-source hashes match `evidence/manifest.json`; original implementer hashes are preserved in `manifest-before-independent-review.json`. The implementer's retained raw log hashes were independently compared and all matched. Each new evidence JSON includes actual command, return code, named inventory and raw log hash; no passing skip is counted as behavior. Build/vet counts are intentionally zero because those checks are not test execution.

## Findings not fixed / verification limits

No remaining D1 defect was found. Missing expected-target detection is deliberately the leader's contract: a generic response consumer cannot know intended targets without changing the command interface. This review does not prove a model follows that contract. Ordinary comment writes still lack durable per-target recovery and request-id idempotence; `squad_leader_evaluated` remains a caller-authored activity, not server-derived delegation proof.

Not run: full repository/frontend checks, browser journeys, hosted CI/Release, paid or real-model behavior, provider execution or durable receipt recovery. No TypeScript changed; Go build supplies the applicable type check. Final integrated acceptance with the parallel capability lane remains the coordinator's responsibility. M1 as a whole and M2/M3 remain open.
