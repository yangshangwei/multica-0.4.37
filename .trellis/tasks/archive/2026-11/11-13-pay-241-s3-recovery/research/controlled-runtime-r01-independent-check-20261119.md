# Controlled runtime R01 independent check — 2026-11-19

**R01 NOT_ACCEPTED: four failed tops, zero passes/skips, retry 0.** Every top stops at its initial target Resume response, HTTP 409, before a human input or Provider turn. The actual ten-subcase inventory is **2 FAILED / 8 NOT_RUN**, not four failed subcases plus six NOT_RUN. All intended execution/result/recovery/withdrawal claims and NR-P02 remain NOT_RUN.

- Reviewer: `/root/notification_runtime_check`, existing native `trellis-check`; no spawning.
- Active task: `.trellis/tasks/11-13-pay-241-s3-recovery`.
- Owned writes: this report and adjacent evidence map only. Ordinary-code acceptance, subscription R04 failure and the existing NR-P01 report remain separate.
- Evidence root: `/Volumes/artisan/.multica-acceptance/pay241-s3-continuation-20261119-uwkcehly` (`B` below).
- Source: `B/source/client-harness-r05`; runtime fixture SHA-256 `fcbac0e765550890c009ae8c96d162f84985b3689a797723432e5db930be4c39`.
- Main executed the round and owned cleanup. The reviewer inspected existing source/files/raw evidence only: no control replay, suite rerun, SQL/API/browser/Provider body, process control, configuration/source/spec/metadata/commit change or raw-credential copy.

## Findings (fixed)

### CR-F01 — correct top/subcase denominators

The initial summary conflated four failed tops with failed subcases. The original JSON was preserved; this report corrects its interpretation:

- Both notification tops call initial Resume at fixture `:703` before entering `run(name)` at `:711`. Their three subcases each remain NOT_RUN: six total.
- Both withdrawal tops enter `queued_input_author_withdrawal` at `:871`, then fail at initial Resume `:881`. Those two started subcases are FAILED; each later `live_resource_grant_withdrawal` remains NOT_RUN.
- Therefore the complete planned ten identities contain **2 FAILED / 8 NOT_RUN**, while Playwright contains **4 FAIL / 0 PASS / 0 SKIP**. No subcase or top is missing, duplicated, retried or borrowed from another round.

This interpretation was reported to Main immediately. No runtime result was changed.

## Findings (not fixed)

### CR-P01 — initial Resume conflicts block every intended execution path

The exact first control requests below return 409. Shared `collaboration-host-runtime.ts:756` asserts HTTP 200 before reading request-shape assertions at `:757`–`:758` or decoding response JSON at `:759`. The case response observer retains status, request ID and safe headers, **not the response body/code or its hash**. No refusal reason, payload-version conclusion or root cause is invented here.

Main's separately owned backend diagnosis must bind its retained control observations to these original keys. This reviewer neither replayed them nor changed a public control contract or fixture behavior.

| Top | Issue ID | Original Resume request ID | Actual control actor |
| --- | --- | --- | --- |
| Web notifications | `01a1f175-5130-74ab-8a9b-14eaff4c07c0` | `9778b4cd-da53-415c-9094-fc03c9bd7a28` | `908da1f3-a822-467f-9228-7c08ceb4ee6c` |
| Web withdrawal | `01a1f175-6129-756e-8224-76706332eb44` | `f7a0a894-1ef5-4d72-8d59-5e330860ce52` | `ee9703ca-c692-484e-9a44-f0fdae1db28e` |
| Desktop notifications | `01a1f175-7801-7ad5-ae63-c57553cc6e9d` | `c5df2c71-7e0b-43cb-b423-7561672ec3e0` | `16de9411-0c67-4728-83cd-114ff880cfd9` |
| Desktop withdrawal | `01a1f175-8e37-7962-8b3a-6513de9ae649` | `ba28dd0b-99a5-41df-81ba-b4e3ba3a039e` | `52dfd8f2-aeda-4ccb-82e9-9f2bc2c0bb7a` |

Each withdrawal control actor is the handler/controller, distinct from the participant who would author the later input. The latter never posts. This distinction must survive a later author-withdrawal test.

### CR-P02 — PID monitor is not an exhaustive executable/role census

- `runtime/runtime-processes-r01/final.json` records 25 observed PIDs: four `host_daemon`, 21 `owned_other`; there are no classified Provider/CLI/native-fixture roles.
- Its `executable` values are truncated prefixes, such as `/Volumes/artisan`, `/Users/artisan/.` and `/opt/homebrew/op`. `helpers/runtime-process-monitor.py` invokes `ps -axo pid=,ppid=,pgid=,comm=,args=` without wide output and splits the result into five whitespace fields. Role checks inspect that truncated executable field. Its copied readiness Provider is also named `fixture-provider`, outside its current Provider basename checks.
- Native main PIDs 36689, 36761 and 36782 are absent from that monitor while their helper children appear. The task-name argument filter likewise does not establish coverage of every held frontend or short-lived readiness probe.
- Zero classified Provider/CLI roles therefore cannot prove an exhaustive zero-subprocess count. Main should correct its owned observer before any future complete per-launch PID acceptance, retaining this original monitor evidence and avoiding raw argv/credential publication. This reviewer did not change that helper.

The present zero-turn conclusion is independently supported by no inputs/runs/attempts/ledger/Provider callbacks, rather than by those role counts. The monitor limitation does not turn these failed tests into execution evidence.

## Raw and source/configuration binding

`verification/runtime-extras-b-r01/stdout.log` SHA-256 is `f0ebdd4b4efa6ad2f7d3376e37f6639f591a6a48b6d1d5209dc98350d77be69b`; stderr is empty. Main's command exits 1. Raw Playwright JSON contains the four exact expected tops, one failed attempt each at retry 0, zero skipped/flaky results and no global reporter error. Each error points at the shared pre-decode 200-versus-409 assertion.

The four attached case JSON files exactly equal their retained case files. They keep every planned subcase name, the original primary failure before client teardown, a separate primary-runtime failure checkpoint, every client cleanup result and the final daemon/transport/gate cleanup. The two started withdrawal failures each retain the new NR-P01 immutable scope checkpoint, including both author/controller sessions. No failed or NOT_RUN identity is erased.

The body's 257-entry harness manifests match before/after and current bytes/modes. Matching `runtime-typecheck-r05`, `runtime-eslint-r05` and exact four-top `runtime-list-r05` Main receipts exit 0, with matching raw hashes and no source/config/tool drift. The actual body still fails. The fixture is the unchanged R02 retention version; no runtime behavior change was inserted after admission.

The actual command binds `runtime-browser-r01.json`, SHA-256 `28ffcc0b94498243aeb24991d911db077a25fcd9d831127aa7ad0ce5f3b0c95c`. Its explicit opt-ins/password configuration, URL database name and `TASK_DATABASE_NAME` match the untouched fresh `runtime_r02` database. API and public API origin are both `http://127.0.0.1:64940`, production Web/baseURL is `http://127.0.0.1:64941`, renderer is `http://127.0.0.1:64942`; the Playwright config explicitly supplies baseURL, workers 1 and retries 0. It uses the accepted R08 client graph and exact final GREEN server binary, not the unrelated subscription database.

Main's explicit kind/capability binding matches current files and `0755` modes:

| Executable | Current bound SHA-256 |
| --- | --- |
| `s3-host-daemon-r02.test` | `59d9fcbe04ed518be6955128dc0263f543fce0878cfd4f75b65d84074bd3ab7e` |
| `s3-host-cli-r02` | `9d935bbb9a1b200c597ffa63e22858143e9a05ab48ae0f88acbdee39c478b639` |
| `s3-host-provider-r01` | `e6987442d61629087fd44f9eb39bd5ee0b23b007b38710cd0b34677cf005fbaf` |

The bound Provider is `multica-host-e2e-provider-v1` with test-report/failure-release/prepared-git-marker capability. Kind/capability/version probe subprocesses are readiness work, not any of the ten intended Provider execution launches or model/account calls. No test executable or vendor CLI was discovered or run by this reviewer.

## Actual effects and intended claims

Every case reaches public password setup, independent source/resource workspace creation, managed enrollment/registration, initial private input-file upload, public issue/handoff/participant setup, target adoption/consent and host readiness. All four daemon readiness summaries report protocol 3, host-process supervision, preparation 2, macOS/darwin arm64, config epoch 2 and one machine slot. These are actual preparation facts; they do not prove a turn or termination.

Each `daemon_final` has exit code 0, empty errors/stderr/ledger, available slots 1, active tasks 0, claims 0 and no Provider visit. The transport has no run-scoped request and no original claim/permit/outcome/proof/snapshot/memory-base request. Ordinary `/api/daemon/tasks/claim` polling receives empty-work replies; it is not a collaboration Claim or a dispatched task. One Desktop-notification capability GET has retained 503, plus shutdown-time unfinished poll observations. Their cause is not attributed to the primary 409 or normalized into successful business outcomes.

The after audit enumerates the same 211 catalog table keys and records zero for comments, ordinary inbox/subscribers/task queue, collaboration commands/groups/runs/attempts/group inputs/business capabilities/reservations/terminations/run events. Thus:

- Planned intentional Provider executions: **10**; actual admitted turns/launches: **0**.
- Planned original human inputs: **12**; actual posted inputs/commands: **0**.
- Natural success/failure/Stop, original run/ledger recovery, result-recipient identity, live/queued withdrawal, original-token positive reads and later denial: **NOT_RUN**.
- NR-P02 original run→grant/input/capability/reservation reconciliation: **NOT_RUN**, because none of those execution identities exists. Empty counts do not satisfy a revocation or preservation proof.

The whole database is not unchanged or empty: setup retains 16 users, eight workspaces/members, four issues/Agents/runtimes/grants/targets/attachments, 12 participants/notifications, 28 collaboration operations/events, scan/configuration/invalidation metadata and other fixture records. The 12 setup notifications are not proven runtime-result notifications. Private upload rows/files remain input preparation, not Agent output uploads. No reviewer cleanup erased those facts.

## Owned closure and preserved failure

Four observed host daemon PIDs **36509, 36587, 36676, 36749** fall in the four sequential preparation/cleanup phases. Their case summaries report graceful exit 0 with no claims or ledger. Three native fixture main PIDs **36689, 36761, 36782** have actual exit code 0/no signal and page/context/profile-close completion. Three Web contexts also close: six client sessions total, all cleanup results fulfilled and no page/cleanup errors. These native exits are recorded by the case owner, not inferred from missing monitor roles. Installed full Main and kernel containment remain outside scope.

Main's API launch/readiness/stopping agree on PID/PGID **33903**, port **64940**, binary SHA-256 `0c8ca9898c5b022df40bf2230849c956ac2c14ee5fe59d4d87b02f805306f999`, configuration SHA-256 `df6566f09ec9f7bc468ecd99bd1a74a903c863dd87af84106548b5c83a2b7c8d` and actual built `b1901a48f` health identity. Owned SIGTERM stopping records `listener_absent=true`.

`audit/runtime-extras-b-after-r01.json` SHA-256 is `589c354e210a18bc957606d27807f9975620e0ce07aaffe3beaca4cba276d729`. Compared with the pre-API runtime audit, it has the exact same 855-entry migration ledger and all 663 unchanged ready/valid indexes. Original booleans are restored—creation true, execution false, host false, no freeze—with the required monotonic epoch **2 → 10**, one open/restore pair per failed top. Final `other_connections=0`.

The monitor's final file/stop marker are retained; its last observed alive set contains held renderer PID 25647. Main still holds Web 25150/64941 and renderer 25647/64942 for separate work. This report does not claim all owned frontends or every sampled process disappeared. Raw credential-bearing private ledgers/profiles were not published or copied; only allowlisted case evidence and hashes are used.

## Verification

- **Actual runtime tests: FAIL / NOT_ACCEPTED**, 4 FAIL / 0 PASS / 0 SKIP, retry 0; exact raw/attachment/source/configuration evidence reconciled.
- **Subcase inventory: complete**, 2 FAILED / 8 NOT_RUN; all result values remain null.
- **Lint/type-check/list:** Main's matching exit-0 static receipts inspected; no checks or bodies rerun by the reviewer.
- **Executables:** exact selected hashes/modes and Main's readiness binding reconciled; no actual execution turn accepted.
- **Cleanup/catalog:** case daemon/native/client closure, owned API stop, original gate restoration, ledger/index identity and zero final DB connections inspected. Process-census limits CR-P02 retained explicitly.
- **Unfixed:** primary Resume refusal CR-P01 awaits Main's retained-source diagnosis; monitor fidelity CR-P02 needs an owner correction before future complete process proof. No production/source changes made here.
- Ordinary-code acceptance, subscription R04 failure, NR-P01 static closure, prior timeout proof and all full-S3/S4/S5 decisions remain separate. This failed round promotes none of them.
