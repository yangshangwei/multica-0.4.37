# 新会话交接：修复验收剩余2项E2E和4项单测失败

## 目标与授权

用户先要求继续当前分支核心业务验收，后明确问「这几个失败的，可以定位修复吗？」并要求继续。当前目标是完成这6项失败的根因修复、原用例复验、独立审查和本地提交，不是重新启动完整验收。随后用户要求写新会话引导语，当前会话停在交接。

已有任务 `.trellis/tasks/11-01-branch-e2e-acceptance` 状态in_progress。用户本轮明确授权产品修复，已写入ROOT的prd.md/design.md/implement.md和research/failure-repair-20261107/plan.md；历史R08「只验收不改产品」仅对此QA001/003/004修复被覆盖。不要重新申请建任务或重复询问修复许可。任务其他平台/Provider边界仍保留：不付费模型、不发布、不推送、不碰其他任务资源。

## 两个工作目录必须分清

- 主仓库：`/Volumes/artisan/code/2026/multica-0.4.37`，分支`codex/cross-workspace-collaboration`，交接时HEAD=`e54d09be4835e03488a2cde0a90ff451ac266f5b`。
- **本轮代码修复都在隔离工作树**：`/Volumes/artisan/code/2026/mc-acceptance-repair-20261107`，detached同一HEAD，尚未提交、尚未复制回主仓库。
- 前一轮只做验收的旧树`mc-branch-e2e-4c817dee9-20261107`和旧证据不要修改。
- 依赖复用了已安装的pnpm store；新树的workspace包链接已绑定新树，没有新依赖。
- ROOT工作区有大量其他线程的历史任务材料、Go ac43未跟踪文件和新出现的`apps/docs/content/docs/*.zh.*`改动；不是本轮产物，不要暂存/回滚。
- ROOT本轮有两份spec修改、任务范围/manifest修改及新research报告。不要漏掉与隔离树代码一起集成。

## 当前修复和证据

### QA001：小队创建路由诊断映射

根因：newSquad/newSquadManual/newSquadTemplate加入路径构建器但诊断模式表未同步。修复只补squads/new、new/manual、new/template静态模式，保留未知路径遮罩与ID隐私保护。新增三个明确路由回归。

- 原文件基线14PASS/1FAIL；新增回归先RED。
- 修复后诊断/paths针对性33PASS。
- 当前Core全量4464PASS/0FAIL。

### QA003：进展报告Skill本地化

根因：后端Skill默认描述已改成「当前项目」，en/zh目录仍为「工作空间或项目」，精确默认值匹配失败，英文界面返回中文。

- 当前en/zh描述已对齐后端当前项目范围，后端权限/指令未改。
- 已核实旧中文描述是`cd95c8e40^`实际嵌入默认，旧英文描述可由模板创建流程持久化。复用原description_v1分支，仅识别这两个精确旧文本，保留旧副本含义、用户自定义文本和改名/未验证记录。
- 原Skill文件106PASS/3FAIL；最终Skill+locale parity180PASS；两项修复合计213项针对性通过，narrow ESLint及core/views typecheck通过。
- 中间新增测试误把进展报告加入「历史仅英文」矩阵造成1FAIL，已修正为独立EN/ZH旧默认测试，原测试断言保留，失败日志保留。

### QA004：详情滚动恢复

**已确认根因**：平台写入620正确；账户详情仍启用浏览器overflow-anchor:auto。缓存的Query错误横幅112px+间距32px在成功refetch后消失，浏览器自动把620减去144成为476。trace没有JS写476，最终最大滚动范围仍允许620。第一次高频诊断2PASS是过早结束，第二次明确等待错误消失后真实Desktop RED620→476。

最小产品修复：`packages/views/collaboration/account-workbench-page.tsx`的account-detail容器加入`[overflow-anchor:none]`及解释注释。未改通用scroll恢复hook、source-workspace容器或添加循环/计时器。取舍已写spec：此显式memento容器后续布局变化不再自动调整锚点，显式评论/描述导航继续走原逻辑。

新增持久回归：`e2e/global-scope-navigation.spec.ts`在Web和真实Electron中制造读失败，保存620、scope离开/回来，暂扣真实恢复GET，释放成功响应，等待横幅消失与渲染帧，再断言620。测试URL必须用pathname谓词，因为有execution_view_version查询参数；复制真实响应头保留CORS；finally先释放并等待活跃handler结束再unroute。首次glob匹配错误的中断记录保留。

## 当前阻塞：最终浏览器验证还未全绿

最后一次canonical4例运行`scroll/runs/repaired-canonical`为**1PASS/3FAIL**，不能宣称6项已全部修复：

1. **完整Electron详情旅程已PASS**，原620恢复失败解决。
2. Web完整详情已越过原恢复失败，在更后面的A17响应式截图出现scroll1762→2082，原几何不变断言失败。诊断捕获`closeForNavigation`的显式评论锚点写入晚于locale reload、进入截图阶段。修复已写入`e2e/fixtures/collaboration-detail-journey.ts`的switchJourneyLocale：从真实WebURL/原生accountTab.url解析保留的commentId，刷新后等待该评论实际获得焦点，再截图。**此最终fixture改动尚未完成canonical复验**，不能只靠私有capture诊断PASS宣称关闭。
3. Web global-scope晚段中文按钮断言仍是旧「协作」，当前实际catalog/DOM为**「协作空间」**。fixture已改实际文案，待最终复验（不是「跨账号协作」）。
4. Desktop新增global-scope回归在释放恢复GET后错误横幅5秒内未消失。三份真实GET返回200且schema safeParse成功；原生scope切换时前两个请求被net::ERR_ABORTED，最后fulfill200后仍有`Could not load collaboration. Try again.Refresh`。准确原因尚未关闭；不得把这点误说成同一个中文标签问题，也不要放宽5秒/620断言。

优先下一步：读`scope-response-diagnosis`和`scope-recovery-diagnosis`的原始日志，确认held请求是否绑定到了已取消的scope/Query。可评估简化永久回归为：scope resume确认cached错误和620后，通过**当前页面真实Refresh按钮**发起一个明确的新恢复GET，暂扣→释放该真实请求→横幅消失→620不变。此方案尚未实施/验证；需保留原自动恢复探针失败，不要用静默额外retry掩盖产品问题。核心回归要证明错误内容消失不会产生浏览器锚定偏移。

## 已完成的扩大验证

私有检查根目录：`ROOT/.omx/private/acceptance-repair-20261107/checks/`。

- core-full：4464PASS。
- remaining-unit-packages：docs62、web290、desktop1272全部PASS，共1624。
- views-full：7241PASS/3FAIL，失败都在**未修改**的`modals/create-issue.test.tsx`（两个STACK_TRACE_ERROR后一个pointer-events:none）。原4个请求修复的单测在此次均PASS。
- create-issue-recheck：不改源、不改超时，整文件61PASS。保留首轮3失败，精确原因未确立，不能说首次全量全绿。综合各文件当前13,332个唯一测试均有通过证据；不是单轮全绿。
- static-final：15个非Mobile lint/typecheck任务全PASS，30warnings、0errors。
- 未重新跑Go：本轮产品后端没有修改。前一轮396顶层+618子测试通过只属于其原版本/范围。

汇总：`research/failure-repair-20261107/unit-full-results.json`，原始命令/退出码/JSON均在上述checks目录。不要重复这些已完成检查，除非新增源修改影响其结论。

## 当前未提交代码清单（隔离树）

1. packages/core/diagnostics/diagnostic-context.ts
2. packages/core/diagnostics/diagnostic-context.test.ts
3. packages/views/locales/en/skills.json
4. packages/views/locales/zh-Hans/skills.json
5. packages/views/skills/lib/skill-presentation.ts
6. packages/views/skills/lib/skill-presentation.test.ts
7. packages/views/collaboration/account-workbench-page.tsx
8. e2e/global-scope-navigation.spec.ts
9. e2e/fixtures/collaboration-detail-journey.ts

`apps/web/next-env.d.ts`是Next生产构建生成的types导入变化，不作为业务修复合入；只能按repo已约定的单行规范化核对，不排除整个文件。

ROOT两个本轮spec：
- .trellis/spec/views/frontend/builtin-skill-localization.md
- .trellis/spec/core/frontend/global-scope-navigation.md

## 隔离服务仍在运行，先核验所有权再使用/停止

- 新环境private：`ROOT/.omx/private/acceptance-repair-20261107/scroll/`
- 配置/所有权：allocation.json、env.json、source.env、registry/envs/scroll-repair-20261107/{api,web}.running.json、renderer-process.json。
- 状态目录在Git外：`/Volumes/artisan/acceptance-state/acceptance-repair-20261107-scroll`。
- API19793；Web14713；renderer5981。
- 唯一新库：multica_scroll_repair_20261107。不要打印URL/认证信息，不用默认multica库。
- 交接查询的监听PID写在handoff-state.json，可能变化，使用前重新核对health/PID/源码/配置。
- 没有run.json仍为running的测试记录；API/Web/renderer未清理，目的是让新会话继续验证。不得误写cleanup完成。
- 当前Web+renderer构建包含7个产品/单元文件修复，之后只有E2Efixture改动。已有source-hashes、provenance及固定构建记录；修改测试后必须区分测试指纹与产品构建字节等价，不能禁用来源校验。

可参考现有runner（不先直接跑全量）：
```
python3 .omx/private/acceptance-repair-20261107/scroll/run.py <全新run标签> pnpm exec playwright test e2e/collaboration-detail-journey.spec.ts e2e/global-scope-navigation.spec.ts --workers=1 --retries=0 --reporter=line,json --output=<本任务私有新目录>
```
runner从其env.json读取配置并在隔离工作树执行。先核对匹配当前源码的detail-provenance与产品等价审计；不要直接重复运行provenance.py去覆盖旧证据，它严格要求source_id完全匹配，测试文件变化也会影响此值。需要保留before/after来源或为新产品变更重建。

## 证据索引（ROOT下）

`research/failure-repair-20261107/`：
- plan.md、progress.md、unit-repair.md/.json
- unit-full-results.json
- scroll-root-cause.md/.json
- scope-test-diagnosis.md/.json
- review-unit.md/.json、review-source.md/.json（静态范围已复核，无源码阻断；最终动态批准仍待补齐）

私有scroll/runs关键记录：
- diagnosis-baseline：2PASS但过早，不能当修复证明。
- diagnosis-after-recovery：Desktop真实RED620→476。
- recovery-regression-red：glob错误后中断130。
- recovery-regression-red-2：新永久回归2FAIL，分别620→660/580；不都在新增横幅移除断言，不混同144px因果RED。
- repaired-canonical：4例1PASS3FAIL，如上。
- capture-diagnosis：定位显式锚点写入，私有诊断PASS。
- scope-recovery-diagnosis、scope-response-diagnosis：native取消/恢复请求日志，仍FAIL。

## 下一步与完成条件

1. 恢复现有Trellis任务，读取ROOT更新后的授权/计划，不依据隔离树旧prd的R08拒绝修复。
2. 优先关闭Desktop新回归的held请求归属/恢复问题；验证fixture里没有取消请求误判成功。无必要不扩大产品修复。
3. 在当前固定产品上跑完整Web/Desktop详情2例和global-scope2例，保留620、截图前后geometry、API/权限/文件及语言断言，0自动重试。当前Webfixture焦点等待和文案修正需要真实最终绿证据。
4. 最终独立审查9个文件+2spec+raw/来源（如果有新修改扩大检查相应范围）；之前review是条件通过，不是最终验收批准。
5. 用baseline前置哈希/差异守卫把隔离树9个文件集成ROOT，排除generatednext-env。不要覆盖其他线程修改。任务报告追加修复结果，旧失败记录保留。
6. 停止且只停止本任务拥有的API/Web/renderer，验证3端口关闭、专库连接0、无自有进程残留。
7. 按Lore协议提交本轮确切文件，不amend、不push；记录journal。原整体验收的安装/其他374库存等门仍未本轮覆盖，不归档成完整产品验收通过。

## 代理与运行限制

上一会话用Codex原生trellis-implement（unit、scroll）和trellis-check；scroll/review后来因`Concurrency limit exceeded for user`断开。该错误是工具并发限制，不是业务测试失败。新会话不必依赖旧agentID，可重新派发有界任务；若同样限额，主会话接管普通可逆工具工作，不让用户处理环境或重复授权。

不要从头重跑旧68/441/Go全部验收来替代完成剩余修复。不要新增依赖、减弱断言、增加自动重试或拿历史PASS覆盖本轮FAIL。
