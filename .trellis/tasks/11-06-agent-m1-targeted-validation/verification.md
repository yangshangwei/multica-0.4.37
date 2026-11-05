# Targeted validation results — 2026-11-06

## Actual result
Targeted API and actual CLI checks pass. Two restricted-role Codex processes completed and their observed guided behavior passed independent review. The third leader process timed out at180s after correctly committing its delegation and partial-failure evaluation; process completion is a failure, not3/3 green.

| Layer | Actual evidence | Verdict |
| --- | --- | --- |
| New production-router matrix |29 named cases; independent29 plus focused4 subset after strengthening persisted-name assertion |Pass; no product source defect found |
| Compiled Multica CLI→real isolated API |20 cases, actual current templates/claims, JSON/identifiers/list/pagination/comment/readback and denied writes/foreign reads |Pass |
| Reporter real codex exec |165.622s, exit0;3 commands; scope-read/current-project list/one inert report comment; full recorded maps checked |Guided action+semantic sample pass |
| Reviewer real codex exec |45.256s, exit0;2 commands; current issue/read+one inert review; correct inclusive100 defect and99/100/101 cases |Guided action+semantic sample pass |
| Leader real codex exec |180.012s, timeout/exit124;3 successful commands including real mixed queued/blocked receipt and failed evaluation; no final message/turn.completed |Required business actions observed; process deadline FAILED |

Model execution: codex-cli0.166.0, existing configured gpt-6-astra, low reasoning, ephemeral private configuration, max3 sequential invocations, no harness/model retry beyond budget. Actual reported reporter/reviewer usage retained verbatim; leader final usage unavailable because the process did not complete. No monetary cost inferred from token telemetry.

## What was newly tested
Reporter filter intersections cannot widen under open_only, project_ids/include_no_project, aliases, paging or encoded forms. Runtime owner revocation/rebinding is tested separately from unchanged originator authority. Human name/prompt/autonomy edits and forged provenance fields cannot remove the new API ceiling; old/custom role controls retain compatibility.

Actual binary commands exercise complete service use, including full UUID project lookup without forbidden inventory. Prefix project lookup is intentionally denied. Allowed comments have exact agent author/source_task_id and durable dispatch_eligible=false. The new test file is the only product-tree change; no production implementation was modified.

Independent review repaired two test evidence weaknesses: persisted-name mutation needed an explicit assertion, and the initial private oracle omitted deletion/change checks. Original outputs remain; supplemental file-only oracles compare keyed recorded issue/task/comment maps and every decoded request/CLI command. No model was rerun to obtain a green result.

## Failures and incomplete evidence
- Initial detached CLI build was interrupted/gone with no binary or usable result; tracked second build succeeded. The first is incomplete setup evidence, not a failed product test.
- Restarted Docker container obtained a new dynamic port. Attempt against previous private allocation refused connection before fixture writes; retained failure, refreshed a new private allocation from verified container ID/label/port. No fallback/shared DB used.
- Leader exceeded fixed180s despite correct committed actions. It is not safe to infer the provider, model or transport root cause from absence of final output. No fourth call or increased timeout was used to hide this result.
- These prompts explicitly identify hostile quoted text as untrusted and provide CLI syntax. Results are guided samples, not evidence of general/unassisted injection resistance or statistically reliable model behavior.

## Cleanup and scope
Task-owned database `rnd_m1_targeted_api01` served synthetic legacy accounts; router packages used different freshly allocated owned databases. No real worker consumed queued leader work. Cleanup revoked4 credential rows for exactly3 trial task IDs, stopped the owned API/proxies, verified their ports free, and stopped the owned PostgreSQL container while retaining data/logs. Private upstream/DB credentials are excluded from tracked evidence; scan result is in cleanup.json.

No remote push, publication, production operation, full-browser rerun, hosted Go1.26 CI or NFR campaign. Existing four-path browser smoke was not repeated because production code was unchanged. Parent M1–M3 remains incomplete. See independent-review.md, deterministic-verification.md, live-cli-verification.md and the bounded source/raw evidence manifests.
