# I1 本地提交交接

> **代码提交已完成（2026-10-07）：** 五个本地代码提交 `823a61774`…`e516b15c2` 已落在 `codex/projects-p1`。本文及配套验收记录随其后的文档提交保存；文档提交身份以 `git log -- .trellis/tasks/10-05-iterations-i1-verification/verification.md` 为准，不预填尚未生成的自身 SHA。不要再次执行代码提交；提交清单与源码对应见[最终验收](verification.md#本地提交与已验证源码的对应关系)。下文保留为当时的交接记录。

> 用途：在新会话中完成 **①核对提交范围 ②完成本地提交**。这是一份执行交接，不是重新规划或重复开发 I1。
> 日期：2026-10-07。本文是这两个步骤的最新授权和上下文入口；其后仍保留“不推送、不合并、不部署”的限制。

## 1. 用户目标与授权

用户在完成本地验收后，明确要求下一会话核对提交范围并完成本地提交。因此下一会话应在辨明文件归属、核对证据和暂存内容后执行本地 `git add` / `git commit`，而不是只给另一个提交建议。

此前验收 handoff 中“本轮不提交”描述的是已经结束的验收会话；它不撤销本次新指定的本地提交目标。本次写 handoff 的会话没有暂存或提交。若遇到无法辨明归属的文件，保留其工作区内容并单列，不为让整个工作区变干净而顺手提交或回滚。

边界：

- 沿用现有任务树、分支和已验收实现；发布开关 `iterations_i1` 默认仍关闭。
- 不推送、不创建远端 PR、不合并、不部署、不运行远端发布动作、不启用试点。
- 不 amend/rebase/reset/clean，不改写已存在的提交。不要自动 stash 全工作区，也不要把排除文件 restore 掉。
- 本地提交完成后仍不能关闭正式 VG；不归档父任务或 verification，不把本地提交等同远端 CI 已通过。
- 主会话独占 Git index 与提交操作。可以使用原生子代理独立只读审查范围，但禁止多个代理同时暂存／提交或覆盖同一文件。

## 2. 工作区与当前事实

| 项目 | 状态 |
| --- | --- |
| 工作目录 | `/Volumes/artisan/code/2026/multica-0.4.37` |
| 分支 | `codex/projects-p1` |
| 本次核对 HEAD | `115b4cd28277941a8d9ad8fda5acecf99b999152` |
| 活动任务 | `.trellis/tasks/10-05-iterations-i1-verification` |
| 暂存区 | 写本文前为空 |
| 写本文前 dirty | 120 个 tracked 修改、176 个逐文件展开的 untracked；`git diff --stat`不包含这些untracked |
| 源码复核 | 6,696 个最终已验收运行／测试文件SHA全部一致，0漂移 |
| 阶段 | FG/LG/HG/CG/UG/FCG通过；五个实施子任务completed |
| VG | `local_passed_remote_ci_pending`；父任务与verification为in_progress |

[本次交接快照](commit-handoff-snapshot.json)列出逐文件状态、SHA及**待核对的候选归类**。它不是批准的暂存白名单，不代替新会话读取 diff。本文、快照及新入口元数据属于本次交接文档增量。

原开发会话 `01a1119c-6652-7230-87ec-e5e6945ee021` 于11:12中断。验收收尾时API/Web已停止；迁移专属DB已删除，原I1测试环境保留。新会话仍应重新核对当前进程、近期会话和文件变化，不能拿当时的静止状态推断现在没有写入者。不要按目录名推断分支，也不要去保留的detached checkout继续。

## 3. 必读入口

按以下顺序读取，避免从历史handoff重新启动CG：

1. 根 `AGENTS.md`、`CLAUDE.md`、`.trellis/workflow.md`，尤其Lore、包边界、测试guard与提交要求。
2. 本文；[最终验收](verification.md)、[29项矩阵](acceptance-matrix.md)、[源码来源](source-provenance.json)、[最终检查记录](final-bookkeeping-evidence.json)。
3. [后端审计](backend-final-audit.md)、[UI审计](ui-final-audit.md)、[移动兼容](mobile-final-compatibility.md)、[迁移恢复](migration-recovery-evidence.md)、[跳过说明](skipped-tests.md)。
4. 父任务[设计](../10-05-iterations-i1/design.md)、[API合同](../10-05-iterations-i1/api-contract.md)、[执行门槛](../10-05-iterations-i1/implement.md)、[测试规范](../10-05-iterations-i1/test-spec.md)；有范围疑问时查原始 `docs/plans/2026-10-04-work-management-prds/iterations-prd.md` §§6–8及对应ITR条目。
5. 对有疑问的遗留fixture，查 [FCG修复来源](fcg-backend-fixture-regressions.md)、[history字段记录](history-fields-verification.md)和父/closure/client证据；必要时读取原会话记录，不把文件名不含iteration当作无关证据。

Trellis上下文已在本会话加载过；新会话按trellis-start恢复，必要时 `task.py start` **已有verification任务**。不新建任务，不回到planning。

## 4. 第一步：核对提交范围

先执行只读盘点：

```sh
pwd
git status --short --untracked-files=all
git branch --show-current
git rev-parse HEAD
git diff --cached --stat
git diff --name-status HEAD
git diff --stat HEAD
git ls-files --others --exclude-standard
git log -5 --format='%h %s%n%b'
```

核对原会话/其他agent是否正在改文件；对比交接快照及最终测试源清单。新增差异先归属和解释。任何已有staged内容必须识别所有者，不自动清掉或混入自己的提交。

### 候选提交组（需要审查后定稿）

| 候选组 | 范围与依据 | 需要保持一起的内容 |
| --- | --- | --- |
| A：服务端闭环与历史一致性 | CG原子关闭/交接/禁用、outbox、批量归属、历史priority/labels、标签写入fence、router/main接线 | 对应service/handler/domain、SQL源、全部sqlc产物及受接口影响的测试double；必要的built-in skill与server spec |
| B：共享客户端与平台接线 | `packages/core` API/schema/queries/recovery/types/realtime/paths；`packages/views`迭代及既有入口；Web/Desktop路由、Mobile兼容、EN/ZH词条 | export入口、types/schema/实现、调用方、相关测试和core/views规范一起核对；package.json当前只是新增exports，无新依赖 |
| C：跨模块验收修复与E2E | 真实Web/Electron I1场景、共享Desktop WS代理fixture、P1页面定位回归、授权fixture隔离、迁移与A–J补证 | 测试应尽量跟随其生产实现；仅真正独立的旧fixture修正拆成单独test提交，避免“测试大杂烩” |
| D：验收、规范与交接记录 | `.trellis/tasks/10-05-iterations-i1*`当前证据/状态、截图与SHA、相关spec、本文和快照 | 放在产品提交之后，补实际本地代码提交清单与源码对应关系，保留VG远端CI待办 |

这些是分组建议，不是固定四个提交。审查时允许合并高度耦合的组；不要为了凑数量产生中间无法编译或漏配套文件的提交。已经在HEAD里的FG/LG/HG提交、550–566生产迁移不重新打包成新增工作。本轮新增的是迁移恢复**测试**。

### 容易误分的已知文件

- `server/pkg/db/generated/db.go`的DBTX新增`SendBatch`；不少daemon/task/grant相关`*_test.go`只是给现有fake补该方法。虽然文件名不含iteration，这些可能是编译闭环必需，逐个追溯后与对应sqlc接口改动同批提交。
- `server/internal/handler/issue_create_position_test.go`、`quick_create_parent_test.go`用专属dbfx runtime/agent替代共享fixture，避免授权/执行顺序干扰；`workspace_delete_manifest_test.go`补七张I1表。相关证据在FCG修复记录，不是放宽生产权限。
- `e2e/project-squad-workspace.spec.ts`显式进入`?section=issues`；`e2e/workspace-defaults.spec.ts`先切Issues。它们是P1整合后的验收定位修正，可以独立test提交，但须记录与本次全绿基线的关系。
- `e2e/fixtures/project-p1-desktop.ts`的WS代理让真实Electron恢复/实时更新实际生效；不是可以随意剔除的无关测试设施。
- `server/internal/handler/label.go`虽然不带iteration名字，保护冻结历史标签字段及并发快照，属于I1验收修复。
- `server/internal/service/builtin_skills/multica-working-on-issues/`是产品内置使用说明，不等同下方agent工具生成物。

### 默认保留在工作区、不要顺手暂存

| 路径 | 原因／动作 |
| --- | --- |
| `.agents/skills/`（本次盘点46个untracked文件） | Trellis工具技能生成物；未作为I1产品提交授权的组成部分，另行核对，默认保留 |
| `.codex/`（8个untracked文件） | agent配置/生成物，默认保留 |
| `.trellis/.template-hashes.json` | 工具模板同步记录，默认与I1分离 |
| `apps/web/next-env.d.ts` | 当前唯一差异是`.next/dev/types/routes.d.ts`→`.next/types/routes.d.ts`，由生产构建自动产生；默认不为I1提交此切换，保留文件内容，不为排除而回滚 |
| `.omx/logs/`、`test-results/`、临时env/profile、构建输出 | 本地原始证据／运行产物；不批量force-add。Git中的验收JSON/Markdown和选定截图记录引用及摘要 |

不要使用`git add -A`或通配整个工作区。大文件清单可使用明确的NUL分隔pathspec文件；有空格、括号、方括号的路径必须正确引用。跨组hunk只在可以保持依赖完整时拆分。

## 5. 验证复用与变更判断

### 已有充分证据

- 完整基线：`.omx/logs/i1-verification/full-check-accepted.log`，10,215 TS tests、15/15静态任务、全Go race/vet、生产构建、249 E2E通过／49跳过。
- 最后13个源／测试增量：views100、mobile196+iOS脚本、迁移2顶层/4子项、History15顶层/21pass事件，类型/lint/vet/gofmt通过。
- 最终生产Web/Electron：`.omx/logs/i1-verification/final-e2e-corrected.log`，四项全部通过，0skip/0retry，24.2s。
- 最终视觉94/pass、12张截图及SHA，位于`browser-evidence/final-screenshot-manifest.json`和`visual-verdict-final.json`。
- 最终源清单：`.omx/logs/i1-verification/final-check-inputs.json`，6,696个文件；87个sqlc产物与已有稳定生成快照一致。`source-provenance.json`记录基线与增量各自覆盖范围。
- P95和普通写入样本仍满足冻结阈值；未改变受测后端路径时不重跑性能。旧`/tmp`部分原日志已过期，不声称重新hash了不存在的文件。

仅暂存、分组、提交和更新文档不会改变运行源码。若新会话核验源清单仍一致，直接复用证据；提交SHA变化本身不是重跑全部套件的理由。文件如果被修改、排除的hunk实际属于功能依赖，或hook改写源码，则补对应验证，不能仍冒称旧证据适用。

需要数据库验证时必须独占测试库；Go/TS/E2E测试使用 `scripts/go-test-with-agent-cli-guard.sh`。若需要浏览器，使用production build，禁止在同checkout并发构建；源指纹包含tracked文档，构建期间连handoff/spec也不能改。

保留测试环境：`check-20261006151843-35615`（API18574/Web13494）。`dev-env.sh up`会按checkout重用注册环境，必须进入该实际环境并使用它自己的ENV_FILE，先检查`.next/routes-manifest.json`代理目标匹配API，避免继承另一个check环境配置。测试环境启用FF仅为fixture，仓库默认开关仍关闭。

### 提交前后最小检查

每批明确列出文件和理由，再执行本地暂存；提交前核对：

```sh
git diff --cached --name-status
git diff --cached --stat
git diff --cached --check
```

检查暂存中没有无关工具配置、生成路径切换或错误遗漏；测试double/SQL/导出/调用方不能拆断。遵守仓库hooks，失败先辨明原因，不用`--no-verify`隐藏失败；若hook改动文件，重新检查实际暂存内容与源证据。

提交后核对`git show --stat --oneline HEAD`及剩余dirty。剩余的排除项是预期结果，不以整仓clean作为成功条件。HEAD若在接手后改变，先解释新提交，不回退别人工作。

## 6. 本地提交格式与最终记录

每个commit使用仓库Conventional前缀及Lore决策记录。首行表达为什么改；正文写约束和做法，实际使用有价值的trailers，例如：

```text
feat(iterations): preserve one confirmed result when manual periods close

Keep frozen history, task destinations and notification receipts in the
same transaction, and expose recovery through the original request identity.

Constraint: iterations_i1 remains disabled by default
Rejected: Rebuild closed history from live tasks | later edits would alter the accepted facts
Confidence: high
Scope-risk: moderate
Directive: Keep existing transaction owners and source-before-target membership ordering
Tested: Refer to the exact retained baseline and final affected-path evidence
Not-tested: Remote CI, historical client binaries, production rollout
```

这是格式示例；必须按**实际暂存内容**重写标题、正文和Tested，不能把仅文档／测试提交称为全部功能实现，也不能把复用的上次结果写成本会话重跑。

产品/测试提交后，在现有verification任务中记录实际本地commit清单（SHA、意图、文件组、复用或新增验证、剩余排除项），并更新父handoff/task metadata中的接续位置。保留历史`base_head`和原始验收SHA，不把它们统一改成新HEAD；新增“这些提交对应已验证源内容”的映射。不要让文档试图记录自身尚未生成的commit SHA；最终答复列出最后文档提交即可。

最终完成标准：

- 已审查的I1代码、配套测试和应入库的验收／规范文档形成可审查的本地提交序列。
- 没有未经说明漏掉必要依赖；未识别或明确排除的文件保持原内容且未提交。
- 运行源码与验收证据一致；有实际源码变更时补对应验证。
- 报告每个commit SHA及范围、剩余dirty及原因、下一步正式CI待办。
- 发布开关仍关闭；无push/PR/merge/deploy；父任务和verification仍in_progress，VG仍`local_passed_remote_ci_pending`。

## 7. 可直接粘贴到新会话的启动语

```text
继续「迭代 I1：手动周期闭环」的本地提交收尾，只完成：
1. 核对提交范围
2. 完成本地提交

工作目录：/Volumes/artisan/code/2026/multica-0.4.37
分支：codex/projects-p1
交接核对HEAD：115b4cd28277941a8d9ad8fda5acecf99b999152

先阅读 AGENTS.md、CLAUDE.md、.trellis/workflow.md，以及：
.trellis/tasks/10-05-iterations-i1-verification/commit-handoff.md
按该文档继续已有verification任务，不新建任务、不重新规划或开发CG/客户端。

先核对当前git状态、原会话是否仍写文件、暂存区及已验证源码指纹，保留所有已有成果。
我授权核对清楚后完成I1相关本地提交；无法辨明或无关文件保留未暂存并报告。
按Lore协议组织原子提交，记录实际SHA与验证证据对应关系。
充分证据直接复用，只有实际源码漂移/依赖遗漏/hook改写时补相关验证。

保持发布开关默认关闭。不推送、不创建远端PR、不合并、不部署。
UG/FCG已通过；VG本地项通过但正式远端CI未跑，父任务与verification保持in_progress。
完成后列出本地提交、剩余未提交文件及原因；不要自动归档任务。
```
