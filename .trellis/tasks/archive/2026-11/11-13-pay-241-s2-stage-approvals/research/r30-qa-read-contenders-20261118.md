# Research: R30 QA read contenders and cancellation

- Query: 原R29 E02 QA candidate503附近，清点真实全局user-row竞争writer与原UI取消路径；为ROOT提供最小可选observer与foreground合同判断。
- Scope: internal；仅冻结R29源码/FRONT与已保存API/PG/evidence文件静态读取。UI与完整R4嵌套链由实施peer持有，本报告只补writer/user-lock与cancellation。仅此新report对可写，封存0444。
- Date: 2026-11-18 (Asia/Shanghai).
- Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`.

## Findings

**找到可到达的独占user-row writer链，尚未找到历史503的唯一物理持有者。** 同源candidate/R4在user fence处共享兼容；human-JWT当前校验、notification list和已接受的capability GET不会凭读身份取得user UPDATE。真实managed claim/reclaim/finalize及runtime current-authority writers仍有原FOR UPDATE，这是与R4 SHARE不兼容的合法writer边界。浏览器signal/issue invalidation可取消某些读，但原PG cancellation记录没有请求/UUID/参数关联，不能把它当作两个target503的已证实原因。

### 原始事实与实际caller时序

`G=/Volumes/artisan/.multica-acceptance/pay241-r29-receipt-read-20261118-6dhi01s2/source-green-r01`；`F=/Volumes/artisan/.multica-acceptance/pay241-r24-continuation-20261117-32uud2uo/frontend-base-r01`；ROOT新R30基线`/Volumes/artisan/.multica-acceptance/pay241-r30-qa-read-20261118-pohc3l9t/source-red-r01`仅作后续来源，本研究不修改或运行它。

- 当前R29 original4为 **1PASS/1FAIL/2NOT_RUN，retry0**。E01通过，E02在QA阶段失败；E03/E04没有result attempt，保留原4分母。原R28则仍3PASS/1FAIL/0NOT_RUN，成功原Desktop完成与失败均不重放。
- Q=`6d1b3151-76eb-4fcd-a3d7-a89b6db85927`；R=`da30e86d-1374-4014-ad86-e38b66c72c23`；P/ordinary artifact producer owner=`2800f524-f886-4e8c-a039-d2ca8b7f352e`。root=`01a1eb06-7fce-7553-b4fa-73e0ec4df342`，candidate=`01a1eb06-824a-77d7-8ada-9e4b588123d7`。`b6b85cb3`、`1cecc233`是**request_id**，不是user ID。
- API原log`:9688`记录Q candidateGET503，03:40:46/236.028ms/request b6b85cb3；`:9789`记录同Q candidateGET503，03:40:47/36.032ms/request1cecc233。记录是结束时间秒粒度，不给锁顺序。
- 原spec`:168–173`先创建Q自己的执行workspace/managed runtime/host daemon；`:197–211`生成/释放实际prepared CLI报告，run=`5d9aed56-9dae-456e-bfe0-4fa025fbb8d4`、report=`d3d0d098-616c-48ca-bf14-fd82a5f1dac2`在19:40:46.440已`passed/usable`。`:216`随后`showStage(q)`重新goto再开QA modal；正常资源关闭要到`:220`，失败finally则在19:40:52.522关闭host。不能把P读取的usable report当作Q新candidate请求已成功。
- `createStageTestResource`的heartbeat38.607→38.618唯一一次，interval15s，下一次应在~53.618，资源在52.522已关闭。原API只有03:40:38 heartbeat200/10.566ms；**46/47没有heartbeat RPC**。不能把普通liveness heartbeat指定为47.486 user holder。
- 40–50秒窗口实际有 **Q普通tasks claim46次、capability GET46次、runtime/runs GET46次、notification list GET82次**。真实host claim1次、permit8次、outcome1次、proof/termination等均保留；同窗口没有notification read/archive POST或profile/password/onboarding写。六账号register/onboarding在33秒结束，Web login34–35秒结束。

原PG文件`/Volumes/artisan/.multica-acceptance/pay241-r29-receipt-read-20261118-6dhi01s2/runtime/original-postgres-window-r01/stderr.log`：66,301bytes/42errors。`:356–358` pid60316于19:40:44.011 `LockArtifactUsers`报row relation user取锁失败；`:584–587` pid60444于47.486同样是 **FOR SHARE NOWAIT**。另46.476 pid60409 GetUser、46.657 pid60411 GetCollaborationAccess、46.806 pid60426 ListWorkbench、46.867 pid60438 GetUser、46.875 pid60437 GetPasswordCredential均`canceling statement due to user request`。没有DB名、HTTP request/user/candidate UUID、args或SQLSTATE，不能唯一贴到b6b85cb3/1cecc233，不能把某附近writer命名为holder。

### 真实user-row读/写compatibility inventory

全局user SQL：`agent_artifact_execution.sql:63–71` user/members/password SHARE NOWAIT；`password.sql:16–17` LockPasswordUser FOR UPDATE；`collaboration.sql:22–23` LockCollaborationUsers FOR UPDATE。user SHARE彼此兼容，和UPDATE/NO KEY UPDATE冲突。这里的“writer user fence”通常是**SELECT FOR UPDATE锁**，并不等于修改user列；它仍保护disable/password/binding/current actor时序。

| branch | 原入口 | user fence /compatibility | 证据边界 |
| --- | --- | --- | --- |
| READ-R4-USERS | Stage candidate R4/read actor + retained artifact/report/signers | LockArtifactUsers -> FOR SHARE NOWAIT; password credential SHARE NOWAIT |  |
| WRITE-ORDINARY-CLAIM | POST /api/daemon/tasks/claim -> ClaimTasksByRuntime -> ClaimTasksForRuntimes |  | An empty tasks[] HTTP response alone cannot prove no account fence: due reclaim takes LockManagedRuntimes before its reclaim query. Empty maxTasks/cache fast paths and no selected candidate can avoid some branches. No actual successful lock/branch/PID was captured for these observed HTTPs. |
| WRITE-RUNTIME-PROMOTION | GET /api/collaboration/runtime/runs | When waiting promotion exists, current=true,daemon=true -> GetCollaborationWorkflowUserIDs -> LockCollaborationUsers FOR UPDATE -> managed owner FOR UPDATE | GET spelling does not make its admitted promotion a pure read. Waiting-list empty is a pure scan path; HTTP200 lacks branch evidence. |
| WRITE-RUNTIME-CLAIM-PERMIT | actual host run claim + permit | current=true,daemon=true -> all workflow users FOR UPDATE then managed owner LockPasswordSession FOR UPDATE | Successful original operations finished before final failed Q GETs in saved second-resolution logs; do not pretend they were still holding their locks. |
| WRITE-RUNTIME-OUTCOME | POST /api/collaboration/runtime/runs/<run>/outcome | new outcome current=true,daemon=false -> workflow users FOR UPDATE; recorded-event acknowledgement returns before this path | Read final report is usable at19:40:46.440; outcome249.63ms may be an earlier same-second contender. No physical lock-release/holder witness. |
| PROOF-WRITERS-SKIP-USERS | launch termination / proof / stop-intent | current=false,daemon=false explicitly skips workflow user locks; other issue/session/run/terminal locks persist | They are real writes/reads but not evidence of an exclusive global user-row holder by this prefix. No broad proof path expansion is proposed. |
| READ-CAPABILITY-GET | GET /api/collaboration/runtime/capabilities | private readAccount=true -> LockArtifactUsers([Q]) FOR SHARE NOWAIT; public LockManagedRuntime stays false for writers | Accepted R22 read/read compatibility is retained. No new declaration that all runtime GETs are mutation-free. |
| HEARTBEAT | POST /api/daemon/heartbeat / existing runtime liveness scheduler | Managed auth performs current version/binding SELECT checks; liveness heartbeat updates agent_runtime.last_seen_at/status via scheduler, not user by its hot path | Do not nominate the plain liveness beat as the47.486 global-user holder. This does not remove account fences from actual claim/registration/minting. |
| NOTIFICATIONS | GET /api/me/collaboration-notifications vs POST read/archive | ListNotifications sets RR READ ONLY:157, ordinary GetUser/current authorize SELECT:134-142; no user UPDATE fence. | Neither82 GETs nor notification delivery prove a global-user writer. No state POST appears in40-50s retained API log. |
| PUBLIC-HUMAN-SETUP | human-JWT authentication / public registration-login-onboarding / fixture email setup | JWT current check GetUser then GetPasswordCredential are plain SELECT; no LockPasswordSession in this middleware path. | Physical setup lock-release was not traced, but sequential completed setup requests are not evidence of retained holder11seconds later. No account secret scan or setup replay. |

核心可选observer对象是已观察的普通claim：

```text
POST /api/daemon/tasks/claim (authenticated managed Q)
Handler.ClaimTasksByRuntime — daemon.go:1841
  TaskService.ClaimTasksForRuntimes
    due reclaim — task.go:3992–3999 — LockManagedRuntimes
    actual selected claim — task.go:4098 — claimTask -> LockManagedRuntime
    actual finalize — task.go:3764,3798 — LockPasswordSession/LockManagedRuntime
  managed_runtime.go:199 -> public LockManagedRuntime:123–124 (false)
    private lockManagedRuntime:136 -> auth.LockPasswordSession
  auth/password_session.go:184 -> LockPasswordUser
  queries/password.sql:17
    SELECT id FROM "user" WHERE id=$1 FOR UPDATE
```

**即使response tasks=[]，due reclaim也可先拿该user fence。** 同时maxTasks0、empty cache、没有selected candidate又可能绕过特定分支；原HTTP日志没有response任务数、canonical query成功事件或事务PID，46次HTTP不能等同于46次已证明持锁。

直接原log`:9769`：03:40:47 Q POSTclaim200/457.735ms，request`yangshangweideMac-mini.local/zQK9H7dDC0-003286`；`:9784`同Q claim200/127.966ms/003312；`:9790`46.535ms/003315。它们是精确eligible contenders，**不是物理holder证据**。

另runtime GET/runs并非全程纯读：`collaboration_runtime.go:645–651`只有waiting promotion才进入`:694`current=true/daemon=true；source prefix`:126–133`取得root creator/handler/所有active participants/owner的UPDATE（SQLworkflow`:3–10`，本scene含P/R/Q），再拿managed Q UPDATE。实际host claim/permit也current=true/daemon=true；new outcome`:455`current=true/daemon=false锁workflow用户。Proof/termination/stop-intent则明确false/false，skip users；它们的其他source/run锁仍保留，不把它们当globaluser holder。

Notification list`:147–196`是RR READ ONLY，GetUser/current version SELECT；只有state POST read/archive`:214`拿viewerUPDATE，这个窗口未观察到state POST。通知INSERT/WS发送若由comment/runtime write触发，竞争权来自**父事务**原user prefix，而非notification list本身。HumanJWT middleware`:77`→CheckPasswordJWT/currentGetUser/GetPasswordCredential是普通SELECT；专门Login/mint/change的LockPasswordUser、onboarding/profileUPDATE以及fixture synthetic emailUPDATE属于各自真实写，均已在前期setup完成，没有本window重放。

### cancellation：可到达路径与明确不成立的推断

- Core R4 `stage-queries.ts:17–21,136–147`放在issue Query subtree，queryFn消费signal；Api client R4→`fetchRaw:2687–2694`将其与endpointController合成真实fetch signal。当前issue/workbench读也传signal。
- `realtime/collaboration.ts:52–55,71–79`的fresh changed/children事件、`:118–123` subscribe_ack会invalidate整个issue subtree；`:125–132`重连onAuthenticated先显式cancel scope再invalidate，连首次无cached data读也包含。空workbench tick仅refreshAccount，predicate排除`key[3]==issue`，**该tick单独不invalidateR4**。
- 实际消费TanStackquery-core **5.96.2**：`queryClient.ts:293–328` active invalidate→refetch默认cancelRefetch=true；`query.ts:408–415`仅在仍fetching且已有cached data时取消旧fetch，否则复用当前promise。不能说每个invalidation一定abort初次GET。
- 读取signal设置consumed；last observer removal`:359–370`可cancel。原`showStage`是page.goto，Q当前QA前导航亦可取消旧document请求。`invalidateSession` abort endpointcontroller；source/access真拒绝会cancel并fence generations。**503不是source denial**，不得擦除原意图或复活私密cached data。
- `stage-queries.ts:116,155`两个version同步明确cancelRefetch=false，不主动取消active fetch。retry=false仍0error-auto-retries；15秒poll/focus/realtime驱动的是新的read，不是自动重试一个失败写。
- 原46秒PG“user request cancellation”证明有cancellation，但没有browser requestfailed/signal reason/event/SQL参数关联。即使QueryEnd/HTTP结束邻近，也不能认作target503或当前modal取消原因。

### foreground行为：real writer refusal允许，Refresh必须靠当前读

规范`collaboration-stage-approvals.md:118,250–251`及R22`TestCollaborationRuntimeCapabilitiesReadOriginalWriterFence:170–208`明确：真实writer先拿独占fence，R4拒绝、零私有projection；不得自动retry、放宽claim/权限锁或用oldcachedauthority批准。这不是read/read共享兼容缺陷的同义词。

原modal`stage-code-forms.tsx:36–46`显示query.error→`CollaborationQueryState`显式query.refetch，并以query.isError锁写；code-panel`:33,60`在error时不把record作为当前证据；query-state`:5–9`已有genericloaderror/Refresh。P的usable report与Q的failedcurrentR4可以同时存在。**现有合同容许临时writer/refetch-cancel后unavailable+explicitRefresh，不能据modal error直接指定锁修复。** 需要检查fresh显式Refresh在对应writer/cancel结束后是否以当前有权200恢复，是否实际陷入空claim独占/重复取消/不可达Refresh；仍保留原scene断言/期限及0auto retry。

### ROOT最小optional observer：exact Q tasks claim，保留纯转发

本建议不执行也不编辑ROOT获准的test paths。原primaryscope只root/R4的firsterror无法看见**成功持锁的writer**，可选增加：

1. **exact POST `/api/daemon/tasks/claim`**，只认一个freshscene的实际authenticated managed Q与其declaredruntime；新fixture动态绑定，不能硬编码旧Q或信任X-User-ID。无需genericallruntime，也不加4096大cap。
2. 保留HTTP requestID/method/path/safeUserID/credentialKind/runtimeUUID，及同context真实backendPID/Tx身份。记录该scope **成功LockPasswordUser** entry/end、canonicalname/hash、safeuserUUID/TxStatus；只加必要Begin/Commit/Rollback dispatch/end和firstrealPgError。可选canonical reclaimquery label区分due branch，不能copytoken行/args。
3. 相邻短窗口例如最多32matchingrequests×每请求4selectedevents；actualresponse仍原body/headers/status，既有content-freeSHA/count可保留。无accountbearer/password/token/private report内容，无SQL/响应stub或行为变更。
4. primaryR4同时要firstactualerror/query/UUID/PID；frontend需requestID/QueryKey/cancelreason/触发event/导航时间。先证明当前R4究竟被writer fence拒绝、被supersede/abort，或另一个路径失败，再限定产品/测试处理。
5. **QueryEnd不是release；CommitEnd也是client ack，非确切PG锁释放点。** 捕获实际commit/rollback dispatch与Txn状态/bothPIDs，只声称操作边界。若需物理唯一holder，ROOT需其获准sameDBlock/xact witness或确定性borrowedTx barrier；本研究无SQL probe。R4已取得Q SHARE后，后到Q UPDATE应等待/拒绝；新的claim也可能在另一个/之后R4取得fence前进入，不把新claim时间或HTTP结束时间倒推成前一个failedGET的原因。

### Canonical controls与后续最小切口

保留已有实际R21–R29source/raw绑定，不自动重跑原116或改writer默认。精确canonical切口：

- `server/internal/service/collaboration_runtime_capabilities_read_concurrency_test.go:103,136,170,212`：四top **4/8/7**，sameQ capabilityGET/R4两顺序、真registrationwriter/当前authority。它已证明capabilityGET纯读兼容，不证明这次ordinaryclaim holder。
- `collaboration_stage_evidence_read_concurrency_test.go:137,159,180,231`：sameartifact R4/R4T4、writer fences与RR revocation；`collaboration_stage_execution_read_concurrency_test.go:139,179`两top **2/8/6**保留plain/marked原writer与retainedissuer fence。
- `managed_runtime_test.go:165`，`TestManagedClaimCapturesBindingAndDoesNotBackfillReclaimedHistory` **1/1/1**：actualclaim/finalize当前binding和历史reclaim不backfill；不能为R4成功破坏该contract。
- Core `collaboration/stage-queries.test.ts:185`的 `R4 protected candidate read > uses the issue invalidation prefix, forwards cancellation and erases evidence on current object denial`；`realtime/collaboration.test.ts:53,72,171`的原POST保持、source refresh和GET-only reconnect。它们是client合同，非历史browser/SQL物理cause证明。

若freshobserver证实all competingreads均SHARE而没有actualQclaim/otherrequireduserUPDATE，只能记录“read兼容/原因仍未知”，继续exactrequest/query cancellation evidence，不能unsupportedfix。若证明真实writer contention，原fence/零projection应保留；下一步是foregroundexplicitRefresh可恢复性与emptyclaim实际branch/公平性定界。唯有新确定性两连接case或exactHTTP/PG证据证明有新增read误用exclusive/helper或持久不可恢复行为，才由ROOT给owner更窄修复范围。

## Files / specs / external references

CompanionJSON保存originalresult/E02evidence/API/PG文件hash、exactfailure/claim47/heartbeat records、windowbranchcounts、安全actors、上述caller/source和optionalobserver。无externalnetwork；TanStack版本来自实际消费package.json源码。PostgreSQLrow-fence语义由原SQL与两连接controls/spec引用，不把窗口缺失SQLSTATE补成历史target55P03。

## Caveats / Not Found

- 无唯一historicalholder、DB/request/UUID/args/SQLSTATE/physicalrelease或browsercancelreason关联；B6/1ce503不能直接贴上pid60444/60316。没有完整UI/R4重复研究或实现。
- ROOT/currentR29local76+6 normal/race/source acceptance仍是其限定证明；currentoriginal4是1PASS1FAIL2NOT_RUN，旧R28仍FAIL。S2/A01–A12、parent24与S3–S5保持既有门槛。
- 没有Go/test/list/TestMain/DB/SQL/browser/API/runner/build/process/signal/Git或subagent执行；未改产品/测试/spec/progress/task/旧报告/peer；仅这两新report0444。Observer/freshruntime/任何修复均由ROOT决定和执行。
