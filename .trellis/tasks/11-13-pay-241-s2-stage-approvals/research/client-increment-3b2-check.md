# S2 privacy + client3b2 independent check

Completed: 2026-11-14T07:48:02.553607+00:00

**Review complete after one privacy repair (B01). No open findings in this client boundary.** The corrected client3b2 is ready for integration. Whole S2, A01–A12, parent and live Web/Desktop acceptance remain open.

## Source and ownership

Input snapshot: `/Users/artisan/.multica/acceptance/pay241-20261113-01/snapshots/client-3b2-20261114T071911Z`. All **496 files**, **31 reported changes**, and **24 logs** match recorded hashes. The frozen wire is `a71d71570cf1f337d0f8c299e77e307e6d37516dd516eaa6e0882548c5669520`. The snapshot and original results remain immutable. Final live changes and hashes are recorded in the companion JSON.

Root expanded ownership to `stage-queries.ts`, its focused tests and the required code-state guard, and approved passing the existing `issue.source_workspace.id` into the internal R1 query helper. No HTTP field, direct-runtime gate, authorization model, server, spec or dependency changed.

## Findings (fixed)

### B01 / P1 — fresh R1 denial left cached source evidence available

**Before:** R1 could report `can_open:false, content_digest:null` for the current child while old R4 still claimed readable/current. `canPrepareCollaborationTest` remained true and a retained T4 log remained visible. A5/A6 already rejected the newly unreadable selection, but source cache and C1/QA display paths did not share that signal.

**Proof:** The frozen-snapshot diagnostic produced two failures (C1 eligibility and source readability) while20 existing tests passed. Added canonical Core regressions failed3 cases before repair; the Views regression failed because the private report body remained rendered. These original red logs are preserved.

**Repair:** `stage-queries.ts:31` receives the actual source workspace, joins the existing workspace generation and clears private R4/T4 caches before returning an authoritative R1 denial. `stage-presentation.ts:10` recognizes that explicit source denial; `stage-code-presentation.ts:24` stops rendering/authorizing from an older readable R4. The typed composer passes the existing issue source identity. Empty child lists remain unknown rather than a denial. Missing source identity refuses the read.

**Regression evidence:** same-workspace/cross-issue private caches clear; another workspace and public issue identity survive; old pending R4 responses cannot restore bytes; a fresh positive R1/R4 read restores availability. The existing typed-comment original key/subject and unsent text remain, with no automatic send. The plain-text report disappears before a secondary refresh. Reason-only returns and C08 recovery remain covered by the shared regression suites.

## Findings (not fixed)

None in this increment.

## Reviewed behavior

- **R1 child confidentiality and A5/A6:** Source-unreadable child requires explicit null digest; current nonempty A5/A6 reject retained old readable selections. Empty A5 remains valid. B01 adds the missing shared source-fence propagation.
- **T4 parser and bytes:** Bounded cases/logs, exact counts/outcomes/log indices, UTF-8 limits, strict inline/record identity, source-only null artifact and SHA-256 of returned log bodies are validated. Endpoint methods correlate issue/run/report and keep structured errors distinct. Plain-text renderer never executes text or turns paths/URLs into fetches.
- **T4 cache and authority:** R4/T4 share the source workspace generation. Ambiguous report404 erases private source data and rereads issue authority; it does not itself revoke independent issue rights. Late errors/success and other workspaces are covered. Cursor chains reject repeats/mismatches/loops.
- **C1 reachable typed-comment path:** Existing protected mention-targets.can_submit_commands enables the path with read/comment, v6, assisted/current QA and valid review/source. Global submit_command remains false. Preparation updates the existing comment draft, preserves its text/files, requires target preview/explicit confirmation, and does not send or rewrite an existing subject. Subject changes invalidate preflight identity. Original-key recovery preserves original subject; receipt does not claim execution or test success.
- **QA exact full report:** The human selects a report; approval waits for matching full T4 issue/run/attempt/candidate/source/artifact and usable passed evidence. Runtime-reported outcome remains distinct from execution applicability and human approval. Failed/unsettled evidence can be returned, including reason-only return with withheld source.

## Verification

- **Final focused Core:**8 suites /70 passed, no failures/skips.
- **Final affected Views:**5 suites /64 passed, no failures/skips.
- **Final Core/Views typecheck:**pass.
- **Lint across original changed files plus expanded files:**pass, zero findings.
- **Other independent pre-repair checks:**56 Core tests and19 Views tests passed; these are not summed into final totals.
- **Historical owner evidence:**477 Core/63 Views and all24 command exits/hash bindings verified.

The frozen diagnostic was bundled from21 snapshot modules, three pure dependencies retrieved from the recorded HEAD, and one reviewer counterexample module. The three supplements match current bytes; the lockfile matches the recorded HEAD. External versions, source hashes and bundle hashes are recorded. This diagnostic remains red against the immutable old snapshot; final repository regressions are green.

## Reviewer-changed files

- `packages/core/collaboration/stage-code-presentation.ts` — `f7bca16382ef8a311becb17b02e8e683696d8e9d954cf139f3b6d38ddf4443f3`
- `packages/core/collaboration/stage-presentation.ts` — `f2769da3900f9fa5f5a657ae68444e2c013479dba2509aec4d8ec3dd016f2685`
- `packages/core/collaboration/stage-queries.test.ts` — `a1609214b42bcf72f2d9b9f20a86ad6a4906f8584a26f705b6cd220f763156c8`
- `packages/core/collaboration/stage-queries.ts` — `d59a3583b030b641f640e781ba30690a50907028c007f1b4beef7ec084890d8b`
- `packages/core/collaboration/stage-test-presentation.test.ts` — `c189c2874235ac15d02afcb654a4dfd830f02957e1b9692579c057f25c87021e`
- `packages/views/collaboration/comment-composer.tsx` — `cfce6a82e9bf2e783c32bb96ebd6b80ae676d1348100dad56548b03e33bc154e`
- `packages/views/collaboration/stage-test-flow.test.tsx` — `e6048b370de187727977e8c017864a3e272a6dac228869014e0c60c885a803ac`

All exact commands, failures, passes, final source bindings and original log hashes are in `client-increment-3b2-check.json`. No actual API, database, browser, model, platform service, full build, Git write or log/path execution ran. Root owns spec synchronization and final joint acceptance.
