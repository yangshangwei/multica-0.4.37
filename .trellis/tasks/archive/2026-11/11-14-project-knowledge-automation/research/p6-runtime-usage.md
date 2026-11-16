# P6 runtime lifecycle

The coordinating session must freeze the selected private Go, application/shared
source, fixtures, specs and generated help before invoking `build`, `start` or
`run` with `--source-ready`. Implementation and syntax/help checks do not grant
runtime admission. Shared ROOT integration and final AC review remain separate
gates; this script does not establish real-provider quality or final acceptance.

The current run is `p6-browser-072e7156`, allocated by
`p6-isolated-allocation-4a8f26ff.json`. The earlier `p6-browser-0cc54918` and its
task pointer remain historical evidence. Read the current allocation's private
`source_root`, change CWD to it, and use the controller retained inside that
source. Invoking the ROOT copy against this private allocation is invalid.

```sh
P6_ALLOCATION=/Volumes/artisan/code/2026/multica-0.4.37/.trellis/tasks/11-14-project-knowledge-automation/research/p6-isolated-allocation-4a8f26ff.json
P6_SOURCE_ROOT=$(python3 -B -c 'import json, sys; print(json.load(open(sys.argv[1]))["source_root"])' "$P6_ALLOCATION")
cd "$P6_SOURCE_ROOT" || exit 1
P6_SCRIPT="$P6_SOURCE_ROOT/.trellis/tasks/11-14-project-knowledge-automation/research/p6-runtime.py"
P6_BROWSERS_PATH=/Volumes/artisan/dev-cache/playwright

python3 -B "$P6_SCRIPT" status --allocation "$P6_ALLOCATION"
```

After the repair's focused static/noEmit and independent delta checks pass, the
leader must issue source readiness for the newly frozen inputs before the
following lifecycle. These are runbook commands, not results of those checks.

```sh
python3 -B "$P6_SCRIPT" build --allocation "$P6_ALLOCATION" --source-ready \
  --browsers-path "$P6_BROWSERS_PATH"
python3 -B "$P6_SCRIPT" start --allocation "$P6_ALLOCATION" --source-ready
python3 -B "$P6_SCRIPT" status --allocation "$P6_ALLOCATION"
python3 -B "$P6_SCRIPT" run --allocation "$P6_ALLOCATION" --source-ready \
  --browsers-path "$P6_BROWSERS_PATH"

python3 -B "$P6_SCRIPT" stop --allocation "$P6_ALLOCATION"
```

No new allocation is required. `start` migrates only the allocated browser DB,
checks its actual identity/headroom, complete migration ledger and manual-only
Collaboration execution gates, then starts the API, deterministic local model,
production Next Web and the existing static Desktop renderer helper. All stage
deadlines are bounded and configurable; `--help` lists their defaults. `status`
reads identity/readiness without starting services or writing state. `stop`
checks recorded PID, process-group and birth identity, retains databases/builds/
logs, and leaves unproved processes untouched. Rebuild requires owned services
to be stopped first. Interruptions and failed stages clean up recorded children.

## Current checkpoint — 2026-11-15

The retained earlier `93c78b9a…` source passed complete API/CLI/migration-tool/production Web/
Desktop builds and managed live startup proof. Both actual journeys completed
password registration/onboarding, formal UI acceptance, worker automatic adoption
and identity/source/rule assertions, with four retained screenshots. They then
failed at managed daemon startup with `managed profile has no user identity`.
The original failure is `p6-original-after-password-fixture-failure-1a428255.json`;
partial UI evidence is
`p6-partial-ui-acceptance-adoption-before-daemon-failure-8670a2df.json`.
All four owned services were stopped with zero cleanup failures; DB/logs remain.

The fixture now mirrors Desktop's real profile owner setup: write the verified
`managed.userId` into the owned `.multica/profiles/<profile>/.desktop-user-id`
with `0600`/exclusive-create, then read it back and require the registered creator
ID. The real PAT, enrolled identity, signed challenges and stdin handoff remain.
Go's `managedProfileUser` still validates the owner and cross-checks the actual
principal/identity. See `p6-managed-profile-user-marker-repair-f872168e.json`.

The new candidate is
`4e0f7cbe36dadf1936c31a5dfc04656e7d6ac4594e5a45b9eac1e8e02f697191`.
Focused noEmit passed for private and ROOT inputs with zero errors. This automation
fixture's actual lint retains five preexisting diagnostics and adds zero;
standalone lint remains false. Independent owner admission has zero confirmed
blockers (`p6-independent-managed-profile-owner-admission-20261115-d9da0e34.json`).
Its fresh full build and both complete journeys now pass on the same frozen
inputs: `build-419ecea5cf21/build.json`, Web BUILD_ID `Suwn9SgGXHAkR5YxpSvU5`,
`start-eaaff8bb0f95/runtime.json` and `test-0440e9486493/result.json`. The actual
Web/Desktop report has two passed journeys, zero failures/skips/retries/flaky
results, one worker and stable source. It covers actual worker adoption, production
daemon input/v2 proof, immediate reopen refusal, fresh reacceptance validity and
execution, plus no-project/manual zero-call and workspace-write invariants. See
`p6-final-bound-journeys-and-cleanup-fc83cb34.json`.

All four owned persistent services are stopped with zero cleanup failures.
`p6-final-owned-resource-audit-569cc14c.json` independently observes no remaining
owned runtime process, no listener on 19618/20088/61990–61993, and zero browser/C3
DB connections. Eight screenshots and actual per-profile `daemon.log`/retained
`daemon.pid` files remain. Original result/report/log/browser binding and the three
`0600` before-stop snapshots are unchanged. The final independent private
AC1–AC12 review now accepts the frozen 4e candidate with zero blockers in
`p6-independent-final-candidate-ac-review-20261115-a710db0a.json`. It verifies the C3 direct-replay/3,104 production-input mapping and
all 23 primary evidence hashes. The Desktop restored screenshot has a retained
transient-refresh qualification; the actual assertions and proof2 checks passed.
Shared ROOT still requires its own final source/build/runtime binding.
The bounded current ROOT knowledge-service normal/race check is now accepted
in `p6-root-knowledge-service-independent-v3-normal-race-20261115-87a4b72f8b7f.json`: each mode has
194 top-level, 428 named and 365 leaf PASS, zero failures/skips/incomplete
names, matching 4,774 current source inputs and zero owned DB connections.
The private v3 parser passes 29 cases. Original recorder/cache-copy failures
remain intact; the existing Go executions were not repeated. This completes
service preflight and does not grant final ROOT source/runtime readiness.

The exact already integrated ROOT fixture deltas are in
`p6-root-narrow-fixture-binding-b7dc8f78.json`; they preserve the existing comment-menu
changes and do not imply complete source parity.

The password/onboarding and managed-profile fixture contract has a bounded
independent documentation PASS with zero blockers in
`p6-password-managed-spec-check-52067d2e.json`. It includes the existing
`requires_account_setup` and `requires_password_change` principal rejection
conditions. Product tests were not run by that documentation review; the prior
standalone fixture lint diagnostics remain recorded.

Current ROOT bounded frontend verification passes Core107/Views46 tests, both
actual package typechecks and 34 knowledge lint inputs with stable source in
`p6-root-knowledge-frontend-verification-jOgYVN-c1c27d4b.json`. The selected source preflight
`p6-root-knowledge-integration-preflight-a34fa9cc4e45ecca.json` confirms zero knowledge
incompatibilities among 113 pairs; it is not an atomic freeze. Final ROOT binding
and journeys follow the PAY owner's accepted P2 r07 source. PAY has formally
accepted CODE_ACCEPTED_R06 with zero current code blockers, while actual P2
and four products remain unaccepted; see `pay241-code-accepted-r06-bound-callback-16483c51.json`.

Earlier f8 registration failures and all associated reports are retained in
`p6-original-password-registration-failure-e562808a.json`. ROOT's existing
comment-menu/Views and PAY server differences are preserved; there is no complete
public/private parity or shared ROOT acceptance. The Go-test-owned Redis container
and network were removed with proved ownership and zero established TCP clients
(`p6-redis-owned-final-teardown-fca701ac.json`); actual P6 uses the single-process
password limiter without a Redis test URL. PostgreSQL data, images, logs and raw
failures remain retained.

## Outputs and fixture compatibility

Every build creates a private external `build-*/source` snapshot inside the
allocation directory. Both `.next` and Desktop `out`/bundled CLI files are
produced there. Source package/configuration files are copied unchanged;
installed third-party dependencies are linked and `@multica/*` workspace links
are rebound to the snapshot. Its isolated detached Git metadata supplies the
original HEAD without a branch, commit or shared-index write. Existing checkout
Web/Desktop build outputs are not used or modified.

The two P6 fixtures derive their checkout and require conventional relative
`.next`, renderer and preload paths. Therefore Playwright, its config, fixtures
and native bootstrap execute from the snapshot. A private
`build-*/execution-allocation.json` binds that source root to the unchanged parent
allocation hash, allocated private source root, DB and allocated origins. This
original source root is the retained private candidate. The original
allocation and repository task pointer remain unchanged by build/start/run/stop.

`RUN/provenance.json` includes the fixture's exact fields: runtime permit, allocation
hash, source hashes, owned listener PID/commit/command, Web production build ID,
renderer/preload hashes and matching API/Web/renderer origins. Additional fields
bind process birth/group, runtime environment, parent allocation and source ID.
The source inventory explicitly includes generated help
`server/internal/docs/content/manifest.json`,
`server/internal/docs/content/pages/collaboration-knowledge.json` and
`server/internal/docs/content/pages/environment-variables.json`.
Only the exact generated Next route-types import is normalized for source drift;
other source, tool, workspace-link or artifact changes refuse admission.

The private environment masks dotenv key names and uses
`MULTICA_AUTH_MODE=password`, `MULTICA_MANAGED_INSTALLATIONS_ENABLED=true` and
`MULTICA_PASSWORD_LIMITER_MODE=single`. The stable deployment identity for this
allocated DB is `MULTICA_DEPLOYMENT_ID=3cc23866-f1a4-575b-a56c-40351c7d35d7`;
live auth provenance must match it after every rebuild/start. Device auth,
platform admin, update/reload and external integrations remain disabled. Only
synthetic auth/model credentials are supplied. Every known provider probe is
denied except the byte-matched fake Claude executable; the model is the existing
loopback fixture, never an account proxy.

Raw logs, environment files, per-stage process receipts, starts, browser binding,
Playwright JSON inventory/results and failures stay outside the repository.
`run` requires an explicit installed browser cache, checks the installed registry
and resource hashes, launches one worker with zero retries, and requires both
named Web/Desktop journeys to finish without skips. It never downloads browsers.

## Remaining execution prerequisites

The current 4e private lifecycle is complete. A future source change or the final
shared ROOT integration needs a new explicit source-ready gate and matching build;
installed Go/Node/pnpm/PostgreSQL client/lsof;
the retained local browser database with cluster headroom; all four allocated
ports free; dependencies and the macOS Electron runtime already installed;
an independently cleared Playwright cache containing Chromium, headless shell
and FFmpeg. The leader owns actual builds, migration/start/stop checks, native
operation, negative fixtures, final source audit and product acceptance. Private
success must still be reconciled against the final shared ROOT inputs and AC1–AC12.

## Advancing the shared source after P2 acceptance

The current full ROOT intake (`p6-shared-ROOT-source-intake-a7127b07.json`) records 11,028 source files
and 32 workspace links with identical complete reads. It is an observation,
not a freeze or source-ready signal. Current ROOT Go build/vet passes with
12,173 stable inputs in `p6-root-knowledge-go-static-verification-20261115-7a3e544be95f.json`.

After the existing PAY owner supplies the accepted P2 r07 source, validate the
exact final ROOT delta and affected Go/SQL/schema/frontend evidence. Preserve
the immutable passed `build-419ecea5cf21/source`, build/test receipts and all
before-stop snapshots before advancing only the private working source files
to that verified ROOT set. Keep the allocation and original environment bytes
unchanged and retain peer ROOT work. Record both old/new source identities and
workspace links; invoke the existing controller from its CANDIDATE CWD for a
new source-ready build/start/run/stop. Both knowledge journeys and final ROOT
review must bind the new inputs. No source copy or runtime operation has been
performed by this intake; original private 4e acceptance stays historical.

## 最终ROOT整合入口已开放

P2已取得 `P2_CODE_ACCEPTED_R09_SCOPED`／0阻塞／ready_for_ROOT_narrow_integration=true，源80abb746／4,968；18成功profile，10新命令＋8明确绑定复用，N／R具名集合相等，4native SQL controls／862恢复／历史与资源审计通过，0所属连接／PID。原R07／R08失败保留；Live／完整S2／四业务场景仍未接受。

ROOT三文件已窄整合，5,035保护路径0漂移、35peer产品差异／865／hooks／virtualoff保留；整合回执SHA b52fc31fd8a0e97e0bbbc76532a4dc2c5d63903aa9f43d00a7aa7b3240af5166。澄清ddb430dbfdf85ac518a8a6e4c2c79a9967aa5a753f423584d0c83263cd9b616a说明P2项目stage策略默认off未变，非知识自动策略的默认值。Go joint e4dbc77e／5,038与frontend315a7654／3,358仅分层证据。

当前开始封存完整共同ROOT（含apps／packages／实际fixtures／spec／task md-py-jsonl／生成帮助），沿现有allocation lock和逐文件before-images推进private mutable源，保护sealed4e／allocation／env／32workspace links。原PAY checker负责4fresh库／853 schema的接入后最小service／API复验；知识暂DB0。通过完整source／code／review门槛后再发新的source-ready并执行匹配build→start→两端journey→stop／资源审计；最终ROOT接受仍false。
