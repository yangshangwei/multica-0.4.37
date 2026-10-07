# P1 浏览器细化用例矩阵

日期：2026-10-06。状态：**61/61 实跑通过，零重试、零跳过**。本文件记录当前 7 份 spec 的 61 条独立 Playwright 测试及其必要断言，本轮以独立61条实跑结果为准，不用此前8条记录代验。完整命令、首轮失败和修正见 [本轮验证记录](e2e-expansion-verification.md)。

2026-10-07 补充：生命周期组新增参数化 P1-L16 两条（成员离开、owner移除），用请求计数验证 `42a98ff05` 修复的撤权请求循环；P1-L14 在关闭 trace 时仍会通过，单看通过/失败无法证明修复。上表61条的实跑结论不变；7份spec现共63条测试，即原61条加此两条。修复前构建两条均因端点循环失败（20秒约25,000次请求），合并后构建均为9次请求且首个5秒后为0，验证见该测试提交说明。

枚举依据：`.omx/p1-expanded-list.log`，`Total: 61 tests in 7 files`。对照基线为 `421ca49eb` 的 `e2e/projects-p1.spec.ts`（7 条，含一个收敛测试）及 `e2e/projects-p1-desktop.spec.ts`（1 条）。需求解释沿用 [父测试规格](../10-05-projects-p1/test-spec.md) 和 [API 合同](../10-05-projects-p1/api-contract.md)。下表 `AC-xx` 指 `PRJ-AC-xx`，`FR-xx` 指父测试规格的 `P1-FR-xx`；映射表示该用例验证相应需求的一个明确片段，不代表单条测试覆盖完整 AC 或 FR。

## 数量与来源

“拆分原覆盖”表示原 8 条大流程已经覆盖主要行为，本次提取成可独立运行、独立失败的测试；其中 C01 是原样保留。“新增/强化”表示增加了关键前置条件、失败恢复、竞争条件、平台交互或持久化断言，即使它仍来自既有 P1 需求，也归入这一类。少量常规断言补充不改变主要行为已覆盖的分类，例如 G02 的版本递增和 H10 的已接受字段核对。

| 分组 | 独立测试数 | 拆分原覆盖 | 新增/强化 |
| --- | ---: | ---: | ---: |
| G：目标与描述 | 5 | 3 | 2 |
| H：统计与正式范围 | 11 | 9 | 2 |
| R：风险下钻与分页 | 8 | 3 | 5 |
| P：进展、验收与证据 | 15 | 5 | 10 |
| L：生命周期、权限与恢复 | 15 | 3 | 12 |
| D：真实 Electron | 6 | 1 | 5 |
| C：双页面收敛 | 1 | 1 | 0 |
| **合计** | **61** | **25** | **36** |

从 8 条增加到 61 条是多出 53 个独立测试入口，**不是增加 53 项业务需求**。C01 内有截止日期、负责人、准入各 10 个测量样本，共 30 个样本，仍只计 1 条测试。五种任务视图及四种风险信号虽然用参数化循环声明，每个参数都生成独立测试、独立工作空间和独立结果。

## 共用前置与读取边界

- 每条测试以 `p1Session` 或测试 fixture 建立自己的账户、工作空间与数据；`finally` 或 fixture teardown 清理该空间。Electron 另有独立 profile、进程和本地代理服务，结束后关闭清理。
- 下表的“API 准备”“SQL 准备”都只是构造前置状态，不宣称经界面创建、修改或删除。用户操作列明确列出实际点击、输入、键盘或导航行为；随后 API/SQL 查询用于核对服务端结果。
- 运行中或已完成的 execution 是数据库中合成的执行记录，不启动真实智能体进程。存在一个合成执行时，“执行数仍为 1”表示没有额外执行，不能写成执行总数为零。
- G/H/R 使用 [基础 fixture](../../../e2e/fixtures/project-p1.ts) 与 [统计/风险 fixture](../../../e2e/fixtures/project-p1-health.ts)。P 使用 [进展 fixture](../../../e2e/fixtures/project-p1-progress.ts)，D 使用 [Electron fixture](../../../e2e/fixtures/project-p1-desktop.ts)。
- 下表全部预期已在本轮61条实跑中通过，未另设一列重复写61次“通过”。故障注入使用浏览器 route 或 SQL 屏障时，已在前置/操作中注明；它们不代替真实旧服务器兼容或服务端并发测试。

## G：目标模板与描述并发（5 条）

默认项目经 API 创建，描述已有 `Goal` 标题和原客户目标。并发写来自第二位真实成员的 API；浏览器只暂扣本人的 PUT，最终仍请求真实服务端 CAS。

| ID／测试文件 | 来源 | 中文场景 | 前置与实际操作 | 明确预期 | AC／FR |
| --- | --- | --- | --- | --- | --- |
| [P1-G01](../../../e2e/projects-p1-goals.spec.ts#L8) | 新增/强化 | 预览并取消模板不写正文 | 在模板中选验收章节并预览，再关闭模板、重载；监听项目 PUT | 预览区有验收章节，编辑器没有新增章节；PUT 为 0；数据库正文和描述版本不变；重载仍保留原文 | AC-02、AC-03 |
| [P1-G02](../../../e2e/projects-p1-goals.spec.ts#L31) | 拆分原覆盖 | 只追加选中的缺失章节 | 同时选择已存在 Goal 与缺失 Acceptance criteria；预览后聚焦追加按钮并按 Enter；重载 | 预览不重复 Goal；保存后原文保留、Goal 仅一次、未选 Out of scope 不出现、描述版本加 1；新增验收章节持久化 | AC-02、AC-03 |
| [P1-G03](../../../e2e/projects-p1-goals.spec.ts#L57) | 拆分原覆盖 | 已有章节不可重复插入 | API 准备已有 Goal、Acceptance criteria 的正文；UI 选择这两项并预览 | 提示所选标题均已存在，追加按钮禁用；正文和描述版本保持原值 | AC-03 |
| [P1-G04](../../../e2e/projects-p1-goals.spec.ts#L73) | 拆分原覆盖 | 真实 CAS 冲突保留本地稿供复核保存 | 浏览器本地编辑的 PUT 等待；另一成员 API 保存新正文；放行旧 PUT；点击 Save reviewed draft 后重载 | 服务端新正文先保留，本地稿不丢；显示比较提示；两个冲突操作按钮完全在视口内；显式保存后本地复核稿入库并跨重载保留 | AC-25、FR-04 |
| [P1-G05](../../../e2e/projects-p1-goals.spec.ts#L108) | 新增/强化 | 读回失败时采用服务端版本，后续编辑使用正确版本 | 制造真实 CAS 冲突，同时将浏览器项目 GET 注入 503；选择 Use server version；继续编辑，再解除 GET 故障并重载 | GET 确实失败；选定服务端文本立即显示且被放弃稿不回灌；后续 PUT 携带选定版本的 `expected_description_revision`，新编辑成功持久化 | AC-25、FR-04 |

## H：统计、任务视图与 T1 准入（11 条）

`6/2/2` fixture 经 API 创建 6 个 done、2 个 cancelled、2 个 todo，候选任务经分拣 API 创建，只带候选项目。H05—H09 通过保存视图的 localStorage fixture 指定视图模式，然后重载；测试的是各视图真实渲染和过滤，没有声称点击了视图选择器。完成/取消数始终分开，闭合百分比不等于人工验收。

| ID／测试文件 | 来源 | 中文场景 | 前置与实际操作 | 明确预期 | AC／FR |
| --- | --- | --- | --- | --- | --- |
| [P1-H01](../../../e2e/projects-p1-health.spec.ts#L8) | 拆分原覆盖 | 正式任务 6/2/2 与旧闭合计数一致 | API 准备 6/2/2 及一个候选；UI 打开概览 | API/UI 总数 10、完成 6、取消 2、未结束 2、闭合 80%；旧 `done_count=8`；两个验收摘要均未记录 | AC-07、AC-10、FR-13 |
| [P1-H02](../../../e2e/projects-p1-health.spec.ts#L28) | 新增/强化 | 候选唯一存在时比例不适用，项目自身延期仍可见 | API 准备已过截止日的项目及唯一待分拣候选；UI 打开概览 | 正式 N/F/C/U 均 0、API 比例 null；页面“暂无正式任务”、百分比不可用；仍显示项目延期；无验收且项目仍 planned | AC-08、AC-10、FR-09 |
| [P1-H03](../../../e2e/projects-p1-health.spec.ts#L52) | 新增/强化 | 全部完成仍不自动验收 | API 准备两个 done；UI 打开概览 | N=2/F=2/C=0/U=0、闭合 100%；当前/最近验收 API 均 null，页面两个未记录提示，项目仍 planned | AC-04 |
| [P1-H04](../../../e2e/projects-p1-health.spec.ts#L52) | 拆分原覆盖 | 全部取消不冒充交付 | API 准备两个 cancelled；UI 打开概览 | N=2/F=0/C=2/U=0、闭合 100%；当前/最近验收均 null，页面无自动通过结论，项目仍 planned | AC-04、AC-09 |
| [P1-H05](../../../e2e/projects-p1-health.spec.ts#L75) | 拆分原覆盖 | 看板排除候选关联 | API 准备 6/2/2 和候选；进入 Issues，保存 board 模式并重载 | Board 控件和正式 Formal item 9 可见，候选标题不存在；概览正式总数仍为 10 | AC-01、AC-10 |
| [P1-H06](../../../e2e/projects-p1-health.spec.ts#L75) | 拆分原覆盖 | 列表排除候选关联 | 同上，独立 fixture 指定 list 模式并重载 | List 控件和正式任务可见，候选标题不存在；正式总数仍为 10 | AC-01、AC-10 |
| [P1-H07](../../../e2e/projects-p1-health.spec.ts#L75) | 拆分原覆盖 | 表格排除候选关联 | 同上，独立 fixture 指定 table 模式并重载 | Table 控件和正式任务可见，候选标题不存在；正式总数仍为 10 | AC-01、AC-10 |
| [P1-H08](../../../e2e/projects-p1-health.spec.ts#L75) | 拆分原覆盖 | 泳道排除候选关联 | 同上，独立 fixture 指定 swimlane 模式并重载 | Swimlane 控件和正式任务可见，候选标题不存在；正式总数仍为 10 | AC-01、AC-10 |
| [P1-H09](../../../e2e/projects-p1-health.spec.ts#L75) | 拆分原覆盖 | 甘特排除候选关联 | 同上，独立 fixture 指定 gantt 模式并重载；正式任务有截止日期 | Gantt 控件和正式任务可见，候选标题不存在；正式总数仍为 10 | AC-01、AC-10 |
| [P1-H10](../../../e2e/projects-p1-health.spec.ts#L96) | 拆分原覆盖 | 普通接受只准入，不启动项目默认小队 | API 准备 runtime、默认小队、6/2/2 及候选；在分拣台点击 Accept 并确认，返回概览并重载 | 总数只从 10 到 11；候选变 accepted/todo，实际项目正确，负责人仍空；候选与原任务执行数均 0，重载总数仍 11 | AC-11、FR-18 |
| [P1-H11](../../../e2e/projects-p1-health.spec.ts#L123) | 拆分原覆盖 | 混合完成与取消的范围闭合不替代验收 | API 将 6/2/2 中两个未结束任务改 done；UI 打开概览 | 比例从 .8 到 1；N=10/F=8/C=2/U=0；页面完成与取消分列、100% 闭合、两个未记录验收；项目仍 planned | AC-04、AC-07 |

## R：风险成员与持续分页（8 条）

R01—R04 的 SQL/API 前置包含：阻塞且逾期的未分配任务、已分配且不在险的父任务、已分配待复核任务、无截止日的未分配子任务、已完成任务和候选任务；保存偏好故意设为“本人、done、隐藏子任务”。每条从概览用键盘打开对应卡片，核对真实响应 ID 和页面项目，显示基准日期/时区，返回概览后检查原偏好未被改写。

R05—R08 各自建立 156 个稳定 ID 任务，最低 ID 初始 done，其余 155 个逾期；其中有父子关系。浏览器提交下一页 cursor 后，route 内才通过 SQL 让最低 ID 重新入险，避免 WS 抢先改变本次游标前置。页面按当前风险集合替换，不累积为冻结清单。

| ID／测试文件 | 来源 | 中文场景 | 前置与实际操作 | 明确预期 | AC／FR |
| --- | --- | --- | --- | --- | --- |
| [P1-R01](../../../e2e/projects-p1-risk.spec.ts#L9) | 新增/强化 | 阻塞下钻严格匹配真实任务 ID | 使用本组混合 fixture；键盘打开 Blocked 1，返回概览 | 响应仅为阻塞任务 ID，总数/卡片均 1；UI 链接仅该任务，候选排除；个人筛选不影响集合、基准日/时区可见、偏好保持 | AC-13、AC-16 |
| [P1-R02](../../../e2e/projects-p1-risk.spec.ts#L9) | 新增/强化 | 逾期下钻排除已完成和未来日期 | 独立混合 fixture；键盘打开 Overdue 1，返回概览 | 仅未结束且过去截止日的那条任务；已完成的过期任务和未来日期任务不进入结果；API/UI 数量 1，基准和偏好保持 | AC-12、AC-16 |
| [P1-R03](../../../e2e/projects-p1-risk.spec.ts#L9) | 新增/强化 | 未分配包含隐藏、无日期子任务 | 独立混合 fixture；打开 Unassigned 2，再点击 Table | 响应恰为阻塞任务及未分配子任务两个 ID；两种渲染均保留两条，子任务显示 No due date；候选排除、偏好恢复 | AC-13、AC-16 |
| [P1-R04](../../../e2e/projects-p1-risk.spec.ts#L9) | 新增/强化 | 待复核信号仅返回对应正式任务 | 独立混合 fixture；打开 Awaiting review 1，返回概览 | 响应仅为内置 in_review 任务 ID；UI 仅该链接；基准与原筛选偏好保持。此条不替代自定义/归档状态的后端矩阵 | AC-16；PRJ-006 |
| [P1-R05](../../../e2e/projects-p1-risk.spec.ts#L54) | 拆分原覆盖 | 变化后继续前进，后续未变化页仍保留提示 | 先以真实 API 遍历静态 155 条，再让最低 ID 在 UI 第二页请求时重入；继续第三页 | 静态 ID 全量/排序精确且包含子任务；第二页 refreshed=true、total=156、严格在旧末 ID 后；第三页 refreshed=false 仍向前；每页仅 50 条且变更提示仍在 | AC-16、FR-05；ADR-05 |
| [P1-R06](../../../e2e/projects-p1-risk.spec.ts#L87) | 拆分原覆盖 | 从头刷新重新包含低 ID | 进入变化后的第二页，确认重入任务尚不可见；点击 Refresh from start | 请求不带旧 snapshot_version/cursor；首项为重入最低 ID、total=156，显示 50 条；成功后清除持续变更提示 | AC-16；ADR-05 |
| [P1-R07](../../../e2e/projects-p1-risk.spec.ts#L112) | 新增/强化 | 刷新失败保留非空当前页和提示，重试可恢复 | 进入有 50 条的变化第二页；将无 cursor 请求注入 503；从头刷新失败后解除故障，点击错误区 Retry | 错误可见，但原 50 条文本和顺序不变、持续提示保留；成功重试才重新出现最低 ID 并清除错误与提示 | AC-16、FR-09；ADR-05 |
| [P1-R08](../../../e2e/projects-p1-risk.spec.ts#L146) | 拆分原覆盖 | 正总数的空后缀不能显示项目无风险 | 走到第三页后，下一页请求到达时 SQL 将剩余五条改 done | 当前页 items 为空、total=151、next_cursor=null；页面明确当前位置之后无结果，而不是 No matching issues；持续提示与从头刷新保留，Next page 消失 | AC-16、FR-09；ADR-05 |

## P：进展、修订、验收与证据（15 条）

每条都有独立项目。`seedProgress` 通过 API 预览/发布构造历史；`seedProgressExecution` 准备合成执行及权限。除明确标记的 API 前置外，下表的预览、发布、更正、移除证据和通知导航均实际操作界面。

| ID／测试文件 | 来源 | 中文场景 | 前置与实际操作 | 明确预期 | AC／FR |
| --- | --- | --- | --- | --- | --- |
| [P1-P01](../../../e2e/projects-p1-progress.spec.ts#L14) | 拆分原覆盖 | 提及预览识别成员但不发布或执行 | API 准备另一成员及 agent；UI 输入正文、选成员/agent 提及并预览 | 200 预览收件者只有该成员，两种 mention 身份保留；显示 agent 不执行说明；进展/outbox/inbox/agent 执行均为 0 | AC-18、AC-27、FR-18 |
| [P1-P02](../../../e2e/projects-p1-progress.spec.ts#L36) | 新增/强化 | 提及末尾空格的快速预览跨过 debounce 仍可发布 | UI 选成员/agent 提及，输入尾空格后在 300ms 内发预览；等待超过 350ms | 200 响应正文已去尾空格，收件者不丢；Publish 仍可用，编辑器未回挂；没有发布记录 | AC-18；PRJ-008 |
| [P1-P03](../../../e2e/projects-p1-progress.spec.ts#L62) | 新增/强化 | 发布时的系统快照不被后来任务变化改写 | API 准备一个 done；UI 勾附统计、预览并发布；API 将任务改 todo，重载 | 首发 201、作者/正文正确且只有一条；预览和历史快照 completed=1；之后实时 completed=0，历史仍 1，页面仍有发布时统计区 | AC-18、FR-05 |
| [P1-P04](../../../e2e/projects-p1-progress.spec.ts#L86) | 新增/强化 | 立即取消仍保留最后输入字符 | UI 输入完整尾句后立即取消，重开编辑器，再重载并重开；监听进展非 GET 请求 | 两次恢复均保留完整正文；预览/发布写请求为 0，服务端无进展 | AC-18；PRJ-008 |
| [P1-P05](../../../e2e/projects-p1-progress.spec.ts#L104) | 新增/强化 | 离开概览不丢未发布稿 | UI 输入后切 Issues，再回 Overview 并打开进展编辑器 | 切走后编辑器卸载，回来恢复完整末句；服务端无进展 | AC-18；PRJ-008 |
| [P1-P06](../../../e2e/projects-p1-progress.spec.ts#L117) | 拆分原覆盖 | 已提交但响应丢失时重用同一意图 | UI 提及成员、附执行证据和统计；首次 POST 经 route.fetch 真提交后 abort，再次点击 Publish | 首次 201 已落一条但 UI 提示未确认；重试 200/replayed、两次 payload 完全相同、相同 update/revision；仅一条进展/outbox/inbox；合成执行仍只有原一条 | AC-19、AC-27、FR-07、FR-12 |
| [P1-P07](../../../e2e/projects-p1-progress.spec.ts#L158) | 新增/强化 | 缺少更正理由可修复，原文作者时间不变 | API 准备 revision 1；UI 更正但不填理由，收到错误后补理由再预览/发布，打开历史 | 缺理由 422、输入保留且仍只有 revision 1；补后历史为 2/1，原文和原编辑者不变，新理由可见，首发作者/发布时间不变 | AC-20、FR-01 |
| [P1-P08](../../../e2e/projects-p1-progress.spec.ts#L185) | 新增/强化 | 过期更正先比较新版再显式重试 | API 准备原进展；UI 开始更正后另一成员 API 写 revision 2；UI 预览、复核保存并再发布 | 409 `project_update_revision_conflict`；本稿和对方新稿均可见；显式重试后历史 3/2/1，最新为本稿、中间修订者为另一成员 | AC-20、FR-07 |
| [P1-P09](../../../e2e/projects-p1-progress.spec.ts#L206) | 拆分原覆盖 | 通过验收缺依据时保留输入 | UI 填通过验收正文/范围但无依据；预览失败后补可复核说明再发布 | 首次 422、正文和范围保留、记录为 0；补后预览 200/发布 201，当前描述验收 passed 且绑定正确版本；项目状态不变 | AC-06、AC-18 |
| [P1-P10](../../../e2e/projects-p1-progress.spec.ts#L223) | 新增/强化 | 验收前目标变化必须显式采用新描述 | UI 先写验收稿；API 更改项目描述；UI 预览失败后选择按新描述复核并再发布 | 409 `project_description_conflict`，本稿不丢、新描述可见、未落记录；明确采用后 draft 携新描述版本，发布后验收绑定新版 | AC-05、AC-25、FR-04 |
| [P1-P11](../../../e2e/projects-p1-progress.spec.ts#L241) | 拆分原覆盖 | 旧通过验收仍指向原目标，新失败结果独立存在 | API 准备 v1 通过验收并改描述为 v2；UI 查看旧版提示，再发布 v2 failed 验收 | v2 初始当前验收为空；旧记录显示需复核；新 failed 不要求通过依据并绑定 v2；原记录仍 passed/v1/原描述快照 | AC-05、AC-06 |
| [P1-P12](../../../e2e/projects-p1-progress.spec.ts#L259) | 拆分原覆盖 | 指定修订的执行证据打开授权记录 | SQL/API 准备合成 completed execution；UI 附证据、发布并点击执行按钮 | 预览含该执行版本；读取指定 update/revision/execution 路径返回 200，workspace/project/update/revision/task 身份相符；执行对话框可开闭；执行数仍 1 | AC-26、FR-06、FR-17 |
| [P1-P13](../../../e2e/projects-p1-progress.spec.ts#L280) | 新增/强化 | 旧通知跨进展分页定位，同时隐藏私有执行 | API 创建带通知/私有证据的目标进展，再创建 20 条较新记录；另一成员 UI 从收件箱打开通知 | 目标已不在首批20条中；通知 revision 为字符串；仍导航到准确项目/update 并显示目标正文；私有执行无按钮且直接 GET 为403/source code，普通编辑入口仍可见 | AC-26、AC-27、FR-06、FR-13 |
| [P1-P14](../../../e2e/projects-p1-progress.spec.ts#L308) | 新增/强化 | 来源失权只拒绝证据，保留成员草稿 | 成员 UI 为可读执行做预览后回编辑；owner API 收回 agent 授权；成员再预览，移除证据后发布 | 403 `project_evidence_forbidden`、提示文本保留、空间仍可读、无记录；移除后成功发布原文，证据为空、作者仍该成员；执行不增加 | AC-26、FR-06、FR-08 |
| [P1-P15](../../../e2e/projects-p1-progress.spec.ts#L337) | 新增/强化 | 更正中再次提及同一成员不重复通知 | API 准备已通知成员的 revision 1；UI 更正，再提同一成员及 agent 并发布 | 修订到 2、收件者仍该成员；outbox/inbox 各仅一条，agent 执行为 0，项目状态不变 | AC-20、AC-27、FR-12、FR-18 |

## L：项目生命周期、权限与恢复（15 条）

`runningWork` 用 API 建项目/任务并通过 SQL 放入一条 running execution。状态和删除的 UI 操作随后直接验证其保留情况；这不是实跑智能体。L13 的角色降级是 SQL 故障场景前置，L14 的离开工作空间则调用真实业务 API。

| ID／测试文件 | 来源 | 中文场景 | 前置与实际操作 | 明确预期 | AC／FR |
| --- | --- | --- | --- | --- | --- |
| [P1-L01](../../../e2e/projects-p1-lifecycle.spec.ts#L37) | 新增/强化 | 取消完成警告不改变项目 | API 准备进行中项目及未完成任务；UI 选择 Completed 后取消警告 | 警告显示一个未结束任务及无验收；取消后项目仍 in_progress，状态审计行数为 0 | AC-21 |
| [P1-L02](../../../e2e/projects-p1-lifecycle.spec.ts#L52) | 新增/强化 | 完成理由入审计，任务与运行执行不联动 | 准备 runningWork；UI 填完成理由并确认 | 项目 completed；任务仍 todo，原执行仍 running 且 issue_id 不变；数据库完成审计 reason 精确等于输入 | AC-21、AC-22、FR-18 |
| [P1-L03](../../../e2e/projects-p1-lifecycle.spec.ts#L69) | 新增/强化 | 暂停、取消、重开和完成都实际走状态控件 | 准备 runningWork；UI 顺序选择 paused/cancelled/in_progress/completed，完成时确认 | 每步按钮与服务端项目状态匹配；任务始终 todo、同一执行 running/issue_id 不变，最终执行数仍 1。原大测试该循环走 API，本条强化为 UI | AC-22、FR-18 |
| [P1-L04](../../../e2e/projects-p1-lifecycle.spec.ts#L86) | 新增/强化 | 普通成员没有删除入口，操作拒绝不等于撤权 | API 准备成员后在独立浏览器上下文中用其身份打开项目；查看菜单，并用其 API 尝试 DELETE | 无 Delete project 菜单/时区设置；DELETE403 `project_permission_denied`；概览仍可见、普通项目 GET 仍可读 | AC-24、AC-26、FR-16 |
| [P1-L05](../../../e2e/projects-p1-lifecycle.spec.ts#L107) | 新增/强化 | 取消删除不发 DELETE | API 准备一条关联任务；UI 打开删除影响确认并取消；监听 DELETE | 提示一条任务保留；关闭确认后 DELETE数为0，项目仍可读 | AC-23；PRJ-012 |
| [P1-L06](../../../e2e/projects-p1-lifecycle.spec.ts#L121) | 新增/强化 | owner 经真实删除对话框保留任务和执行 | 准备 runningWork，API核对 delete-impact；UI 打开确认并 Delete | 影响说明保留任务/执行且正式数1；导航回项目列表；任务 project_id=null/status=todo，原执行仍running且总数1。原大测试删除走API | AC-23、AC-24 |
| [P1-L07](../../../e2e/projects-p1-lifecycle.spec.ts#L135) | 新增/强化 | 删除失败仍能读项目并重试 | 浏览器 DELETE 注入503；UI确认删除，解除故障后在保持打开的确认框内重试 | 首次收到503时确认框保持打开、服务端项目仍可读；后续真实删除成功才导航至项目列表 | AC-23、FR-08、FR-09 |
| [P1-L08](../../../e2e/projects-p1-lifecycle.spec.ts#L150) | 拆分原覆盖 | 初次概览失败可通过 Retry 恢复 | 概览 GET 注入503后打开页面；解除route并点击Retry | 初次显示加载错误；重试真实接口后显示空正式集合 | AC-17、FR-09 |
| [P1-L09](../../../e2e/projects-p1-lifecycle.spec.ts#L160) | 新增/强化 | 成功后的刷新失败不得继续显示健康结论 | API准备已指派任务/负责人；先加载概览，再注入503并点Refresh；解除后Retry | 显示旧统计提示，No current risk signals消失，风险按钮禁用；真实重试后按钮恢复可用 | AC-17、FR-09 |
| [P1-L10](../../../e2e/projects-p1-lifecycle.spec.ts#L174) | 拆分原覆盖 | 不支持新能力时保留描述并限制编辑 | 将真实能力响应的四能力改为false，再打开项目 | 显示安全描述编辑限制；Goal template/Write progress入口缺失，原描述可见且数据库不变。此条是能力响应故障注入，不是真实旧router验证 | FR-13、FR-14 |
| [P1-L11](../../../e2e/projects-p1-lifecycle.spec.ts#L188) | 新增/强化 | owner 修改规划时区并清回UTC | UI填Asia/Shanghai并Save、重载；再清空输入并Save | API时区先变Shanghai，重载可见；清空后明确显示未配置/使用UTC，API返回UTC | FR-03 |
| [P1-L12](../../../e2e/projects-p1-lifecycle.spec.ts#L202) | 新增/强化 | 无效时区保留输入，修正后可保存 | UI输入不存在的IANA时区并Save；随后改Europe/London再Save | 错误可见、无效输入不丢、API仍UTC；修正后API变London | FR-03 |
| [P1-L13](../../../e2e/projects-p1-lifecycle.spec.ts#L215) | 新增/强化 | 时区写权限丢失不擦除仍授权的进展稿 | owner UI打开进展并输入；SQL将其降为member；使用尚显示的时区表单Save，最后恢复fixture角色 | 真实PUT403 `project_permission_denied`；进展正文和概览保留，不作整个空间撤权 | FR-08、FR-16 |
| [P1-L14](../../../e2e/projects-p1-lifecycle.spec.ts#L233) | 新增/强化 | 无WS通知时真实空间失权仍清保护内容 | 独立成员上下文屏蔽WS；UI打开保护描述并写稿，确认稿已持久化；成员API真实leave返回204；UI点Refresh | 概览GET404携`workspace_access_denied`；进展编辑器、保护描述和未发送文本从页面消失，持久存储中的该稿也被清除 | AC-26、FR-08、FR-15 |
| [P1-L16 成员离开](../../../e2e/projects-p1-lifecycle.spec.ts#L271) | 新增/强化 | 无WS通知时成员自行离开后请求有界 | 独立成员上下文屏蔽WS并打开项目；成员API真实leave返回204后开始计数，UI点Refresh；观察20秒内所有`/api/`请求 | 概览GET404携`workspace_access_denied`，保护描述消失；任一端点>5次、20秒合计>40或首个5秒之后>5即失败，之后renderer响应<1秒；按5秒分桶及端点计数作为报告附件 | AC-26、FR-08 |
| [P1-L16 owner移除](../../../e2e/projects-p1-lifecycle.spec.ts#L271) | 新增/强化 | 无WS通知时owner移除成员后请求有界 | 同上，撤权改为owner真实DELETE成员行返回204 | 同上 | AC-26、FR-08 |
| [P1-L15](../../../e2e/projects-p1-lifecycle.spec.ts#L316) | 拆分原覆盖 | 中文390px键盘预览与底部保存按钮可达 | API/locale设置中文；390×844打开项目，用Enter进入进展并预览；滚动到底部保存按钮 | 发布按钮可滚入视口、文档无横向溢出；保存按钮25/50/75%九点hit-test均命中自身/子元素，没有被聊天浮钮遮挡 | AC-18（预览片段）；PRJ-014 |

## D：真实 Electron（6 条）

每条实际启动 Electron，使用已构建 renderer 和现有 preload fixture。共同 teardown 检查 `pageerror=[]` 与 `daemonStarts=0`，再关闭进程、代理与临时 profile；这些共同断言不另计测试条数。

| ID／测试文件 | 来源 | 中文场景 | 前置与实际操作 | 明确预期 | AC／FR |
| --- | --- | --- | --- | --- | --- |
| [P1-D01](../../../e2e/projects-p1-desktop.spec.ts#L6) | 拆分原覆盖 | 原生导航及键盘风险下钻 | API准备一条逾期任务；Electron经Projects打开项目，用Enter开Overdue，再返回Overview | 正确任务链接与概览可见；总数1，任务执行数0 | AC-16、FR-18；PRJ-014 |
| [P1-D02](../../../e2e/projects-p1-desktop.spec.ts#L20) | 新增/强化 | 原生快速尾空格预览不被卸载flush打断 | Electron输入带尾空格正文，在300ms内Preview；等待350ms后Publish | 无成员通知提示，Publish仍在且编辑态Preview按钮不回挂；正文归一化后可见，服务端只有一条进展。原原生快预览基础上加入尾空格与持久数量断言 | AC-18；PRJ-014 |
| [P1-D03](../../../e2e/projects-p1-desktop.spec.ts#L42) | 新增/强化 | 原生立即取消和重载保留最后字符 | Electron输入末句、Cancel、重载、重开进展 | 草稿完整恢复；服务端进展数0 | AC-18；PRJ-008、PRJ-014 |
| [P1-D04](../../../e2e/projects-p1-desktop.spec.ts#L57) | 新增/强化 | 原生历史显示原文和更正原因 | **API准备**首发及更正；Electron打开项目并点Revision history | 当前更正文案可见，历史中原文及更正原因可见。本条验证原生读取呈现，不宣称通过原生UI创建更正 | AC-20；PRJ-014 |
| [P1-D05](../../../e2e/projects-p1-desktop.spec.ts#L74) | 新增/强化 | 中文680px原生窗口可键盘进入进展 | API准备中文进展后设置中文并重载，Electron内容区域调为680×900；打开修订历史，再聚焦记录进展并Enter | 已有中文进展及修订历史可见，概览无横向溢出；键盘打开后预览发布按钮可见。保留原中文历史断言并强化原生键盘交互 | AC-18（预览入口）；PRJ-014 |
| [P1-D06](../../../e2e/projects-p1-desktop.spec.ts#L94) | 新增/强化 | 两个原生项目不共享草稿 | API准备两项目；Electron在第一项目写稿并取消，切第二项目查看，再回第一项目 | 第二项目没有第一项目文本；返回第一项目恢复原稿；两项目服务端进展均0 | AC-18；PRJ-008、PRJ-014 |

## C：双页面收敛（1 条）

| ID／测试文件 | 来源 | 中文场景 | 前置与实际操作 | 明确预期 | AC／FR |
| --- | --- | --- | --- | --- | --- |
| [P1-C01](../../../e2e/projects-p1.spec.ts#L8) | 拆分原覆盖 | 两个真实Web页面观察30次服务端变更 | 同一browser context的两页打开同项目；**API执行**截止日期变更10次、负责人变更10次、接受候选10次；记录HTTP提交开始至两页DOM及新版本就绪的耗时 | 每次两页计数符合预期且与新overview版本一致；恰好30原始样本；整体及三个分类的P95均≤5000ms；无pageerror、原任务及候选执行数均0。不是30条测试，也不是Web与Electron互测 | FR-15；AC-11（准入收敛片段） |

## 本矩阵不扩大的结论

1. 当前只确认清单与源码映射；61条的通过/失败/跳过、构建指纹及命令退出码待父任务统一运行后写入。此前8/8通过记录继续作为旧候选证据保留。
2. “新增/强化”36条包含把既有后端/组件契约落实到浏览器的验证，不代表新增36项需求，也不替代原canonical规则矩阵。
3. AC-22/23中的真实I1迭代归属与结束快照仍待I1；本轮只验证现有任务、项目关联和执行保留。移动端185条兼容测试、迁移/锁竞争及服务端30分钟分页压力各有自己的证据，不能计入本表61条。
4. 本表无真实智能体账户调用、外部通知或生产部署结论；合成execution的原有行不等于本轮启动了执行。C01的预算是整体及分类P95≤5秒，不把它改写为每个样本均≤5秒。
