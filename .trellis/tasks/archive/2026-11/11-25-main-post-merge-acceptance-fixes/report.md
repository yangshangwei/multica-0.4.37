> 用户于2026-11-26随后授权提交、推送并合并main，见 [集成记录](research/integration-publication-20261126.md)；发布HOLD不变。

> 2026-11-26 续修结果见 [continuation-report.md](research/continuation-report.md)。下文为前轮原报告，失败、计数和交付事实原样保留。

# main 合并后验收修复报告

当前结论为 **HOLD / 不建议发布**。已完成可证实的权限、依赖、诊断和客户端修复及回归，两端真实 Main 原旅程通过；完整 service 包和旧 CLI 仍有实际失败。独立终审及全局资源收尾证据随报告交付，未解决项不以重试、放宽守卫或增加超时掩盖。

工作区 `/Users/artisan/.codex/worktrees/main-acceptance-fixes/multica-0.4.37`，分支 `codex/main-post-merge-acceptance-fixes`，基线 `a7bf6ba70fcc3af8171d6a406e0fae6a1b0c29e1` 加未提交变更。原候选 `103e2bc329a9fb845d9f45c20b2a9bb3ebb54625`、默认400PASS/138SKIP、补测117PASS/21FAIL和历史114PASS/24FAIL均原样保留。没有提交、推送、发布或部署。

独立最终对账：原21个最近失败身份现为 **11 PASS / 9 policy-BLOCKED / 1 FAIL**，完整身份、标题、原记录、新证据及不同冻结候选范围见 `research/original-21-dispositions.json`。非浏览器D01/D02/D03/D10另列，不能由这21项推断全产品通过。`research/independent-final-review.md`已完成Main和全局资源附录，无待补终审材料；父任务仍in_progress以追踪实际未解决发布阻塞。

## 根因与修复边界

| 问题 | 原始证据 → 区分结果 | 最小修改与验证 | 当前处置 |
|---|---|---|---|
| D01 评论管理员越权 | 真实任务凭据继承绑定人owner/admin；独立复核又发现有效Agent/Task头可走旧作者归属回退 | 评论管理分支要求人类身份；评论写作者仅接受已验证任务上下文，其他凭据保留真实成员身份。上下文同时决定事件操作者和编辑来源任务。实际API颁发矩阵、有效头组合、正反权限及副作用检查通过 | 全handler normal/race各6211PASS/7既有SKIP，全router各2284PASS/8既有SKIP；独立复核接受 |
| D02 service race锁冲突 | 原55P03的持锁者仍UNKNOWN；新观测到autovacuum冲突锁，但与原场景无同时失败证据。原wrong_consumer多次通过 | 未修改安全锁或凭据守卫。支持工具链完整normal/race又出现checkpoint/stage/knowledge超时；两次40分钟结束均不完整 | 未解决；不称Go数据竞争，不以窄测通过关闭 |
| D03 安全扫描 | Go1.26.11基线消除原13项stdlib发现，但WebSocket仍可达；当前x/net公告还要求0.62.0 | 最小现有依赖升级，无新增模块；7目标源码、8实际二进制扫描通过；最终作者身份修复后当前源码扫描再次exit0 | 已验证修复范围；跨平台静态扫描不等于平台运行验收 |
| D04 Host启动 | 原许可status0/503与池压力无法定因；新完整memory/order通过。新CLI观察又显示无捕获Lock等待、较长ClientRead事务间隔 | 增加脱敏transport/context/单调时间事实，无扩池、加时或重试。完整Host旅程与daemon/controller normal/race通过 | 新旅程通过；历史慢请求根因仍UNKNOWN |
| D05 Namespace | 原9身份缺实际Probe原因；原fixture要求privileged/unconfined，不符合本次安全策略约束 | 增加安全Probe阶段/errno/context诊断。另跑默认策略探针得到helper_start errno1，仅解释该探针 | 原9身份policy-BLOCKED，历史原因UNKNOWN；不混用配置证据 |
| D06 打包Main | 原旧Collaboration标题失配；后续控制链未跑 | 改为当前明确Current handling语义，生产包重新绑定。修复pnpm依赖收集、私有framework拓扑和Docker文件传输。Linux完整原旅程1PASS，Mac恢复原旅程知识OFF前置后1PASS；两端launcher0、源码/包未变、清理通过 | Linux约3.5分钟、Mac约1.4分钟完整通过。Mac默认ON失败另存，不能用OFF通过覆盖默认ON范围 |
| D07 manual/PAY/资源 | PAY旧标题、隐藏资源分组、manual取消范围恢复入口 | 语义定位器修复；用户明确批准新“返回上次协作”入口，复用两端已有恢复机制。PAY2/资源3通过；manual最新全Web/Desktop2通过，精确保留同任务/620滚动/单账户标签与38截图 | 实际旅程与独立复核通过；各冻结候选范围单列 |
| D08 Snooze | 错误工作区菜单；路由提交前点旧Inbox；观察器reload/close时丢读响应 | 等待目标href、排空请求、显式封口并最终检查所有已接纳响应。独立发现的迟到错误原记录保留；确定性反例与完整6项新回归通过 | 最新6PASS、9封口/9最终检查、0观察失败；独立复核接受 |
| D09 延迟拒绝提示 | 页面重复提示使全局定位器匹配6个元素 | 限定详情根的直接alert；真实自然响应暂停先独立验证，再完整Web/Desktop各3子场景通过 | 2完整身份通过 |
| D10 公共GitHub | 历史403缺头，不能归因；本次真实导入403且remaining0 | 安全分类响应，不回显敏感头/正文，不假设所有403是限流；测试保持失败无写入 | 新失败确认为匿名配额；历史原因UNKNOWN，外部依赖FAIL |
| D11 旧CLI | 原240秒只见9次relay；新原旅程仍超时，复制/单relay短探针不解释它 | 清理错误不再被吞；独立快照读并行但全部完成后才进下一操作，保留10次操作和240秒。新增DB等待/请求计时，没有证据支持扩池/加时 | 三次完整尝试均未达完整10次通过，性能原因未定；清理失败历史保留 |

generic `UploadFile(comment_id)`单独确认了当前实现契约：成员资格、同工作区关联、独立上传者来源；未找到要求它等同评论编辑的明确产品契约，因此没有擅自改写权限或宣称已修复该观察。

## 变更文件与简化

- 权限：`server/internal/handler/comment.go`，直接处理器/真实路由测试及合法任务编辑fixture；复用现有验证上下文与身份分类，不改变全局旧归属解析器。
- 安全：`server/go.mod`、`server/go.sum`、`server/internal/handler/skill.go`及错误分类测试；删除无依据的统一限流建议。
- 运行诊断：`server/internal/daemon/collaboration.go`、`collaborationrun/controller.go`及对应测试、Host harness和E2E transport事实；复用既有日志与回执，没有新服务或配置层。
- 客户端：共享AppSidebar、两端平台scope wiring、en/zh文案、对应测试和受影响E2E fixtures；独立返回动作保留原分组展开与显式列表导航。
- CLI：已有fixture清理与snapshot等待顺序，保留所有业务断言。规范及内置技能文档同步。确切全文件清单以最终diff及来源清单为准。

## 验证范围与保留失败

独立对账的其余81包：normal/race每种11155PASS、6既有SKIP；原环境误配造成的36FAIL/7SKIP历史保留。修正仅限测试子进程umask、安全临时目录、实际Redis索引隔离与opt-in；未修改产品断言。handler/router和daemon/controller分别计数，不合并成独立功能数。

前端首轮无worker限制的既有用例5秒超时保留；按仓库原2worker命令完整views7856/Web322/Desktop1314通过，core5503/docs64先前通过。新返回入口另有共享/平台/scroll回归、生产构建、lint/typecheck与完整manual旅程。已有E2E lint诊断与基线一致，原非零日志不伪装成全绿。

每组新运行使用新标签、实际来源/工具/制品/配置绑定。阶段冻结源码与后续小变更分别说明，不把旧SHA/clearance改写成新候选。旧精选88文件摘要匹配，全部46个原失败身份报告/完成回执路径存在；目录存在不代表全部嵌套证据完整，具体lane已另行核对。

原生iOS、Windows运行、未安装浏览器、pg_bigm及Namespace策略限制单列。没有调用个人登录Agent或付费模型、生产数据、发布标签、部署或降低Docker安全策略。

## 资源与交付限制

最终资源核对：4个自有基础容器全部停止，66个记录服务端口关闭，自有进程0；PostgreSQL停止前其他客户端0，221个数据库保留。3个任务原生传输卷经精确owner检查后删除，原生包、控制器原件和失败证据保留。旧验收数据库/卷及原工作区无关未跟踪文件未改动。证据为 `integration/global-resource-audit.json`、`postgres-final-cleanup.json`、`redis-cleanup.json`及native清理记录。

macOS协议状态存在证据边界：最早native基线是在部分初步客户端运行之后才捕获，包含worktree Electron默认项。后续系统状态出现来源未知的变化：Electron注册集合消失，默认变为Applications/Multica；旧基线保护因此拒绝一次运行，未执行重建或重置。两次只读快照确认新的实际状态后，Mac03以该状态为显式基线运行并精确恢复；最终独立getter再次完全匹配。无法证明或恢复整项工作开始前的未知状态，不能把先前campaign的记录冒充本次起点。

自动安全审核曾中断评论子代理，未给出具体违规操作。主会话继续本地防御性代码修复与回归，保留中断记录。此事不是产品漏洞结论，也未导致绕过任何宿主机/数据库守卫。

交付包括独立终审、当前Main结果、仍未解决D02/D11/外部或平台限制及最终资源账本。发布建议保持不发布：必须先闭合service完整包失败、旧CLI完整240秒兼容性和所需平台/配置缺口，再对新的最终候选准入。所有原始失败和中间准备问题保留。

## 后续提交授权

用户于2026-11-25明确要求提交并推送。修复代码、规范、任务文档和精选JSON验证回执交付到当前修复分支；本报告前文的未提交状态描述验证结束时的事实。Git证据清单及SHA256见QA根目录 `publication-manifest.json`；原始日志、trace、截图、编译缓存、冻结构建和私有运行配置仍在本机，不因被回执引用而视为已随Git发布。此次推送不改变HOLD结论，不合并main、不发布或部署。

代码与规范提交：`d90d76e6bd90e9c9e2f45fef0e0efd48d359bed6`。本节补充交付状态，不回填或改写任何先前运行的候选SHA、计数、退出码和清理结果。
