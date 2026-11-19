# S2 最终验收矩阵预检查（2026-11-14）

**结论：可以按以下完整清单接续最终验收；当前不能接受 S2。** 本轮只审阅已有合同、冻结结果与入口地图，未修改产品/测试、未运行数据库/API/browser，也未重复执行检查。2d正在由同一服务端owner实施，本文不把其中间态列为已证实产品缺陷。

分母分别保留：**A01–A12共12条、已编写E2E共4场景且实际0、实际入口W01–W18共18项、跨事务检查Q01–Q12共12项**。后两项是覆盖清单，不是新增业务任务或新增父任务用例。父24例仍单独记账。

已有独立通过只属于各自冻结：S2B-C01修复、2c后续C01/C02、client3d与E2E4编写检查。server2c最终4,939文件，client3d2,532文件，E2E4图2,552文件。最终candidate还必须接入2d并重新绑定。

## A01–A12对应关系

| ID | 要求与已有规范用例 | 最终仍须证明 |
|---|---|---|
| A01 | 角色配置、稳定责任人与当前处理人分离；当前授权变更和留痕<br>`TestStageRolesRequireSourceAdminParticipantsAndSeparation`、`TestStagePolicyRevisionCASAuthorityAndOriginalKey`、`TestStageHTTPActionsConfirmationsRefusalsAndRecovery` | 在最终源码重跑角色/政策/原键用例；实际共享UI启用P2、配置、拒绝自评与QA/release同人，核对activity当前actor及稳定owner。 |
| A02 | 不可变建议的接受、修改后接受、拒绝；并发单次物化且零自动派发<br>`TestStageProposalVersionsDecisionsAndSingleMaterialization`、`TestStageProposalConcurrentAcceptCreatesOneChildSet`、`TestStageProposalAcceptRollsBackEveryChildOnRefusal`、`TestStageAcceptRequiredWriteFailuresAndLostCommit`、`TestStageChildCreationRequiresApprovedProposalForMachines`、`TestStageDescendantEnqueueEntryPoints`、`TestStageChildStartSerializesWithRootLifecycle` | 实际UI修改新版本→接受同一组child IDs、拒绝版本和原键重放；后端保留机器create/attach/queued→started与重开竞争，人工手建/普通child控制组。 |
| A03 | 缺失/失败/错head/陈旧或不受信CI不能进入代码评审<br>`TestStageCodeReviewStrictCIRefusals`、`TestStageCodeReviewDigestAndSourceDrift`、`TestStageCodeCandidateRejectsUnboundInputs`、`TestStageCodeVCSWithoutRepositoryCheckIdentityIsUnsupported`、`TestStageCodeReviewFencesChecksAndTrustThroughConsumption` | 最终服务/HTTP回归与实际UI缺失→失败→合成完整CI；保留candidate/head/trust/check-set身份和阶段未变化。合成CI不是S4真实远端CI。 |
| A04 | 没有有效人工代码评审不能进入QA；自评、机器伪装、撤权拒绝<br>`TestStageCodeReviewTransitionsAndOriginalKey`、`TestStageCodeCandidateVersionsAndReviewerIsolation`、`TestStageCodeReviewSignerRevocationAndExpiryBounds`、`TestStageCodeReviewSourceMembershipRejoinDoesNotReviveApproval`、`TestStageAdvanceRechecksOriginalApprovalAtFinalClock` | 实际指定reviewer签字及原键恢复；服务/真实auth router覆盖自评、AgentActor/机器凭据、账号/成员/角色撤回后不能消费。 |
| A05 | 原运行/失败case/log不可变；QA退回及新测试保留历史<br>`TestStageTestReportActualInputClaimAndReceipt`、`TestStageQAPassedReportNeedsSuccessfulSettledExecution`、`TestStageTestReportOriginalCandidateConcurrentRecoveryAndPrivacy`、`TestStageTestReportLostCommitRecoversOneOriginalEvent`、`TestStageTestReportHostCheckpointExactLogAndRedaction`、`TestStageArtifactReportReadAuthorityMatchesR4`、`TestStageArtifactReportQAReferenceUsesCurrentManager` | 实际Git bundle→首次claim subject→baseHost run→CLI T1/T2/T3→T4失败case/log；裸run不能当QA报告，passed报告但run失败/未settled不能批准；reason-only return保留。 |
| A06 | QA和构建必须匹配才能发布批准；发布权与QA/Agent分离<br>`TestStageExecutionReleaseRollbackAndOriginalReceipt`、`TestStageExecutionSigningStateDriftPreventsDispatch`、`TestStageExecutionOriginalArtifactReaderRevocation`、`TestStageExecutionR4ReflectsTransitiveArtifactAuthority`、`TestStageHTTPExecutionEvidenceExtensions` | 实际共享release签字、QA越权/错build拒绝；重跑已有原issuer撤权回归，保留target_digest/QA dependency/真实tar摘要。 |
| A07 | 回滚建议、批准、一次调用与实际结果分开，绑定原成功部署和立即前一制品<br>`TestStageExecutionTargetAndPhysicalIdentity`、`TestStageExecutionUnknownBlocksRebindingAndCrossWorkspace`、`TestStageExecutionMetricAmbiguityInvalidatesExistingRollback`、`TestStageExecutionMetricReverseOrderAndOlderArrivalCannotResolveAmbiguity`、`TestStageExecutionConcurrentPrepareHasOneEntry`、`TestStageExecutionCommitUncertaintyAndPreEntryCancellation`、`TestStageExecutionProductionUnavailableAndSourceDeniedProjection` | 实际私有control harness验证pending/unknown非成功，稳定物理target冲突，equal-native-version持久歧义，新metrics及精确rollback正例；生产constructor无adapter仍不可执行。 |
| A08 | 版本/角色/权限/到期/载荷变化拒绝；原意图仍可按权限恢复<br>`TestStageCodeReviewConcurrentIntentAndLostCommit`、`TestStageScopeRoleRestorationDoesNotReviveApproval`、`TestStageScopeMemberFenceAtActualDecisionWrite`、`TestStageAdvanceRechecksOriginalApprovalAtFinalClock`、`TestStageExecutionCommitDelayRetiresExpiredEntry`、`TestStageExecutionLateAuthenticatedFactDoesNotRenewApproval`、`TestStageExecutionFinalizationSharesOneDeadline`、`TestStageReviewRequestPreservesPreExecutionCanonicalBytes` | 最终2d新增submit/done最终时钟与before→allowed-after、历史replay、reopen新scope矩阵；实际迟到部署只留历史，无续期/背书/再次调用/自动推进。 |
| A09 | 所有实际完成/退出入口保持stage+M1与R38原边界<br>`TestDeliveryCompletionCollaborationActualSourceAndOriginalRecovery`、`TestDeliveryCompletionCollaborationGrantAndRefusalDoNotBindAttachments`、`TestDeliveryCompletionCollaborationCurrentBindingsRefuse`、`TestDeliveryCompletionR38AuthorizationHonorsCurrentConfiguration`、`TestDeliveryCompletionAuthorizationRollbackRevokeAndTransactionIdentity`、`TestDeliveryCompletionAuthorizationTerminalInsertRechecksExpiry`、`TestDeliveryCompletionCollaborationRouterActualSourceAndRecovery`、`TestDeliveryCompletionEnforcementRouterOrdinaryAndMove`、`TestDeliveryCompletionEnforcementRouterBatchCommittedPrefix`、`TestDeliveryCompletionEnforcementRouterCreateAndCommentChild`、`TestDeliveryCompletionWebhookRouterExactGrantConsumesWithState` | 2d当前待实现/验证，必须完成下方W01–W18全部入口与Q01–Q12事务交叉断言。原M1未启stage用例只作兼容控制；E04只有submission完成正例和self/R38负例。 |
| A10 | 私有证据读取按真实主体与当前权限隔离，中途撤权无泄漏<br>`TestStageSourceDeniedDigestAndCachedScopeSelection`、`TestStageArtifactReportReadAuthorityMatchesR4`、`TestStageArtifactReportAuthorityLocksAcrossReadAndQA`、`TestStageArtifactReportRevocationBeforeAuthorityLockDiscardsSnapshot`、`TestStageArtifactReportCachedCursorDoesNotRestoreAuthority`、`TestStageArtifactReportEmptyListStillChecksFrozenSource`、`TestStageArtifactReportHTTPReadAuthorityEnvelope`、`TestDeliveryCompletionCollaborationRouterPrivateEvaluationAndAuthentication` | 保留S2B-C01原冻结artifact ACL、stale可读/普通member拒绝/list-detail-cursor-emptyQA引用与race；2d新增root/source/R38候选preview读取ACL和迟到响应隔离，P≠E最小grant receipt不得泄露制品。 |
| A11 | 一致源码production Web与Desktop实际配置/批准/拒绝/恢复<br>构建/运行/独立证据 | 必须四场景实际执行4/4；绑定最终source、API/CLI/Git/baseHost/router/Web BUILD_ID/renderer/preload、配置、actor、原key/请求体/双回执和资源清理。测试人类签字明确synthetic；native harness不等于完整安装Main验收。 |
| A12 | 完整独立Trellis检查与父矩阵更新，保持真实外部边界<br>构建/运行/独立证据 | 2d独立检查、最终A01–A12证据复查、完整差异/ownership/static/schema/运行清理检查均通过后才S2归档；父24例分母及S4真实NOT_RUN不变，S5最终候选仍需重跑。 |

所有A条目当前状态均为`PENDING_FINAL_ACCEPTANCE`。用例的文件和行号在JSON中，来源为已检查2c副本；这些名字证明存在的回归切口，不声称它们已经针对2d运行。

## 四个真实场景（当前全部NOT_RUN）

- **E01** — `e2e/collaboration-stage-approvals.spec.ts:27`：stage decisions and prepared test reports retain human and runtime history across Web and Desktop。
- **E02** — `e2e/collaboration-stage-execution.spec.ts:233`：release and rollback keep exact builds, human decisions and controlled execution separate on Web and Desktop。
- **E03** — `e2e/collaboration-stage-execution.spec.ts:349`：a late authenticated deployment stays historical and a fresh approval cannot endorse it。
- **E04** — `e2e/collaboration-stage-execution.spec.ts:398`：stage evidence gates real submission, M1 completion and administrative or project exits。

E01覆盖A01–A05及权限/原键恢复；E02覆盖release/rollback与歧义指标；E03覆盖迟到执行仅历史；E04覆盖stage不完整的submit/self/R38与项目退出，以及Web→Desktop共享M1 submission正例。**E04没有self/R38完成正例，也没有P≠E签发到消费正例。** 这些正例必须从2d后的真实service/handler/router补齐。

已存在的`TestDeliveryCompletionCollaborationRouterActualSourceAndRecovery/{self,submission,admin}`明确使用不同source issuer和无来源成员身份的initiator，并通过真实router/实际tar/grant/lifecycle/原回执；它是可复用的M1控制组，必须增加/接入stage-enabled fixture才证明两道门禁组合。不能把原M1未启stage通过直接贴到A09。

## 实际入口完整清单

### W01 协作正式交付submit

路径：`server/internal/handler/collaboration_workflow.go:SubmitCollaborationAcceptance → server/internal/service/collaboration_handoff.go:SubmitAcceptance → mutateCollaborationWorkflow`。

M1三模式下stage不完整都409，附件/新submission/handler/status零效果；合法submission持久绑定唯一精确candidate并在最终提交时钟再核查。

证据切口：E04负例+实际submission正例；2d事务单测待补/绑定。

### W02 协作精确submission完成

路径：`ChangeCollaborationLifecycle → CollaborationService.ChangeLifecycle → PrepareDeliveryMutationInTx → prepareCompletion → changeLifecycle → 最终stage guard → outer commit`。

同精确submission，stage有效才允许；enforce原grant一次消费；Web失败前转发→GET404→Desktop同key/body完成→Web双回执。

证据切口：E04正例；后端三模式及最终guard待2d。

### W03 协作self-result完成

路径：`ChangeLifecycle/prepareCompletionMode(false) → PrepareDeliveryMutationInTx → prepareCompletion`。

结果/附件先只读验证，拒绝零绑定；合法路径真实生成自己的submission；阶段三模式都生效，enforce原M1仍独立。

证据切口：E04仅负例；实际stage-enabled正例需后端。

### W04 R38失能发起人管理完成

路径：`server/internal/service/admin_collaboration.go:ChangeLifecycle → PrepareDeliveryMutationInTx → recordAdminCollaborationHistory/CompleteCollaborationOperation → commit`。

当前真实password admin/organization且creator disabled；无stage豁免；合法三模式/admin grant正例，不造initiator accepted；最终guard在admin历史和原回执写完后。

证据切口：E04仅负例；stage-enabled正例与P≠E需后端。

### W05 普通PUT/旧client/Agent的完成请求

路径：`server/internal/handler/issue.go:UpdateIssue → updateIssueAtomicallyOnce`。

协作root保留既有不支持普通mutation边界；普通task与普通child不被强加整个root阶段流程；M1/custom-done、事务guard及Agent真实身份不退化。

证据切口：原M1 ordinary/router控制组+2d回归。

### W06 普通Move

路径：`server/internal/handler/issue_move.go:MoveIssue → UpdateIssue → updateIssueAtomicallyOnce`。

同W05完成器；真实Move不能绕开root限制或M1，合法child move使旧语义candidate stale；并发后再读。

证据切口：TestDeliveryCompletionEnforcementRouterOrdinaryAndMove+stage admission。

### W07 普通批量更新

路径：`server/internal/handler/issue.go:BatchUpdateIssues → updateIssueAtomicallyOnce per item`。

root批次在副作用前拒绝；普通逐项committed/rejected/not_attempted/unknown前缀保留；重复authorization/key在写前拒绝；合法child不全禁。

证据切口：TestDeliveryCompletionEnforcementRouterBatchCommittedPrefix+TestOrdinaryDeliveryBatch*。

### W08 Create/快速创建/comment-to-child/create-as-done

路径：`server/internal/service/issue.go:IssueService.CreateInTx → checkStageChildCreate → CheckDeliveryCreateInTx`。

先解析继承project；机器拆解需要内部proposal许可；人类手建不造批准。协作root本来不能初始done；普通child由既有M1 create-as-done规则管，不强加root verification。

证据切口：StageChildCreation/StageAdmission/EnforcementRouterCreateAndCommentChild。

### W09 GitHub/VCS/system状态writer

路径：`server/internal/handler/github.go:advanceIssueToDone; vcs_webhook.go → server/internal/service/issue_status_write.go:WriteIssueStatus`。

IsCollaborationIssue稳定拒绝system完成root；普通child GitHub/VCS保持exact trust/signature/唯一grant与M1；所有新写的最终guard同tx，不把202当done。

证据切口：TestDeliveryCompletionWebhookRouter*与WriteIssueStatus回归。

### W10 完成CLI及run-local machine边界

路径：`server/cmd/multica/cmd_issue_completion.go:runIssueCompletionSubmit → ordinary issue PUT；cmd_collaboration.go运行能力allowlist`。

已有ordinary CLI不得把collaboration variant强转/创建第二完成器；mcb命令allowlist不能签真人或完成root。实际匹配CLI报告可用，越权completion拒绝且零副作用。

证据切口：CLI纯协议回归+最终匹配API负例；不新建CLI授权体系。

### W11 协作root project move/clear

路径：`server/internal/service/collaboration_edit.go:EditIssue → CheckDeliveryProjectChangeInTx → workflow commit`。

stage启用时M1 off/observe/enforce均stage_policy_exit_required；版本CAS+理由显式关政策后才按原权限移动；root状态/回执不伪成功。

证据切口：E04 move/clear负例+2d事务竞争。

### W12 项目delete-impact和实际delete

路径：`server/internal/handler/project.go:GetProjectDeleteImpact / DeleteProject → CheckDeliveryProjectDeleteInTx`。

启用stage的项目即使M1 off也报告阻挡并零删除/清空；显式关闭仍受project_has_collaboration_issues等原限制；policy竞争锁至commit。

证据切口：E04 impact/delete负例与unmanaged empty项目204；2d lock/shape。

### W13 managed root普通delete与batch-delete

路径：`server/internal/handler/issue.go:DeleteIssue/loadIssueForUser；BatchDeleteIssues/rejectLegacyCollaborationBatch`。

root仍unsupported_collaboration_action；批次/普通父含collaboration child在CancelTasks/FailAutopilot前拒绝，事务内复检，不能先产生取消效果。

证据切口：原collaboration边界+2d真实入口回归。

### W14 普通child语义编辑/换parent/project/删除

路径：`CheckStageChildUpdateInTx/updateIssueAtomicallyOnce；deleteIssueAndCollectAttachmentURLs/deleteIssuesAndCollectAttachmentURLs`。

普通child按原权限可操作；内容/project/parent变化使semantic_version或缺失观察失效，不能靠拆parent抹映射。仅状态更新不使已审代码自身失效。

证据切口：StageChildUpdateKeepsMapping +2d完成消费旧candidate拒绝。

### W15 真正enqueue与queued→started

路径：`TaskService.checkIssueExecution；EnqueuePreparedIssueTaskInTx；startTaskAtomically`。

未批准机器child、已过期root lifecycle映射拒绝；接受proposal零自动enqueue；合法已确认普通child仍能按原invoke规则启动；两种并发顺序。

证据切口：TestStageDescendantEnqueueEntryPoints/StagePreparedEnqueue/StageChildStartSerializesWithRootLifecycle。

### W16 cancel与reopen

路径：`CollaborationService.ChangeLifecycle / admin原允许动作 → changeLifecycle`。

cancel独立不被stage阻止；reopen回development、stage_revision递增、新lifecycle需新A5；旧批准不复活但已消费原key历史仍恢复。

证据切口：StageScopeConfirmationFirstEnableAndReopen+2d真实lifecycle。

### W17 root/R38 completion candidates与preview

路径：`冻结F06四路由 → root/source同事务当前权限+M1 observer+stage blocker`。

numeric root/child revision，直接普通child同workspace/project；done child可提供artifact；preview无授权/写入；stage不完整三模式200非授权；源拒绝无泄漏，root不存在/旧route区分。

证据切口：client3d检查已通过；2d真实router/normal-race待补。

### W18 活动与原操作回执投影

路径：`collaboration workflow event/finishWith、activity allowlist、GetWorkflowOperation/O1、原M1 GET`。

closed stage metadata不带私有hash/prose/report，execution仅hint；stored stage_blockers按wire保留且O1不能掉字段；原key恢复不写事件/重复消费。

证据切口：client3d activity/receipt检查；2d服务投影及最终实际timeline待补。

## 必需事务与权限交叉断言

- Q01 stage enabled × M1 off/observe/enforce：submit/submission/self/admin每个适用writer分别验证stage不完整拒绝与合法正例，enforce缺grant仍拒绝；不把重复fixture次数当新业务分母。
- Q02 私有stage guard绑定原pgx.Tx与before及该writer允许after；错误tx/复用guard/未finalize/无guard不得放行。M1 NotApplicable仍跑stage最终检查，Replayed不做新消费。
- Q03 最终database clock在全部状态、handoff/submission、attachment、iteration、Stop/queue、event/outbox、原operation/admin审计写完后、commit前；原审批/issuer/caller expiry、role/credential/source/trust更换两种顺序都覆盖。
- Q04 submit持久绑定candidate/lifecycle/语义证据；legacy或未绑定submission在启用stage后stage_submission_unbound，不能根据当前候选事后补绑。
- Q05 consumer E与source issuer P分离：E无source membership且仍当前initiator；P当前可读artifact通过既有授权API签给E；E仅读最小grant并实际完成。错consumer、P撤权/过期、E撤权、错误组织admin均拒绝，零隐私/副作用。
- Q06 preview只读及附件只读：0 authorization/evaluation/submission/status/attachment-link/queue writes；done child不能早退root preview；绑定root/child数字revision和精确source。
- Q07 HTTP strict unknown/duplicate/case alias/trailing JSON、64KiB及分页1..50、cursor root/source/viewer/revision绑定；list/detail/empty/cursor mid-revoke竞争均保留。
- Q08 late error或commit unknown不自动重试；已提交refusal原status/code/blockers冻结；原GET404无POST；已consumed key在reopen/当前stage变化后仍历史回执且当前权限重检。
- Q09 stage schema readiness与所有实际writer依赖一致；缺表/索引/trigger/partial migration为不可用不是off。全部writer齐备验证后才stagePolicyActivationAvailable=true，再真实P2启用，不能测试开关绕过。
- Q10 项目policy change/root move/clear/delete与完成/提交锁争用；同target物理冲突不能经localbinding/workspace/trust改键绕过；普通child独立生命周期/手建控制组保持。
- Q11 2d新DDL单条CONCURRENTLY索引全部注册cleanup、真实shape/noFK、empty down/up及各直接table-drop保留历史拒绝；不得执行并行knowledge844–860+迁移。
- Q12 配置/源码/数据冻结后才执行四场景；进程只关自有，gate恢复allocation原booleans且epoch单调；原始fail/skip/NOT_RUN、源版本、完整分母不覆盖。

## 主会话执行顺序与命令

1. 等2d owner冻结DDL/sqlc/源清单；root重新核对并行migration号码，只迁移明确的S2文件。八个已有profile截至2c为843；真正执行前核对当前DB身份、零占用及完整schema。最多两个包套件同时运行。
2. 独立复查2d的W/Q清单后，绑定最终测试名称和实际源码。先源码提取顶层用例清单，再用最终go JSON事件核对每一顶层及子例的pass/fail/skip；缺名、skip、TestMain提前退出不能算通过。当前已冻结stage库存：service72顶层、handler12顶层、router6顶层（含一个opt-in长期harness）；2d后数量应由最终源码重新提取，不锁死旧分母。
3. 下面为已分配profile下的命令形状；`PAY241_FINAL_SOURCE`必须是root冻结副本，不是混合dirty checkout。原normal/race profile按package使用，不能以-run换包。新2d测试如不匹配选择器，按最终owner清单明确补入。

```sh
node /Users/artisan/.multica/acceptance/pay241-20261113-01/run.mjs /Users/artisan/.multica/acceptance/pay241-20261113-01/configs/s2_impl_service.json go -C "$PAY241_FINAL_SOURCE/server" test ./internal/service -run "^TestStage" -count=1 -json
node /Users/artisan/.multica/acceptance/pay241-20261113-01/run.mjs /Users/artisan/.multica/acceptance/pay241-20261113-01/configs/s2_impl_handler.json go -C "$PAY241_FINAL_SOURCE/server" test ./internal/handler -run "^(TestStage|TestDeliveryCompletion|TestOrdinaryDeliveryBatch)" -count=1 -json
node /Users/artisan/.multica/acceptance/pay241-20261113-01/run.mjs /Users/artisan/.multica/acceptance/pay241-20261113-01/configs/s2_2c_router_r01_safe.json go -C "$PAY241_FINAL_SOURCE/server" test ./cmd/server -run "^(TestStageExecutionRouter|TestCollaborationStageRoutes|TestDeliveryCompletion(CollaborationRouter|EnforcementRouter|WebhookRouter|ManagedRouter|AuthorizationRouter|PreviewRouter))" -count=1 -json
```

对应race使用`s2_race_service`、`s2_race_handler`、`s2_2c_router_race_r01_safe`并加`-race`；不改变原timeout/pool/清理规则。长期`TestCollaborationStageExecutionHarness`只由root单独启动，其opt-in、private control pipe/socket、固定30秒entry与5秒共享finalization预算不能混入普通包测试。

4. 运行最终Core/Views类型、lint、canonical测试与完整构建，保留普通M1兼容测试。可复用已检查配置和当前pnpm lock；外部HOME/TMP及workspace链接只指向冻结副本。Go vet/build、gofmt、新迁移shape/cleanup和源码边界检查必须有最终源证据。无新变化/未决问题不必机械重复旧整库测试。
5. 用最终API/router-harness、CLI、Git、baseHost、production Web与匹配Desktop renderer/preload执行两spec的四场景，workers=1/retries=0；显式`COLLABORATION_STAGE_ACCEPTANCE=1`，不得skip配置失败。预置合成admin走既有真实初始化/登录/密码要求；E04缺admin必须失败而非skip。
6. 每场景保留原请求key/body/hash、真实回执、persisted state、失败/超时、应用/fixture/binary/source/config身份及清理结果。E2E4-Web拦截是转发前aborted，Desktop才有一次到达API的final writer；不能把两个浏览器attempt合并成同一种计数。
7. 最终独立review验证A01–A12、日志与源码绑定、所有writer/effects、gate恢复及只停自有进程，再更新父矩阵/规范/本地Lore提交。仅S2自身验收全部满足后归档；S3–S5依赖继续。

## 保留的限制与审阅验证

- 既有全局migration检查`TestEveryConcurrentUpBuildHasCleanup`的F2版本795/796/797/798/799/801/802/803仍FAIL；S2索引检查单独证明，不改assertion掩盖全局结果。
- 2c followup首次race的`55P03 github_installation`为48 PASS/1 FAIL，后续单例及49项通过；原因未知。事后无锁与后来的autovacuum时间不能归因原失败。
- S4 E01–E07真实支付/履约、远端CI、非生产部署/回滚等仍NOT_RUN；受控local-synthetic tar和test-only callback只能证明合同。Cloud Stripe billing不能代替PAY。
- S5 r02为旧源码演练；最终candidate性能及TC24仍NOT_RUN。父矩阵原4 PARTIAL/20 NOT_RUN不是代码完成度。
- 本轮Lint/TypeCheck/产品tests均NOT_RUN（只写两份research）。执行了文档结构、12/4/18/12完整ID、冻结case引用存在性、四个最终authored场景身份及报告SHA绑定检查；没有产品通过声明。

输入报告/文档SHA、规范用例精确位置、全部清单及未运行标记在JSON伴随文件。
