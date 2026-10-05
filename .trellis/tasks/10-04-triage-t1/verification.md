# T1 acceptance ledger

Status as of 2026-10-05: T1 implementation is complete and merged/pushed to `main` as `85b0d2e7e`. Independent branch acceptance substantiates all 37 AC cases and 8 FR obligations, with a 94/100 visual pass. Subsequent local main-worktree regression has passing evidence for all 17 triage cases. Remote CI is failing and production release is unconfirmed; implementation completion does not mean release readiness.

## Main integration and final regression follow-up (2026-10-05)

- [Merge `85b0d2e7e`](https://github.com/yangshangwei/multica-0.4.37/commit/85b0d2e7e0346f77cb127716842bd36de8dd3fc8) landed at 11:14:33 Asia/Shanghai. Local ancestry and `gh api repos/yangshangwei/multica-0.4.37/commits/main` both confirm T1 is on main; the remote API returned that exact full SHA.
- The merge record reports lint, typecheck, 9,951 TypeScript tests, 6 local production-mode Web triage scenarios, migrations through 535, Go race checks and `go vet`. It also records a pre-existing full-Go failure: the built-in agent-creation skill was 534 lines against a 500-line budget. This was not a fully green main CI result.
- The later regression session `01a10a62-eb9e-7bd3-a835-0ea8df902a6b` completed. Its [coverage report](../../../docs/qa/e2e-branch-coverage-2026-10-05.md) records the final results below. These runs used main commit `85b0d2e7e` plus uncommitted test expansion and fixes, including the CORS `Retry-After` exposure fix; they do not prove the clean remote commit passes every check.

| Check | Final recorded evidence |
| --- | --- |
| E2E inventory | 234 unique cases across 74 spec files; all have a one-attempt pass, with no final failure, skipped-only case or missing evidence. Results aggregate multiple environment-specific runs, not one all-green full-suite command. |
| Triage regression | 17/17: 6 existing Web cases and 1 native Electron case in `baseline-current.json`; 10 new cases in `branches-final.json` (7 browser and 3 HTTP integration cases). |
| New triage branches | Disabled intake, pending snooze blocking disable, required priority, reviewer rules, stale revision recovery, invalid snooze time, reopen rules, intake replay, decision replay and execution/assignment fences. |
| TypeScript unit tests | 9,956 passing tests across 5 package tasks; the docs task used valid cache. The historical mobile suite is not included. |
| Static checks | Typecheck, lint and strict E2E TypeScript checks recorded exit 0; typecheck/lint reused valid Turbo cache, with 29 existing lint warnings. |
| Go affected package | `go test -race -p 2 -parallel 2 ./cmd/server` and `go vet ./cmd/server` recorded exit 0 against an isolated migrated database. The later session did not rerun the whole Go repository or `make check`. |
| Final rerun correction | The last combined non-password run passed 41/42; one chat test used an unstable generated title. After switching its navigation to a stable conversation ID, the full chat file passed 6/6. Earlier failures remain in the evidence history. |

Closeout independently reconciled all 234 unique identities against their referenced raw Playwright JSON reports, including one-attempt passed results and no report-level errors. All eight recorded source hashes match the current test/backend files. This closeout audited existing execution evidence and did not rerun application tests.

Local evidence lives in `.gstack/qa-reports/2026-10-05-branch-expansion/` (Git-ignored): `evidence-summary.json` generated at 2026-10-05 05:27:00 UTC, `checks-final.json`, `provenance-final.json` and the referenced raw reports. SHA-256: `evidence-summary.json` = `80ddc814d424d5ee4eeacdab81375d7d149bb53afccf0f47e6bca77e606f2c15`; `checks-final.json` = `829c36cbf4a31c1c03ad5bb481088965dca36f27b40460b4c2ac7ecc4c42d727`. The coverage report, added tests and accompanying fixes were still uncommitted at closeout; this ledger preserves the result even before that separate work is committed.

## Remote CI and deployment status (checked 2026-10-05)

| Surface | Observed status | Meaning |
| --- | --- | --- |
| [CI run 37260088874](https://github.com/yangshangwei/multica-0.4.37/actions/runs/37260088874) | Failed on `85b0d2e7e`: `backend-tests` / `Test`, and `frontend-build` / `Verify in-app docs bundle is up to date`; aggregate frontend/backend jobs also failed. | Main CI is not green. Job failures need resolution and a fresh successful run; local regression does not override them. |
| [Mobile Verify run 37260088889](https://github.com/yangshangwei/multica-0.4.37/actions/runs/37260088889) | Failed on `85b0d2e7e` at `Type check, lint, and test`. | Mobile verification is still open; full mobile triage editing remains outside T1. |
| Release / deployment | Latest published release is [v0.5.5](https://github.com/yangshangwei/multica-0.4.37/releases/tag/v0.5.5), published 2026-10-03 23:55:19 Asia/Shanghai. The [latest release workflow](https://github.com/yangshangwei/multica-0.4.37/actions/runs/37134019640) is for that version; GitHub deployments returned `[]`. | No T1 production release is evidenced. This does not rule out an unrecorded manual deployment; no production URL/version proof was available. |
| Default local environment | `bash scripts/dev-env.sh status --json` reported API/Web mismatch. API `127.0.0.1:18573/health` returned old commit `e3415e798`; Web 13493 responded but its current build identity was not verified. | The default local environment does not establish that current main is running. |
| Regression environments | APIs 18577/18578 and Web 13497/13498 are stopped, consistent with the regression cleanup record. T1 branch API 18557 still returned `1c391b625`. | These are local test/branch environments, not production deployment evidence. |

Release follow-up remains: land the separately owned regression fixes/tests, resolve and rerun failed remote checks, then verify the version and health of the intended deployment. This closeout performed no push, release, service restart or deployment.

## Original branch acceptance ledger

| Requirement | Scenario | Expected | Evidence | Status |
| --- | --- | --- | --- | --- |
| TRI-AC-01 | 默认工作空间尚未启用 | 普通任务流程保持原样，无分拣导航 | TestTriageAcceptanceEnableDoesNotBackfillOrdinaryProjectWork; default-off sidebar/search tests | Verified |
| TRI-AC-02 | admin 启用分拣 | 入口出现，历史正式任务不回填 | TestTriageAcceptanceEnableDoesNotBackfillOrdinaryProjectWork; Web settings/manual intake scenario | Verified |
| TRI-AC-03 | member 修改开关 | 操作被拒绝且原设置不变 | TestTriageSettingsRoleAndResolvedAgentGates | Verified |
| TRI-AC-04 | 成员导入 CSV | 有效行进入待分拣，每条有稳定编号，来源记录导入批次和行号 | TestTriageImportResolvesScopedNamesAndRetainsCandidatesOnly; Web CSV scenario | Verified |
| TRI-AC-05 | 未授权来源提交，或无任务权限的成员导入 | 不因开关开启而成功创建 | TestTriageDefaultAndHumanGate; TestTriageIntakeHonorsAgentAutonomy; TestTriageImportAccessAndEncodingLimits | Verified |
| TRI-AC-06 | 导入行带 `in_progress` 状态、迭代、智能体负责人和 @提及 | 仍待分拣，状态和迭代报告未采用，智能体只作候选；不加入迭代、不执行 | TestTriageImportResolvesScopedNamesAndRetainsCandidatesOnly; TestParseQuotedBOMAndIgnoredExecutionFields | Verified |
| TRI-AC-07 | 同一导入批次确认后网络中断并重试 | 每行只有一条任务，批次只有一条汇总通知 | TestTriageImportPartialRetryDuplicateAndDeletedReplay; TestTriageImportSameRowConcurrentRetryAndLongExternalID | Verified |
| TRI-AC-08 | 成员从项目普通创建任务 | 保持既有流程，不无条件被拦入分拣 | TestTriageAcceptanceEnableDoesNotBackfillOrdinaryProjectWork | Verified |
| TRI-AC-09 | 优先级要求开启，priority 为 none | 接受失败并定位优先级字段 | TestTriagePriorityAndCandidateRollback; action-dialog priority validation tests | Verified |
| TRI-AC-10 | 合法普通接受带项目和负责人 | 准入与字段同时生效，项目计数增加，不启动执行 | TestTriageOrdinaryAcceptanceAdmitsProjectWithoutExecutionOrInheritance | Verified |
| TRI-AC-11 | 接受时目标项目失效 | 不接受、不保存半套属性，保留用户输入 | TestTriagePriorityAndCandidateRollback; retained-draft action-dialog tests | Verified |
| TRI-AC-12 | 接受并执行合法且连续重试 | 只准入一次、只启动一次执行 | TestTriageAcceptExecuteRetryAfterTerminalTask; TestTriageLostCommitResponseResumesSameExecution; Web explicit execution scenario | Verified |
| TRI-AC-13 | 已接受后执行启动失败 | 保持已接受，显示未启动和重试执行入口 | TestTriageExecutionFailureKeepsAcceptanceAndContext; triage-execution-status.test.tsx (real banner/reason/original-actor retry) | Verified |
| TRI-AC-14 | 重复目标是自身或其他空间任务 | 不允许提交，不泄漏目标内容 | TestTriageDuplicateTargetsAreScopedFormalAndUnchanged | Verified |
| TRI-AC-15 | 标记为正式已结束任务的重复输入 | 可关联且保留原输入，不复制评论附件、不改目标 | TestTriageDuplicateTargetsAreScopedFormalAndUnchanged; duplicate target reference rendering | Verified |
| TRI-AC-16 | 拒绝原因只有空格 | 拒绝未提交，提示填写原因 | TestTriageAtomicAdmissionAndHistory; Web Chinese compact validation scenario | Verified |
| TRI-AC-17 | 正常拒绝后查看普通项目进度 | 不计为项目正式范围或交付 | TestTriageBoundaryFormalCollectionsAndExplicitSearch | Verified |
| TRI-AC-18 | 任务稍后至未来时刻 | 所有成员默认队列隐藏，可在稍后列表看到 | TestTriageAcceptanceSnoozedCommentKeepsSharedReadinessAndExecutionFence | Verified |
| TRI-AC-19 | 稍后到期 | 恢复队列，不改变截止日期和原始进入时间 | TestTriageDueQueueAndNotificationIdempotency; TestTriageAcceptanceRescheduleAndCancelSnoozePreserveAuditAndDates; 30-second queue polling | Verified |
| TRI-AC-20 | 稍后期间任务已被接受 | 到期不重新进入分拣 | TestTriageAcceptanceFinishedSnoozeCannotNotifyOrReenter | Verified |
| TRI-AC-21 | 稍后期间收到新评论 | 保留稍后状态，评论通知与审核结果独立 | TestTriageAcceptanceSnoozedCommentKeepsSharedReadinessAndExecutionFence; real HTTP comment-listener Web scenario | Verified |
| TRI-AC-22 | 收件箱通知被归档或稍后处理 | 分拣状态和任务稍后时间不变 | TestTriageAcceptanceInboxStateIsIndependent; real HTTP inbox Web scenario (archive/read/unarchive/unread) | Verified |
| TRI-AC-23 | 责任人被指定或离开 | 不替换执行负责人；离开后任务仍可由成员处理 | TestTriageAcceptanceResponsibilityModesAndDeparture; TestTriageSnoozeReassignmentDeliversToCurrentReviewer | Verified |
| TRI-AC-24 | 批量接受部分任务发生冲突 | 展示逐项结果，重试只处理失败项 | TestTriageAcceptanceBatchReportsMixedOutcomesAndRetriesOnlySelectedFailures; batch-dialog selected-only retry tests | Verified |
| TRI-AC-25 | 已拒绝任务重新审核 | 新轮次待分拣，旧理由和操作者仍可追溯 | TestTriageAtomicAdmissionAndHistory; TestTriageAcceptanceHistoricalFiltersUseDecisionSnapshotsAfterReopen; Web reopen scenario | Verified |
| TRI-AC-26 | 已接受任务请求重新分拣 | 第一阶段不支持，不暗中撤销执行 | TestTriageAtomicAdmissionAndHistory | Verified |
| TRI-AC-27 | 两成员同时处理 | 只有一个生效结果，没有重复通知或覆盖 | TestTriageAcceptanceTwoMembersRaceHasOneDecisionAndOneResultNotice (10 repeated passes) | Verified |
| TRI-AC-28 | 待分拣任务经 CLI、旧客户端或@提及启动 | 服务端拒绝并说明待审核原因 | TestTriageBoundaryOrdinaryMutations; TestTriageAdmissionEnqueueAndRuntimeBoundaries; real HTTP legacy PUT409 Web scenario | Verified |
| TRI-AC-29 | 全部未决任务均在稍后列表，admin 关闭 | 仍阻止关闭，并显示未决数量 | TestTriageIdempotencyCASAndSnoozeDisable; TestTriageDisableFencesConcurrentIntake; settings component test | Verified |
| TRI-AC-30 | 最后一条处理成功或队列筛选无结果 | 空队列与筛选无结果文案可区分，键盘焦点合理 | Final Web settings/manual scenario: rapid search clear, confirmed last action, actual empty-heading focus; triage-page component tests | Verified |
| TRI-AC-31 | 用户在描述输入框按数字或 J／K | 不触发审核快捷键 | Final Web input shortcut guard; triage-page focused queue/editable/portal guard tests | Verified |
| TRI-AC-32 | 已失去访问权限的用户打开结果通知 | 无法读取任务、目标或附件内容 | TestTriageAcceptanceRevokedMemberCannotOpenNotificationTargetsOrAttachments | Verified |
| TRI-AC-37 | 分拣台已关闭，从历史链接、API 或旧客户端重新审核或主动投递 | 拒绝产生新的待分拣任务，保留原历史结果并提示先启用 | TestTriageAcceptanceDisabledReopenKeepsRoundAndReadableHistory | Verified |
| TRI-AC-38 | 导入文件含缺标题、日期非法和越权项目的行 | 预览逐行标出原因且不写入任务；确认后只创建有效行，越权项目留空并标明，失败行可下载 | TestTriageImportReferenceRevalidationAndFailureCSV; TestTriageImportHidesForeignAmbiguousAndPrivateCandidates; final CSV Web/visual proof | Verified |
| TRI-AC-39 | 导入行的外部编号已导入过，或标题与正式任务相似 | 外部编号相同默认跳过并标明；标题相似只提示重复候选，不静默丢弃 | TestTriageAcceptanceTitleSimilarityWarnsWithoutDroppingInputs; import dedup/override/concurrency tests; final CSV state labels | Verified |
| TRI-AC-40 | 文件超出行数或大小上限，或不是 UTF-8 编码 | 不进入预览，说明上限或编码要求，不产生任何任务 | TestTriageImportAccessAndEncodingLimits; TestParseRejectsWholeInvalidFiles; frontend fatal UTF8 tests | Verified |
| TRI-AC-41 | 分拣台未启用时通过页面或 API 导入 | 不显示导入入口，API 拒绝并提示先启用 | TestTriageImportDisabledWriteAndMalformedSelection; default-off UI gates | Verified |

Additional mandatory checks: package lint/typecheck, Go vet and tests, safe migration rollback rehearsal, boundary scan, malformed API response, two-client race, CSV limits baseline, Web/Desktop browser evidence.

## FR-only proof obligations

| Requirement | Required proof | Status |
| --- | --- | --- |
| TRI-FR-04 | Every specified filter, stable oldest/newest/priority tie ordering and independent global counts | Verified; see final audit and named proof map below |
| TRI-FR-10 | Duplicate selection by pasted link and formal closed task; deleted target keeps reference | Verified; see final audit and named proof map below |
| TRI-FR-12/13 | One hour/tomorrow/next week/custom presets display actual instant+timezone; cancel/reset snooze audit | Verified; see final audit and named proof map below |
| TRI-FR-15/18 | none/notify/assign modes, human reviewer identity, reassignment and current-member notification dedup | Verified; see final audit and named proof map below |
| TRI-FR-16 | Both batch endpoints reject every non-whitelisted action, explicit valid-only selection and categorized failures | Verified; see final audit and named proof map below |
| TRI-FR-17/26 | Global action/round history survives reopen and contains CSV batch summaries | Verified; see final audit and named proof map below |
| TRI-FR-03/09/26 | Consumed keys survive issue deletion; terminal task and deleted imported row cannot be recreated by retries | Verified; see final audit and named proof map below |
| TRI-FR-19 | Delayed pending-era mention never executes after acceptance; legacy agent identity cannot review | Verified; see final audit and named proof map below |

## Original branch verification evidence

- `make test`: exit 0, complete Go tests with race detection and the ambient-agent CLI guard, including all migration, handler, service and agent packages.
- `go vet ./...`: exit 0; scoped handler/cmd checks also passed after subsequent boundary changes.
- `pnpm typecheck`: all 9 tasks successful after final UI fixes.
- `pnpm lint`: all 6 tasks successful, zero errors; 27 pre-existing warnings outside changed feature code. Scoped new feature lint has zero warnings.
- Bounded full TypeScript suites: core 2,506, views 5,946, desktop 957, web 282 and docs 62 passing tests. The final local UI/route/error-state fixes additionally passed 97 targeted tests; earlier broad suite failures were fixed, not waived.
- Local production-mode Web: 6/6 scenarios passed (14.8s), including settings/intake, rapid search clear and empty focus, snooze/reject/reopen, CSV, actual comment-notification delivery and inbox independence, legacy PUT rejection, explicit resource/executor confirmation with one durable queue row, and Chinese compact validation.
- Native Electron: 1/1 scenario passed (3.8s), using the real built preload/renderer/router and backend in an isolated profile. Unrelated daemon/updater actions are isolated by the existing native fixture; no real agent CLI runs.
- Independent visual review: 94/100 pass across 9 final screenshots; misleading validation recovery and CSV status labels resolved. Remaining P3 notes are optional polish, not material acceptance failures.
- Migration verification includes clean schema application, every retained-data down-step refusal, concurrent first-writer refusal, empty maintenance rollback, and registration of all concurrent-index interruption cleanup hooks.

Current detailed evidence is archived under `.omx/triage-t1/evidence/`; screenshots are under `.omx/triage-t1/final-web/` and `final-desktop/`. The final independent audits are `.omx/reports/triage-t1-acceptance-final-audit.md`, `.omx/reports/triage-t1-visual-review.md`, and `.omx/state/triage-t1/visual-verdict.json`. These runtime artifacts are intentionally ignored by Git; commands and named assertions above remain reproducible from committed source.

Browser provenance: production Next build `PF54g-LpKP8MRAiekljNQ`, API port 18557 / Web port 13477, separate database `multica_multica_triage_t1_477_browser`. Go tests use `.env.worktree` and database `multica_multica_triage_t1_477`. `api.running.json`/`web.running.json` retain PID, source fingerprint, configuration fingerprint and base commit proof. When using the existing environment manager, its broad config fingerprint includes Make's `MULTICA_ARGS`; the matching status invocation preserves the launch value `--web-mode production --name triage-browser`.

## FR-only evidence and applicability

- FR04: `TestTriageAcceptanceQueueFiltersStableSortAndGlobalCounts`, `TestTriageDateFiltersIncludeWholeDayAndExactInstants`, immutable historical-filter tests, and member/agent creator picker regression.
- FR10: duplicate closed/self/foreign/nonformal/deleted target matrix plus pasted-link and deleted-reference rendering.
- FR12/13: actual reschedule/cancel audit/date/deadline matrix, timezone presets, queued polling, browser snooze/reopen and current-reviewer reminder tests.
- FR15/18: none/notify/assign settings, departure, active/current recipient filtering, self-event exclusion, creator subscriptions, DB-clock immediate delivery and 60 repeated passes.
- FR16: backend negative action allowlist, no hidden duplicate-selection writes, mixed per-item outcomes, stable operator summary and selected-only UI retry.
- FR17/26: original-round historical snapshots and filters plus one cumulative CSV summary per batch/recipient; real Web history/CSV evidence.
- FR03/09/26: consumed intake/action/import identities survive issue deletion; lost acceptance/enqueue commit response and terminal execution replays do not create another task.
- FR19: real persisted lower-level admission fences, delayed pre-admission comments, scoped optional source context, service/SQL enqueue/merge/retry/claim/start/token checks, direct old-client PUT409 and formal-work controls.

AC22 is phrased as archive **or** notification snooze. The existing Inbox implements read/unread/archive/unarchive, all proved independent; it has no snooze feature to exercise. Triage task snooze is fully implemented and verified. Iteration membership UI is intentionally absent because I1 is not implemented and T1 is independent; incoming iteration/status columns remain unadopted.

## Performance and limits

Local PostgreSQL + handler baseline, not a production-network SLA: 10,000 formal issues plus 2,000 pending, 10 samples each, P95 list 97.333ms, filtered list 95.966ms, acceptance 18.558ms. Latest 1,000-row CSV sample: preview 605.8ms, commit 5.128s. Baseline JSON is in the backend child task. Locked intake limits: UTF-8, 5 MiB / 1,000 data rows, 500 Unicode title characters, 1 MiB description and 1,000 UTF-8 bytes per external ID.

## Delivery boundaries

T1 was implemented and accepted on `codex/triage-t1`, then merged and pushed to `main` in the subsequent integration recorded above. Production release is unconfirmed and remote CI remains failing at closeout. T2/T3 rules/AI/automatic admission and full mobile editing remain outside this task. Live authenticated agent execution was deliberately not run; fake runtime fixtures prove request authority, context, queued-task identity and replay safety. Local production-mode builds are not evidence of production deployment.

## Code delivery commits

- `b0cda446a`: transactional backend, execution/query boundaries, migrations and server regression tests.
- `36bc06c23`: shared typed client, Web/Desktop UI, notifications and browser/native acceptance scenarios.
- `85b0d2e7e`: main integration merge, retaining the existing administration and MCP work plus the concurrent authentication-upgrade documentation commit.
