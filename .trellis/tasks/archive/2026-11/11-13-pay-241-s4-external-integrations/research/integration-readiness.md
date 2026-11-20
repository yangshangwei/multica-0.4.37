# Research: S4 external integration readiness

- Query: Map existing adapters and precise inputs, authority and evidence for E01–E07 without contacting accounts or targets.
- Scope: internal; static source inspection and external execution preparation only.
- Date: 2026-11-14
- Active task: `.trellis/tasks/11-13-pay-241-s4-external-integrations` (explicit dispatch destination; injected S2 context is background).
- Result: **E01–E07 NOT_RUN.** No network, accounts, credential stores, models, databases, Git commands or external targets were accessed. No product files were edited.
- Source identity: `integration-readiness.json` contains the checkout path and SHA-256 of inspected source/contracts. These identify local bytes, not a release commit or running API. Concurrent S2 edits require a fresh source binding before execution.

## Findings

### Preparation and inherited evidence

No S4 research file existed at the beginning of this pass. This report extends S2 `research/server-gates.md` §4 and `research/verification-plan.md` §3/8. Controlling requirements are S4 `prd.md`, `design.md`, `implement.md`, and S2 `research/wire-contract.md` §8/10/13 plus `design.md` F05.

Existing GitHub/VCS implementations provide authenticated ingress, PR mirrors and CI observations. Their existence does not prove this task has a repository, matching build or deployment target. At capture, S2 increment 2 is still marked frozen/not served in `wire-contract.md:731`, `:999`, §13. The inspected router exposes increment-1 stage routes at `server/cmd/server/router.go:1702`; no stage deployment/rollback fact registration symbol was found in inspected handler/service/collaboration/SQL sources. Recheck the S2 owner's finished increment before binding S4 to an implementation.

S2 `verification-plan.md:62` reports empty root `.env` `GITHUB_APP_SLUG`, `GITHUB_WEBHOOK_SECRET`, `GITHUB_APP_ID`, `GITHUB_APP_PRIVATE_KEY`, and no payment/release keys in its checked root/process inventory. This pass did not reopen dotenv files. That observation does **not** inventory encrypted connections or prove all accounts/secret stores/environments absent.

### Existing-adapter source map

Paths are repository-relative; symbols identify implementation, not runtime availability.

| Source / symbol | Existing behavior and use |
| --- | --- |
| `server/internal/handler/github.go:365` — `isGitHubConfigured`, `isGitHubRepositoryBrowseConfigured` | Connect requires app slug + webhook secret; repository browsing requires app ID + private key. Capabilities are separate. |
| `server/internal/handler/github.go:749` — `ListGitHubInstallationRepositories` | Resolves local installation UUID, verifies workspace ownership, lists remote repositories under the installation. Use only after a real target is supplied; listing contacts GitHub. |
| `server/internal/handler/github.go:1058`, `:1098` — `HandleGitHubWebhook`, `verifyWebhookSignature` | Existing raw-body HMAC ingress at `POST /api/webhooks/github`; PR events mirror records, CI events trigger an API refresh. |
| `server/internal/handler/github.go:1266`, `:1562` — `handlePullRequestEvent`, `mirrorPullRequestForWorkspace` | Installation/repository/PR/head identity and workspace fanout; qualifying title/branch/closing references create issue links. Workspace clone-repository registry is deliberately not the webhook subscription allowlist. |
| `server/internal/integrations/ghsnapshot/client.go:87`, `:162`, `:216` — `NewClientFromEnv`, `mintInstallationToken`, `graphQL` | Existing GitHub App auth and API client; requires actual installation permissions. |
| `server/internal/integrations/ghsnapshot/snapshot.go:21`, `:139` — `prSnapshotQuery`, `FetchPRSnapshot` | All check-context pages, 100/page, maximum100 pages; rejects changed head, missing/repeated cursor and overflow. Null rollup means no checks. |
| `server/internal/integrations/ghsnapshot/refresh.go:365` — `Manager.applySnapshot`; `server/pkg/db/queries/github_snapshot.sql:19` | Current-head guarded PR snapshot plus full per-check replacement in one transaction. This protects asynchronous snapshot writes, not ordering of PR webhook writes. |
| `server/internal/handler/vcs.go:159`, `:276` — `ConnectVCS`, `RotateVCSConnectionWebhook` | Deployment switch/encryption gate; validates provider token, encrypts token/secret, returns new webhook secret once. These are mutations that change trust, not discovery probes. |
| `server/internal/handler/vcs_webhook.go:95`, `:313` — `HandleVCSWebhook`, `mirrorVCSCIStatus` | Connection UUID selects workspace/provider/secret; normalized PR/CI ingestion and timestamped status upsert. |
| `server/internal/integrations/vcs/vcs.go:97`, `:117` — `CIStatusEvent`, `Provider` | GitLab, Forgejo, Gitea registry. CI shape has SHA/context/state/URL/time but no repository, pipeline, attempt, job or native check ID. |
| `server/internal/integrations/vcs/gitlab.go:41`, `:76`, `:160` | Shared-token ingress authentication, namespaced MR repository/head parsing, pipeline collapsed to `gitlab/pipeline`. |
| `server/internal/integrations/vcs/forgejo.go:49`, `:149` | Forgejo/Gitea HMAC and commit-status parser; same repository-less normalized CI boundary. |
| `server/pkg/db/queries/github.sql:22`, `:45`; `server/pkg/db/queries/vcs.sql:15`, `:58` | Reconnect/permission restoration and VCS reconnect/secret rotation advance trust revision. GitHub display enrichment alone does not. |
| `server/internal/service/delivery_completion_webhook.go:21`, `:128`, `:187`, `:215` | `DeliveryWebhookProof`, `FreezeDeliveryWebhookConsumerInTx`, `checkDeliveryWebhookProofInTx`, `CheckDeliveryWebhookConsumerInTx`: current trust/signature and exact ordinary-completion binding under locks. Reusable boundary pattern; not a stage deployment writer. |
| `server/internal/service/issue_status_write.go:26`, `:63` — `WriteIssueStatus` | System PR-merge completion excludes collaboration roots; merged PR does not substitute for collaboration acceptance. |
| `server/internal/handler/cloud_billing.go:16`, `:520` — `HandleCloudBillingStripeWebhook` | Multica Cloud credit/subscription proxy. Raw Stripe bytes/header forwarded to Cloud; not the PAY-241 order/payment/fulfillment business system. |
| `server/internal/handler/agent_artifact.go`; `.trellis/spec/server/controlled-artifact-execution.md:5` | Controlled local-synthetic tar packager with frozen approval/receipt and durable uncertainty; not a release/rollback adapter or remote artifact registry. |

### E01 repository/CI limitations and concrete risks

**GitHub repository binding:** fetch address is remote installation ID + owner + repository name + PR number (`ghsnapshot/refresh.go:24`); local trust also needs installation UUID/workspace/revision. Capture both installation IDs. PR conflict key is workspace + owner + repository name + PR number (`github.sql:126`), not stable remote repository ID. Retain actual remote repository ID/path and establish rename/transfer/reconnect semantics; a URL alone is insufficient provenance.

**GitHub build identity:** the query/table retain check name/status/conclusion/details URL or legacy context/state/URL, not native run/job/attempt IDs or artifact digests (`snapshot.go:34`, `:47`; `github_snapshot.sql:43`). E01 needs additional native run/job identity, workflow definition revision, artifact digest and verifiable correspondence to the exact head. A local check ordinal is not a provider run ID. The snapshot does not independently enumerate native branch-protection required checks; obtain actual required-check/merge rules without changing them.

**Strict CI differs from display:** S2 F05 requires current PR/head = snapshot head; a complete nonempty set; DB-clock observation age0–30minutes; every check explicitly `completed/success`. UI counts include neutral/skipped as passed (`github.sql` check aggregation); do not consume them as approval evidence. `normalizeNode` drops unknown/invalid nodes (`snapshot.go:215`), so completed pagination alone cannot prove unsupported returned check types were represented. Preserve response schema/version and treat unrepresentable checks as an evidence gap.

**Static risk — old GitHub PR redelivery:** `UpsertGitHubPullRequest` unconditionally overwrites head/state/`pr_updated_at` (`github.sql:126`), and `mirrorPullRequestForWorkspace` has no newer-row early return. VCS protects both SQL and derived link writes (`vcs.sql:71`, `vcs_webhook.go:223`). Therefore old authenticated GitHub PR events can overwrite newer mirrored heads in this source. The snapshot guard does not repair that write. E01 must deliver a newer head then replay a retained older event and check that current head/approval eligibility cannot revert. Actual old-approval revival depends on completed S2 consumers and is **not runtime-proven** here. This risk was reported to root; no source fix was made.

**VCS remains unsupported:** `vcs.sql:107` joins connection + head; `:171` upserts connection + SHA + context with no repository. Cross-repository equal SHAs can collapse. GitLab merges all pipeline types/attempts into `gitlab/pipeline`; merge-train/merged-results SHA may differ from MR head (`gitlab.go:173`). `skipped` becomes passed (`:194`); Forgejo `warning` becomes passed (`forgejo.go:173`). These display simplifications cannot satisfy S2's strict CI gate.

After a real provider/version is supplied, the smallest viable VCS extension needs instance/connection generation + stable repository ID + source head + native pipeline/run/job/check/attempt identity, original result semantics, repository-scoped uniqueness/ordering and complete-set provenance. Keep old unbound rows unsupported; do not guess their repository or relabel them verified. No provider/API/schema is selected by this research.

### Callback authentication, replay and idempotency

| Existing ingress | Authentication / event identity | Limitation |
| --- | --- | --- |
| GitHub | `X-Hub-Signature-256: sha256=<hex>`, raw-body HMAC constant-time comparison; `X-GitHub-Event`; PR proof uses `X-GitHub-Delivery` or `sha256:<body digest>` (`github.go:1071`, `:1098`, `:1301`). | Missing config503, invalid signature401.202 only acknowledges dispatch; ignored types and logged parse/storage failures can also finish202. It is not domain success. |
| GitLab | Constant-time nonempty `X-Gitlab-Token` comparison; `X-Gitlab-Event`; PR proof uses `X-Gitlab-Event-UUID` or body digest (`gitlab.go:27`, `:41`; `vcs_webhook.go:148`). | This is shared-token authentication, **not body HMAC**. Do not call the locally computed hash a provider signature. Register actual instance/version/transport authority. |
| Forgejo/Gitea | `X-Gitea-Signature` raw-body HMAC-SHA256, bare hex (also tolerates `sha256=`); `X-Gitea-Event` with GitHub header fallback; PR proof uses `X-Gitea-Delivery` or body digest (`forgejo.go:32`, `:49`). | Authentication does not supply missing CI repository/attempt identity. |
| Stripe Cloud billing relay | Exact body + `Stripe-Signature` forwarded to Cloud `/api/v1/webhooks/stripe`; header presence check,1MiB cap, optional IP limit (`cloud_billing.go:520`). | Cloud owns full verification and billing effects. This route is not selected as an S4 payment adapter. |

PR/link upserts prevent duplicate mirror/link rows. VCS ignores strictly older provider timestamps, but equal timestamps can replace and missing/unparseable timestamps fall back to ingestion time. Latest-state tables are not immutable delivery histories or proof of exactly-once business handling. Preserve real delivery attempts and audit records separately.

M1 rechecks current trust/signature under transaction locks (`delivery_completion_webhook.go:187`); `FinishDeliveryMutationInTx` checks and persists matching provider event ID/digest (`delivery_completion.go:319`, `:376`). That protects an exact ordinary-completion authorization. It is neither payment deduplication nor stage deployment proof. Callback delivery ID, payment-create idempotency key, order-transition dedup key and fulfillment-effect key are different domains.

### S2 release/rollback fact handoff

The frozen public contract already separates authorization and execution:

- A6 candidate binds child, local PR ID/provider/head and optional controlled artifact approval; F05 additionally binds lifecycle, requirements, child semantic version, configured implementer and repository/trust generation.
- A7 code review consumes CI digest; QA consumes typed test report + exact code-review decision; release consumes artifact digest/environment + exact QA decision; rollback consumes deployment ID/target artifact digest + that deployment's release decision (`wire-contract.md:774`).
- Consumption rechecks actual signer/account/participant generations, credentials, configured role/policy revision, finite expiry at DB time and exact current evidence (`:733`). Approval creates no deployment.
- A9 verification requires release approval **and** recorded external deployment (`:834`). R4 execution facts expose confirmed `succeeded|failed` with operation/environment/artifact/previous artifact/approval/time (`:900`). Unknown/pending is neither a success nor a fabricated definitive failure.
- §10.2 requires stage evidence and exact delivery binding at submit/complete/R38, alongside existing M1. Missing execution and unbound delivery keep completion blocked. An original operation's historical receipt is not a new execution authorization.

Before execution, S2 2c and S4 owners must freeze the actual **internal** registration/consumption symbol/types: trusted integration ID/generation, adapter/version, external operation/original request key, environment, root/candidate/lifecycle, exact approval/consumption, before/after digests, outcome/observation revision/time, retained evidence hashes and current-read authority. Register immutable, same-operation-idempotent facts; changed-content reuse must fail. No public human/Agent request may supply its own `verified` flag. These are required handoff fields, not invented endpoints or claimed existing symbols.

Two unresolved design inputs were reported to root:

1. F05's last paragraph binds rollback approval to a metric record, but §8.3 has no metric ID/digest field. S2 2c must define its internal typed reference or review an explicit contract amendment; free-text reason cannot become connector provenance. This report does not edit the frozen public contract.
2. Define where pre-call authority consumption, durable external intent, pending/unknown result and original-key reconciliation live. R4's public confirmed-fact projection does not define this execution ledger. Query original external operation after timeout; do not call again on assumption of failure.

At capture no stage execution-fact registration symbol was found. Generic agent execution notes, installation `MULTICA_DEPLOYMENT_ID`, Cloud node operations and synthetic tar receipts are not substitutes. S2 isolated synthetic facts prove platform contracts only and cannot enter S4 real PASS evidence.

### Input and authority register

These are intake IDs, **not environment variable names**. Named people, real environment names/URLs/IDs and grants remain unresolved. Root coordinates; it cannot invent business approvers.

| ID | Inputs and responsible role | Current status |
| --- | --- | --- |
| I01 source/runtime | Coordinator: exact source/API/Web/Desktop/CLI versions, artifact/file digests, run root, owned environment ledger, actual workspace/project/root/child IDs and candidate/lifecycle/revisions. | No real S4 run bound. PAY-241 scenario label does not rename existing PAY-1 evidence. |
| I02 repository | Repository owner/admin: selected provider/instance/version, stable remote repository ID/path, local installation UUID + remote installation ID or VCS connection UUID/trust revision, permitted test branch/PR namespace, native rules, scoped read/replay and cleanup authority. | Existing adapters, no target verified. No code auto-merge/protection bypass authorized. |
| I03 CI/build | CI owner: workflow revision/required checks, native run/job/check/attempt IDs, source-vs-merge SHA semantics, immutable artifact digest and retention, read/replay/failure controls. | Not supplied; VCS repo and GitHub native-run binding gaps. |
| I04 payment | Payment owner: actual system/provider, official sandbox API/version, test merchant/tenant/account/endpoint/instrument, amount/currency, create/pay/query/compensation contracts, idempotency scope/TTL, callback/replay rules, authoritative charge/capture ledger reader. | Not supplied; no live funds; Stripe is not selected by its presence in the repo. |
| I05 order/fulfillment | Business and messaging owners: real services/version, order-create/read/state contract, payment linkage, durable transition ledger, fulfillment effect IDs/ledger, consumer dedup scope, queue/topic or equivalent, failure/recovery controls and compensation policy. | Not supplied; building an in-repo commerce service is outside scope. |
| I06 callback/time | System owners: reachable task callback/routing, signing-secret reference/generation, valid redelivery facility, event/attempt identity, restricted exact-byte storage, clock source/skew, monotonic measurement, permitted45s delay point/deadline. | Not supplied; no tunnel, webhook registration or secret rotation executed. |
| I07 release | Nonproduction operator: named environment/URL, actual release/rollback API or CLI/version/contract, current deployment, new/previous allowed digests, scoped executor, immutable operation query, idempotency/reconciliation and health checks. | Not supplied; no general release adapter in identified boundary. |
| I08 metrics | Observability/operator: real time-series source/query/labels/window, baseline/threshold/rule version, allowed fault injection/restoration, original anomaly samples/time/hash, approved rollback target and validation. | Not supplied; metric-to-approval binding unresolved. |
| I09 people | Project/business owner: real implementer/reviewer/QA/release approver/creator IDs, source membership, role generations and credential expiry; QA/release distinct; test versus formal signer labels. | Names/grants unresolved. Test-account signatures are simulated human; formal sign-off requires designated people. |
| I10 S2 boundary | S2 2c + S4 implementer/checker: actual internal symbol/types/migration/version, integration/metric binding, unknown-operation recovery, evidence storage/read policy, independent evidence review. | Frozen intent, no runtime proof at capture. |

Known configuration names to verify only after targets are provided: GitHub `GITHUB_APP_SLUG`, `GITHUB_WEBHOOK_SECRET`, `GITHUB_APP_ID`, `GITHUB_APP_PRIVATE_KEY`; VCS `MULTICA_VCS_INTEGRATION_ENABLED`, `MULTICA_VCS_SECRET_KEY`, `MULTICA_PUBLIC_URL` (`router.go:443`, `:448`, `:1224`). VCS connect request uses `provider`, `instance_url`, `access_token`; encrypted stored connections are not dotenv inventory. Secretbox master key is32bytes/base64; replacing it is not webhook-secret rotation (`server/internal/util/secretbox/secretbox.go:1`). Do not change an existing master key to enable a test. Payment/release/metrics config names are **unknown** pending actual contracts. `MULTICA_CLOUD_URL` configures existing Cloud proxy, not a PAY-241 sandbox. Store secrets only in private config; reports retain reference names/scopes and nonsecret identity, never values.

### E01–E07 execution and evidence matrix

Every case needs run/case ID; real target/version; source/artifact identity; actual actor; role/policy/lifecycle/candidate versions; original request/operation IDs; UTC times; observation deadline; retained redacted request/response/log references and hashes. Keep original-byte and redacted-body hashes distinct; exact bytes belong in restricted storage. Never publish signing tokens. Failures and recovery attempts remain linked, not overwritten.

| Case | Required inputs | Authorized sequence once targets/contracts are supplied | Required authoritative evidence / pass criterion | Status |
| --- | --- | --- | --- | --- |
| E01 / TC06–08 | I01–03,I06,I09–10 | Use an owned branch/PR and real CI. Bind candidate; exercise missing/pending/failed/stale checks, review/QA separation, new head, old-head replay, duplicate events, connection removal/replacement/rotation within explicit test scope. Re-read real remote head and stored evidence. | Remote repository ID/path + installation/connection generation; branch/PR/head; full native check/job/attempt set and required rules; server CI digest/time; actual artifact digest/source mapping; precise human approval/refusal receipts. Old head/trust never revives approval; unsupported VCS stays blocked. Retain initial defects. | NOT_RUN |
| E02 / TC09 | I01,I04–06,I09 | Create a new sandbox order and pay once using the real business contract; observe until its declared deadline. | Payment processor charge/capture ID + terminal state/amount/currency; business order ID/version becomes paid; linked fulfillment ID/result; environment/service versions and query timestamps. A callback202 or UI label alone fails. | NOT_RUN |
| E03 / TC10 | E02 inputs + valid redelivery authority | Redeliver the same real event three times using provider-supported replay semantics. Keep event identity and distinct delivery attempts; signature timestamps may differ legitimately. | Three ingress attempts; authoritative funds ledger shows one economic effect, order transition ledger one effective paid transition, fulfillment ledger one effect. Record each domain's actual dedup key and queries. One comment or one webhook handler call is insufficient. | NOT_RUN |
| E04 / TC11 | E02 inputs + I05/I06 fault controls | Separate new cases for real45s delay, timeout-then-success, out-of-order events, consumer failure/recovery. Read original operation before any retry. Resume consumer using supported mechanism. | Real sender/receiver UTC timestamps, monotonic elapsed interval and injection point (not simulated clock); initial timeout/failure and later terminal success; no state regression or duplicate charge/transition/fulfillment; retained queue/consumer attempts, compensation linkage and three-domain ledger results. Define acceptable delay tolerance from actual target, not invented pass bounds. | NOT_RUN |
| E05 / TC17 | I01–03,I07,I09–10 | QA attempts release and is refused with durable audit and zero external invocation. Designated release human signs exact code/build/environment after QA; executor rechecks current authority/expiry immediately before external call. Exercise changed target/digest and revoked/expired signer. | Exact candidate/head/build/env and decision dependencies; authenticated actor/credential class/revisions;403 `stage_role_required` or relevant frozen refusal; denied-operation invocation count0; successful operation ID + trusted observed result distinct from approval; service-account authority and native target permissions. | NOT_RUN |
| E06 / TC18 | E05 inputs + I08 + resolved metric binding | Cause permitted nonproduction anomaly, retain raw metrics, derive rollback suggestion, obtain designated human approval for exact original deployment and allowed previous digest, execute only that target, query actual result and health. | Metric source/query/window/threshold/version/sample identity; suggestion/approval IDs; deployment ID, previous/target digest, environment, actual rollback operation/result; post-rollback artifact and health checks. Suggestion/signature alone is not rollback. Late/duplicate notifications cannot roll back a different deployment. | NOT_RUN |
| E07 / TC19–20 | All relevant inputs, real E05/E06 facts | Bind actual deployment/rollback + verification to exact final delivery. Read through API/Web/Desktop; try unknown/unexecuted/stale/revoked evidence and direct completion paths before valid submission/acceptance. | Immutable execution fact/approval/test report references with integration generations, actual verification run/results, exact submission ID/digest and current designated acceptance; refused completion has `stage_incomplete`/specific blockers and no side effect. Unknown/unexecuted never renders success; cross-client references match real source identity. | NOT_RUN |

For E03/E04 the payment processor is authoritative for financial effects; business order service for effective order updates; fulfillment service for fulfillment effects. Consumer/inbox logs explain delivery attempts but cannot replace any of these ledgers. S4 tests the external business system and reports evidence to Multica; it does not turn Multica Cloud subscriptions or issue metadata into an e-commerce backend.

### Safe next steps and ownership

1. **Now, offline:** root reviews this intake/risk list; use companion JSON as a missing-input checklist. Keep E01–E07 NOT_RUN and S4 open. S2 2c resolves internal execution/metric bindings; server owner evaluates the GitHub stale-event defect. No public-contract rewrite by this research lane.
2. **After real target data is provided:** bind source/API and actual owner/environment names; inspect only the supplied supported system's official API/version. Verify task scope, sandbox/nonproduction target, callback route and scoped authority. Existing user authorization covers preparation; real integration is conditional on provided targets and authority. No production, live funds, real-model account calls or automated code merge.
3. **Before implementing:** select the smallest existing-adapter extension and obtain independent plan review; hand product edits to `trellis-implement`. S4 owns true external adapters/evidence; S2 owns gates/internal facts; S5 harness owner owns test-environment allocation. Do not edit or reset their work.
4. **Controlled local verification:** after root supplies owned test environments, reuse the regression entries below. They prove platform behavior only. Record exact command/source/environment and named result inventory; do not run ambient DB TestMain or provider CLI discovery.
5. **Actual integration:** record each run separately, execute E01–E07 only against bound targets, query authoritative ledgers, retain failures and unknowns, independently review real evidence. E08/closure requires that review and all required real evidence; preparation alone does not satisfy it.

Existing regression entry points (identified, **not executed**):

- `server/internal/integrations/ghsnapshot/snapshot_test.go`: `TestFetchPRSnapshotPaginatesContexts`, `TestFetchPRSnapshotRejectsHeadChangeDuringPagination`, `TestFetchPRSnapshotRejectsPaginationBeyondLimit`, `TestFetchPRSnapshotNullRollup`.
- `server/internal/integrations/ghsnapshot/refresh_db_test.go`: `TestApplySnapshotHeadSHAGuard`, `TestApplySnapshotReplacesRuns`, `TestInFlightOldHeadKeepsTrailingRefresh` (DB ownership required).
- `server/internal/integrations/vcs/vcs_test.go`: `TestForgejoVerifySignature`, `TestGitlabVerifySignature`, `TestGitlabParseTimestamps`.
- `server/internal/handler/vcs_webhook_test.go`: `TestVCSWebhook_StaleEventDoesNotRewriteLink`, `TestVCSWebhook_CommitStatusMirrors`, `TestVCSWebhook_BadSignature` (handler DB environment required).
- `server/internal/handler/delivery_completion_webhook_test.go`: `TestDeliveryCompletionWebhookSignatureRecheckedInStatusTransaction`.
- `server/cmd/server/delivery_completion_webhook_integration_test.go`: `TestDeliveryCompletionWebhookRouterTrustChangeRefuses`, `TestDeliveryCompletionWebhookRouterExactGrantConsumesWithState` (router DB environment required).
- `server/internal/handler/cloud_billing_test.go`: `TestStripeWebhookForwardsRawBodyAndSignature` proves forwarding only, not PAY-241 payment correctness.
- New needed coverage: old GitHub PR event after newer head; VCS two repos sharing SHA and multiple attempts; strict skipped/neutral handling; native build identity; S2 2c trusted fact/replay/unknown/metric/current-authority integration. Tests do not exist merely because this list names a behavior.

### Related specs and external references

Read and applicable: root `AGENTS.md`/`CLAUDE.md`; `.trellis/workflow.md`; `.trellis/spec/server/index.md`, `security-boundaries.md`, `delivery-evidence-observation.md`, `controlled-artifact-execution.md`, `verification-environment.md`. Observe-only PR/artifact results are not authorizations; local-synthetic packaging is not deployment. Verification must bind real environment/source and retain failed runs.

Adjacent boundary: `.trellis/tasks/11-10-git-engineering-material-workflow/design.md:13` and `:155` keep Git material publishing, requirement merge and D8 orchestration in their existing task. Multica human decisions and native Git actions retain different actual actors; no bot impersonation, protection disabling or automatic code merge is introduced here.

No external documentation was fetched. First-party API references evidenced in local code: GitHub `https://api.github.com`, installation token endpoint and GraphQL (`ghsnapshot/client.go:40`, `:162`, `:216`); repository-browser REST header version `2022-11-28` (`github.go:917`), which is not a GraphQL version guarantee; GitLab token validation `/api/v4/user` (`gitlab.go:205`); Forgejo/Gitea adapter validation (`forgejo.go:184`). Repository Go module declares Go1.26.6 (`server/go.mod:3`). Actual GitLab/Forgejo instance, payment, fulfillment, deployment and metrics versions/contracts remain unknown and must be supplied/verified before use.

## Caveats / Not Found

- All external cases and all referenced code tests are NOT_RUN in this research. No account/credential/database inventory or external API validation occurred.
- GitHub stale-event finding is source-backed; approval resurrection is a risk to reproduce, not a claimed tested exploit. Unknown node completeness is also a strict-evidence concern, not a claim that GitHub sent such a node.
- S2 is actively changing. No final 2c implementation availability or completed contract verification is asserted. Metric binding is a design input for its owner, not a silent public API amendment.
- Existing Stripe billing proxy is real code but has a different business boundary. No PAY-241 payment/order/fulfillment service or release/rollback target was identified in the bounded source/task review.
- Actual environment names, named owners, official provider contracts and execution grants remain unresolved. Null intake fields mean unknown, not proof that the resource does not exist.
- Machine-readable companion is a preparation manifest, not a launcher, credential file, test report, approval or external success ledger.
