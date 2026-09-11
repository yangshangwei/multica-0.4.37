# 周期性 Agent 自动化模板

> 本文件只记录需求、约束与验收标准。技术设计见 `design.md`,执行计划见 `implement.md`。

## 背景

仓库中已存在成熟的 **Autopilot** 子系统:`autopilot` / `autopilot_trigger` / `autopilot_run` 三张表,配套 DB 租约调度器(`server/internal/scheduler/`)、cron 解析与预览、前端结构化 cron 编辑器。周期性派活给 agent 的能力**已经完整**。

缺的是入口。目前前端存在 6 个硬编码模板常量(`packages/views/autopilots/components/autopilots-page.tsx:145-207`),存在四个硬伤:

1. **只在空态可见** —— `showEmpty`(`totalCount === 0`)才渲染;用户一旦建了任意 autopilot,模板永久消失。
2. **prompt 是 tsx 里的英文字面量** —— 无 i18n、无版本、无 provenance。
3. **服务端无模板概念** —— 全仓库 `AutopilotTemplate` 在 `.go` / `.sql` 零命中。前端须自行发两个请求(建 autopilot + 建 trigger),并自行兜"建成一半失败"的残局(`autopilot-dialog.tsx:345-365`)。
4. **无高频型预设** —— 6 个模板 schedule 全是日/周粒度的 `{kind:"at"}`。

本任务不造引擎,只补模板层。

## Goal

为 Autopilot 提供一个与仓库既有模板范式(`AgentRoleTemplate`、`SquadTemplate`)对齐的**服务端模板注册表**,并把模板入口从空态中解放出来。用户从预置卡片选择一项、指定执行者,即可创建一条常驻的周期性自动化。

## 已确认的产品决策

### 1. 走服务端注册表(方案 A)

模板以 Go embed 形式住在服务端,提供 `GET /api/autopilots/templates` 与 `POST /api/autopilots/from-template` 两个端点。

已否决:纯前端扩充硬编码常量(无 provenance、无版本、部分成功问题仍在、多端各自抄)、模板入库支持用户自建(YAGNI)。

### 2. 模板是预填初值,不是实体

模板不入库。选中后生成一条**普通的、可编辑的** autopilot 实例,周期与 prompt 均可修改。数据模型中只存在「实例」一种实体,模板仅提供创建表单的默认值与 provenance 标记。

### 3. 产出策略按模板性质分流

**关键架构约束**:`dispatchCreateIssue`(`server/internal/service/autopilot.go:646-837`)在 agent 开始执行**之前**就已建好 issue —— agent 是 issue 的下游消费者,其输出不参与"建不建 issue"的决策。一次 run 恰好产出 1 个 issue,无条件、无循环。

因此"仅在有实质发现时创建 issue"在 `create_issue` 模式下**架构上不可实现**。改为按模板性质选择执行模式:

| 性质 | 模式 | 产出行为 |
| --- | --- | --- |
| 摘要型(每期都应有产出) | `create_issue` | 每次运行产出一个 issue,如日报周报 |
| 巡检型(多数时候无事发生) | `run_only` | 不预建 issue;agent 运行时按 prompt 指引,仅在有发现时自行调用 issue API 创建 |

已否决:全部使用 `run_only`(摘要型失去每期必有产出的确定性,且失去 issue 终态回填 run 状态的机制)、全部使用 `create_issue`(高频巡检一天堆积 24 个 issue)、改造后端新增第三种执行模式(需改动 autopilot 核心 dispatch,超出模板层范围)。

### 4. 模板集为十项(原六项迁移保留)

新增模板集:

| Key | 分类 | 标题 | cron | 执行模式 |
| --- | --- | --- | --- | --- |
| `workday-repo-audit` | REPO 健康 | 工作日仓库审计 | `0 9 * * 1-5` | `run_only` |
| `release-readiness` | 发布准备 | 发布准备情况 | `0 17 * * 1` | `create_issue` |
| `daily-change-review` | 定期评审 | 每日变更回顾 | `0 18 * * *` | `create_issue` |
| `hourly-queue-check` | 维护 | 每小时排队检查 | `0 * * * *` | `run_only` |

原有六个模板中**五个迁移保留**(决策于实现期由用户修正,此前一度决定删除):

| Key | 原 id | 分类 | cron | 执行模式 |
| --- | --- | --- | --- | --- |
| `stale-pr-reminder` | `pr_review` | 定期评审 | `0 10 * * 1-5` | `run_only` |
| `bug-triage` | `bug_triage` | 分诊 | `0 9 * * 1-5` | `create_issue` |
| `weekly-progress-report` | `weekly_progress` | 发布准备 | `0 17 * * 1` | `create_issue` |
| `dependency-audit` | `dependency_audit` | REPO 健康 | `0 8 * * 1` | `run_only` |
| `documentation-check` | `documentation_check` | 定期评审 | `0 14 * * 1` | `run_only` |

第六个 `daily_news`(每日新闻摘要)**不迁移**:它靠联网搜索公开资讯才能产出,内网/离线部署不具备这个条件,留在册子里等于给这类部署一个必然空跑的模板。册中其余九个只读取本工作区的仓库、issue 与项目状态,不需要出网。`daily_news` 与其余五个旧 id 一同保持为未知 key。

迁移保留的是**内容**,不是旧实现:prompt 原文与四语言文案从 git 取回后迁入服务端注册表,前端的 `TEMPLATES` 硬编码常量仍然删除。

**一处必要的 prompt 改写**:三个改判为 `run_only` 的旧模板(`stale-pr-reminder` / `dependency-audit` / `documentation-check`),其原 prompt 结尾为「在本 issue 上发表评论」——那是 `create_issue` 模式的写法,因为系统会预建 issue。改为 `run_only` 后不再预建 issue,该句将指向不存在的对象,故结尾改写为「仅在有实质发现时创建 issue 并写入结论」,并追加与其他巡检型模板一致的去重指引。三个 `create_issue` 模板的结尾保持原文不变。

`bug-triage` 是十个模板中唯一执行写操作的(设置 issue 的 priority),其 prompt 声明该要求;执行 agent autonomy 不足时应在评论中给出建议优先级而非静默失败。

### 5. 创建流程是两步,不是一键

autopilot 的 `assignee_id` 为必填,模板无法预知用户拥有哪个 agent。因此流程为:选择模板 → 配置(必须指定 agent,其余字段可改)→ 创建。与既有 `template-create-agent-page.tsx` 的两步式一致。

## Requirements

### 必须

- 提供模板选择界面,每个模板呈现分类、标题、描述三层信息。
- 模板集覆盖每小时 / 每工作日 / 每日 / 每周四种节奏。
- 模板标题与描述支持 en / zh / ja / ko 四语言,由服务端下发。
- 从模板创建的实例,其周期、prompt、执行模式均可在创建时与创建后修改。
- `POST /api/autopilots/from-template` 必须在**单个事务**内同时创建 autopilot 与 trigger,消除部分成功路径。
- 创建出的 autopilot 记录 `template_key` 与 `template_version`。
- 模板入口不得依赖列表空态;已有 autopilot 的工作区同样可访问模板。
- 巡检型模板(`run_only`)的 prompt 须指引 agent 在创建 issue 前检查本 autopilot 已创建的未关闭 issue,同一问题改为追加评论。
- 删除 `autopilots-page.tsx` 中的 `TEMPLATES` 常量及其 locale 条目。dialog 的部分成功处理**保留**——空白创建流仍走两次调用,那段仍是必要的正确性代码。
- 卡片选中态在 hover 时须保持可辨识(遵循 `CLAUDE.md` 的 UI 规则)。

### 不做(本期)

- 系统级的"同一问题跨周期去重"(指纹 / 相似度 / 更新既有 issue)。当前仅有 60 秒防抖(`autopilotRecentDuplicateWindow`),本期不扩展,巡检型依赖 prompt 软约束。
- 自动化执行代码写操作(修改文件、提交、合并)。
- 自动化运行历史的独立可观测面板。
- 用户自建模板 / 模板入库。
- 修改调度器、cron 解析、派活链路的任何代码。

## Acceptance Criteria

- [x] `GET /api/autopilots/templates` 返回九个模板(决策 4 的四个新增 + 五个迁移保留),含分类、四语言标题与描述、cron、执行模式、prompt。
- [x] 模板列表在工作区已存在 autopilot 时同样可访问。
- [x] 选择模板后进入配置步骤,标题、prompt、周期、执行模式预填自模板并以只读形式预览;创建后通过详情页修改(设计 D1,创建时不可编辑)。
- [x] 未指定 agent 时无法提交创建。
- [x] `POST /api/autopilots/from-template` 成功时,autopilot 与 trigger 同时存在;trigger 创建失败时 autopilot 一并回滚,不留孤儿记录。
- [x] 创建出的 autopilot 行携带正确的 `template_key` 与 `template_version`。
- [x] 各模板创建出的 trigger,其 cron 表达式与模板定义一致,且能通过服务端 cron 校验。
- [x] `hourly-queue-check` 与 `workday-repo-audit` 创建出的 autopilot,`execution_mode` 为 `run_only`,运行时不预建 issue。
- [x] `release-readiness` 与 `daily-change-review` 创建出的 autopilot,`execution_mode` 为 `create_issue`。
- [x] `autopilots-page.tsx` 中不再存在 `TEMPLATES` 常量。
- [x] 调度器、`service/cron.go`、autopilot dispatch 链路无代码改动。
- [x] 后端 handler 测试覆盖 list、from-template 成功、from-template 事务回滚。
- [x] 前端存在模板 schema 的 malformed-response 测试。
- [x] ~~卡片选中态与 hover 态同时命中时,选中态仍可辨识。~~(作废:实现方案为点击卡片即进入配置步,卡片无选中态,该项无可测对象。)
