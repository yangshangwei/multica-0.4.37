# CCG v14 independent offline review

Verdict: **pass-with-notes**, zero confirmed runner/product defects; **not ready to execute**. Scope is the current unready preparation on `d0b132fff0c477eb2d744933e13db730240231aa`. This is not a final execution review, a passing CCG observation, or #7 acceptance. Reviewer: `/root/ccg14_admission_review`.

All thirteen prepared files and thirteen retained v13 originals match their manifests. All eleven selected source files, four pinned tools, auditor and environment parser match. Both executable runners are byte-identical to v13 after the registered candidate/version/origin substitutions; their assertions and commands were preserved. The four actual permission templates remain `ready_to_execute=false`, with matching runner/input hashes and unset allocation fields.

## Findings (fixed)

None. No executable, permission, product or task record was edited. Only this report and `review.json` were written.

## Findings (not fixed)

`source-inputs.json` still has a null `audit_review.sha256`. The actual current v14 independent review exists at the registered path with SHA-256 `7b7047c02fba74dbc2a080581eb8adcf99922a2aa6a89d43e2942bbc041497c3`. This is an expected pending binding, not a bypass: both runners refuse at line50 before SQL/test. Root must preserve initial preparation, pin the accepted review, refresh all four still-false input hashes, then obtain a new exact-byte CCG review. This report cannot approve the changed input/template bytes in advance.

The observed v14 state remains `running` with no final exit. Actual exit0, persisted final audit pass, complete named/explicit-skip inventory and independently observed source/API/DB/process ownership must precede resource release and CCG allocation.

## Actual guards and root responsibilities

Both runners check labels/raw permit symlinks/owned location at30–36, exact root booleans for fullcheck release/inventory/DB freshness and fake-only scope at37–39, candidate/runner/input/tool/source hashes at40–50, and the registered final auditor at50–55. The auditor requires a complete run and exact integer exit, rechecks tracked bytes/modes, tools/workspaces/CLI/Desktop/config/log, then exposes `wholecheck_passed`; a nonzero exit never passes CCG admission. This revalidation happens before SQL or test commands. Missing or changed final-audit inputs fail closed. The runners rerun the auditor rather than consume an independently persisted final-audit JSON; root still retains and checks that actual final report.

At56–70 the runners enforce owned0700 Git-free temp, explicit loopback matching DB URL/name and actual `database|764_...` migration identity. The identity query cannot prove the database was newly created or distinguish a reused matching database. Root must verify allocation history, zero sharing and actual migration identity separately. Output directories/results/logs use fresh/exclusive creation, preserving failed history.

CCG2 at91–161 adds contained bound inputs, reviewed data parsing, password/managed/knowledge flags, no model config, loopback origins, actual API commit/PID/config/CORS and renderer-byte checks, plus pinned executable fake daemon/provider/CLI. Registry manifests, actual API startup flags, production Web build/rendered API/WS URLs, distinct API/Web/Redis resources, account identities and binary build records remain root observations. `root_permission_record` and `build_record` contents are not independently parsed by these runners. The full attachment's account/contribution/run/authorized/observed/reaped/sealed identities must be independently inspected; the runner itself checks only its selected ON/OFF fields. These are the documented trust boundaries, not newly discovered implementation defects.

The retained `ccg-admission-template.py.txt` is reference material with older path ordering. Use the actual reviewed `run-ccg1.py` and `run-ccg2.py`; do not regenerate from that reference.

## Preserved acceptance contracts

CCG1 retains the exact anchored composite test, `knowledgeintegration`, `-p=1 -parallel=2 -count=1 -json -timeout=5m`; race only adds `-race`. Acceptance requires exit0, exactly one named run/pass and zero fail/skip, plus the unchanged physical fact. The actual Go test preserves two physical starts, human-approved version2 in actual stdin, superseded input absence, third real authorization409 before any third process, prior run/exposure and original receipts.

CCG2 retains the single deployed spec, one worker, zero retries, original timeouts and line/JSON reports. It requires exit0, expected1 and unexpected/flaky/skipped0, no report errors and one attached deployed evidence record with correct ON/OFF input/events and zero Provider. The underlying spec retains exact candidate/contribution/source/account/run identity, one actual fake visit, DB event counts and the observed/reaped/authorized/sealed proof ledger.

## Evidence and verification

The four retained raw refusals each have exit2 and matching actual log hashes. All eighteen early-admission counterexamples and their script/profile hashes were independently checked. They cover early clauses only; deeper auditor/DB/deployment clauses were traced statically here. No new dynamic controls, lint/typecheck, Go, Playwright, DB, API, Web, renderer, Redis or Provider execution was run. AST parsing succeeded for both Python runners. All relied-on paths and hashes are in `review.json`. Knowledge allocation/calls stay0/0; production injection remains default false.

## Next handoff

After actual v14 success/final audit/inventory/root release: pin the final review hash, rehash inputs and still-false templates, record new exact-byte review, then allocate four fresh DBs and two separate registries plus current fake binaries and a separate renderer/Redis. Confirm real manifests/config/ports/identities and issue separate new permits/labels. Run CCG1 normal→race and CCG2 OFF→ON, retaining each independent result and full attachment for #7 terminal review.
