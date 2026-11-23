# Ordinary UI GREEN R01 fixture correction — 2026-11-19

**FROZEN STATIC PASS; new runtime body NOT_RUN.** WORK R05 corrects two observed fixture assumptions while preserving the complete genuine UI and protected-state assertions. Main owns a fresh whole GREEN run and final acceptance. Original GREEN R01 remains **0 PASS / 2 FAIL tops; 2 PASS / 2 FAIL / 2 NOT_RUN subcases**.

## Source and ownership

Campaign root is `/Volumes/artisan/.multica-acceptance/pay241-s3-ui-cas-20261119-u691amwe` (`C`). Source `C/source/client-ui-harness-work-r05` was cloned from `client-ui-harness-green-run-r01` with symlinks preserved. Of 3,646 recorded source entries, exactly one path changes, without mode changes:

`e2e/fixtures/collaboration-recovery-ordinary-ui.ts`

- Before SHA-256: `d8441e916aecc36a09725ed8e9f9108dbaceeef68b64c34f7a04df48197db977`
- Frozen R05 SHA-256: `fea2807c7c27c74bc9f7ea440081e58e04fb0dee1acc8312a18b1069bdd26fa9`

ROOT writes are only this report and its JSON companion. No product, shared recovery helper, spec, task metadata, historical source/result, Git or runtime state was changed. The existing 31 visible workspace links are retained: the 30 nonmobile links resolve to the unchanged frozen harness RUN R02 packages, and the unconsumed mobile link resolves to ROOT. This copy does not claim relocated package links.

## Failure and correction

**GET/PUT labels:** The retained Web journey completed both genuine field competition orders before its separate versionless API control failed. `IssueResponse.Labels` in `server/internal/handler/issue.go:103` explicitly documents that UpdateIssue omits the field while detail GET supplies the authoritative list. The original equality compared those different projections. R05 asserts the raw PUT response has no `labels`, records the detail result, requires GET `labels=[]`, then compares the entire parsed GET against the parsed PUT plus that exact empty label list. Every other field remains under exact equality.

**Parent reader credential:** Desktop's `openSession` calls `actor.login()` before each native session. That replaces the shared `TestApiClient` bearer, while the parent comparison previously read through that mutable client before and after the sessions. `informationAuthorityVersion` includes credential kind/expiry and password/PAT facts; `loadInformationState` hashes that authority into `information_workflow.context_version`. A changed reader credential can therefore change this view fingerprint without changing parent business state. GREEN R01 retained exactly that DTO difference while all ten protected SQL inventories matched.

R05 captures the original creator bearer once per subcase and uses a local direct GET closure with explicit Authorization for both parent reads. It requires HTTP 200 and parses `CollaborationIssueSchema`, records actor identity and only a SHA-256 credential fingerprint, and retains **full parent DTO equality including context_version**. It does not pass an Authorization override through `requestJSON`: `TestApiClient.authedFetch` would overwrite that header from its mutable token. The original reader token is never emitted or refreshed by this closure. Source and failed response evidence support this cause; the fresh complete runtime must verify the correction.

The genuine held picker requests, observed numeric revisions, exact 409 bodies, fresh explicit intents, request counts, native session cleanup, full ten-relation SQL equality and zero run/command/dispatch assertions are unchanged. The entire source prefix containing those helpers and assertions was compared byte-for-byte. No deadline, retry, field comparison or authority gate was weakened.

## Static verification

All commands ran once through `C/helpers/record-run.py` with closed `configs/ui-harness-static-r05.json`. Its dedicated HOME/TMP paths are outside Git; DATABASE_URL/JWT and model credentials are absent, both runtime body gates are zero. Only paths were relocated in the existing strict TSC, ESLint and discovery configs.

| Recorder label | Result |
| --- | --- |
| `e2e-cas-ts-r05` | Strict no-emit TypeScript exit 0; empty stdout/stderr |
| `e2e-cas-lint-r05` | All three explicit E2E targets exit 0 with `--max-warnings=0`; empty stdout/stderr |
| `e2e-cas-discovery-r05` | Exactly two tests in one file; exit 0; no test body |

Each result records no source, configuration or tool drift. The companion JSON contains complete command/result/raw hashes, source before/after hashes, unchanged assertion checks, workspace-link inventory and exact patch. No database, API, service, browser/native or Provider action was executed in this lane.

## Handoff

WORK R05 is frozen for Main's source-bound, newly allocated complete GREEN round. Retain GREEN R01's original failure, original real PUTs and source/config/database/raw. This static correction does not accept TC-16, R09 or full S3; both platforms must complete all six subcases, followed by source/resource and independent acceptance checks.
