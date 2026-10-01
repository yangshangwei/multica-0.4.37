# Handoff：平台管理后台

保存日期：2026-10-01。仓库：`/Volumes/artisan/code/2026/multica-0.4.37`。
保存时分支：`main`；HEAD：`6dd927657`（fix(navigation): make AI tools easier to find）。新会话先核对实际Git状态，不假设仍停在此提交。

## 1. 当前任务与授权状态

父任务：`.trellis/tasks/10-01-platform-admin-console`。

已经完成需求、架构、详细设计、容量、迁移、测试和实施分解，并创建7个实施子任务。当前父子任务全部为`planning`，`implementation_authorized=false`；尚未实现管理后台功能、运行迁移或创建管理员。

本次用户要求保存handoff，准备新会话推进；没有在当前会话启动实现。新会话若收到用户明确要求“按现有方案开始实施”，应将其作为新的实施授权，按Trellis核验最新设计并启动S01，不因本快照中的旧false字段永久停留在规划。若新消息仅要求阅读/评审，则继续对应范围。

## 2. 已确认的需求，不要重新访谈

- 企业内部先落地，后续支持多个客户组织。
- 首期100–1000台桌面终端。
- 管理员与普通用户共用用户、密码和登录认证，平台权限与工作空间权限独立。
- 管理后台覆盖账号、终端、任务执行、能力策略、监控告警、统计与系统管理；首期开放六个菜单，完整八菜单分期实现。
- 复用现有Go/Next.js/PostgreSQL及共享包，不新增依赖，不先拆服务。
- 已有普通注册仍为用户名/密码/姓名，注册自动登录、用户自行创建空间；不新增邀请码、注册审批或默认加入别人的空间。

容量负载、阈值和保留天数中未由用户明确给定的数值都是工程初值；不能当作实测或真实数据删除授权。

## 3. 首先阅读

1. 根`AGENTS.md`、`CLAUDE.md`、`.trellis/workflow.md`及适用spec。
2. 父任务[README](README.md)、[PRD](prd.md)、[设计总纲](design.md)、[实施计划](implement.md)。
3. [架构](architecture.md)、[安全设计](security-design.md)、[终端执行协议](terminal-execution-design.md)、[详细页面/API设计](detailed-design.md)。
4. [测试规格](test-spec.md)、[容量](capacity-plan.md)、[迁移发布](migration-rollout.md)。
5. [独立审阅记录](research/design-review.md)、[现状源码索引](research/current-state.md)、[本轮验证](verification.md)。
6. S01目录中的prd/design/implement及implement.jsonl/check.jsonl。

父文档是共同契约，子任务文档是范围和所有权。文件中的“新增”路径代表计划落点，不代表文件已经存在。

## 4. 子任务与依赖

| ID | 目录（均在`.trellis/tasks/`下） | 范围 | 依赖 |
| --- | --- | --- | --- |
| S01 | `10-01-platform-admin-access` | 平台身份权限、组织/审计/operation基础、后台入口、通用幂等查询 | 无 |
| S02 | `10-01-platform-admin-installations` | 安装接入证明、daemon关联、终端台账与状态 | S01 |
| S03 | `10-01-platform-admin-executions` | 全局任务/执行元数据与权限投影 | S01 |
| S04 | `10-01-platform-admin-controls` | 单次取消、停止/恢复接单、回执与协调 | S01、S02、S03 |
| S05 | `10-01-platform-admin-accounts` | 账号禁用/恢复、强制改密、凭据撤销 | S01 |
| S06 | `10-01-platform-admin-observability` | 总览、固定告警、服务健康和审计页面 | S02、S03、S04 |
| S07 | `10-01-platform-admin-acceptance` | 跨层安全/兼容/容量/原生验收与发布演练 | S01–S06 |

先完成S01并验证，再考虑S02/S03/S05并行。router、公共导出、locale注册、迁移编号和sqlc生成由当批集成人串行协调。原生子代理只接有明确所有权的独立范围，不相互还原改动。

## 5. 最容易踩错的设计决定

### 共用认证不等于共用授权

管理路由为Web `/admin`，登录复用`/login?next=/admin`；`admin`已是保留slug。管理入口不能复用需要workspace的DashboardGuard。API在Auth内、workspace guard外，额外验证当前平台权限。

首期自托管password模式，只允许完整有效的人类JWT/cookie；`RequireHumanActor`单独使用不够，它允许个人PAT。当前`/api/cli-token`的password分支存在PAT→JWT转换入口，设计要求在启用角色前收紧该签发入口，**不能把公共LockPasswordSession改成JWT-only**，否则破坏PAT续期。

历史PAT兑换的JWT也必须失效：首次授予/提升平台角色时提升目标账号版本、撤销旧凭据并要求重新密码登录。普通CLI浏览器JWT→PAT和PAT→PAT续期须验证仍可用。

管理员A禁用/恢复B时，断连目标必须显式为B，审计操作者为A；不能原样复用从请求X-User-ID取目标的publishPasswordRevocation。最后有效超级管理员保护覆盖降级、禁用和强制恢复。

### 终端身份与运行状态

物理设备、device_id、统计install_id、daemon_id、runtime_id不是一个对象。台账使用经过持钥证明的受管理安装实例；历史关联未知就显示未关联，不按名称或IP猜归属。

安装enroll/bind挑战必须绑定部署、组织、user/auth_version、workspace、daemon、公钥、nonce和时效；双方签名按服务端发行的原字节，不各自重序列化。首期桌面与daemon主体必须相同。

客户端活跃、daemon可达、执行准备度分开。沿用现有daemon心跳/sweeper语义，新客户端上报拟定60秒错峰；日活数据不能用于实时在线判断。

### 取消、门禁、幂等与审计

业务issue与一次task执行分开；改issue为cancelled不自动停止全部执行。单次取消立即复用现有task终态变更，独立追踪daemon ack。超过确认截止只标未确认，不反转已应用取消；重连仍协调持久终态。

停止接单由服务端在全部HTTP/WS/batch/fallback/内部claim路径执行，在途继续；配置提示或隐藏按钮不是门禁。恢复接单要考虑原空队列/领取缓存失效。

通用`admin_operation`幂等创建与`GET /api/admin/operations?idempotency_key=...`由**S01**负责，S04/S05复用。未知提交结果用同key查询/重试，不能换key重放。

不同管理员对同一task的取消各有operation与请求审计，通过root_operation_id共享一次取消事实和确认。不能给“同target未确认操作”加唯一约束而丢掉第二人的请求；根确认后的子请求同步必须可恢复。

### 数据与内容边界

平台读权限只提供受控运行元数据；私有标题/正文/日志/路径/附件仍按原资源权限获取。平台管理员不自动成为所有空间成员。估算成本不当作账单，未上报用量不填0。

禁止数据库外键和级联。所有索引（包括唯一/新表主键所需索引）独立单语句CONCURRENTLY迁移；避免inline PRIMARY KEY/UNIQUE隐式普通索引。新编号按实施时实际仓库分配。

## 6. 已有验证与未做工作

规划交付时：8个任务Trellis validate均通过；37份Markdown、182条上下文引用、7个依赖节点校验无问题。该数量是加入handoff之前的记录。

独立审阅限定于PRD/安全/终端协议/实施依赖，发现3个协议问题和1个任务依赖问题，已修订并复核关闭。其它文档由主代理做一致性与结构检查；不要宣称独立审阅覆盖了整个实现。

未运行应用lint/typecheck/业务测试/E2E/性能压测/迁移/原生桌面验证，因为只写了规划文档。未启动/停止服务、未控制真实终端、未发送外部消息、未提交或发布代码。

后续验证按test-spec.md选择权威层。`make check`默认classic auth的隔离环境不能替代password模式验收；password模式另建task-owned环境，Web用production build/start。默认测试不得调用用户安装的真实agent或消耗模型配额。

## 7. Git与文件保全

保存时tracked工作树与staged diff均为空。本任务的父目录及7个子目录均为**未跟踪、未提交**，位于当前工作区，不会自动出现在新worktree中。

如新会话在同一目录工作，先确认8个目录仍完整；如建立隔离worktree，先将这8个任务目录完整带入并验证JSONL引用。可以在合适的提交步骤只保存本任务文档，不能直接`git add .`夹带其他任务。不要在当前handoff阶段擅自提交。

另有未跟踪目录`.trellis/tasks/10-01-settings-integration-catalog/`，不属于本任务；不得覆盖、清理或夹带提交。先前会话中的sidebar改动目前已由其它工作形成HEAD导航提交，不要回退它。

新的会话task指针可能为空或属于别的任务；读取实际状态，不能把旧会话指针当全局唯一状态，也不要修改无关任务的运行状态。

## 8. 新会话第一轮行动

1. 检查Git、任务文件和当前Trellis指针；保全未提交规划，读取上述入口和S01上下文，不重做整套菜单讨论。
2. 在当前源码基线上核对S01涉及的JWT签发、密码版本锁、CLI登录/PAT续期和路由；HEAD已变化时只复核有关差异。
3. 若用户新消息已明确按方案实施，则将S01置为活动实施任务，元数据如实记录新授权；父任务继续聚合追踪，不把全部子任务一起标in_progress。
4. 首先实现并验证S01的权限/凭据边界、组织/审计/幂等基础，再接后台shell和无workspace登录；不得先上线按钮后补权限。
5. 对每个切片更新真实verification记录，按依赖推进，保留未测项；遇到新事实修正文档，不将建议稿当作已实现事实。

Trellis命令（最后一条仅在进入实施已获授权且准备完成后执行）：

```bash
python3 .trellis/scripts/task.py current --source
python3 .trellis/scripts/task.py validate .trellis/tasks/10-01-platform-admin-console
python3 .trellis/scripts/task.py validate .trellis/tasks/10-01-platform-admin-access
python3 .trellis/scripts/task.py start .trellis/tasks/10-01-platform-admin-access
```

不要创建同名重复父任务，不要重新填模板覆盖已完成设计。不要把本文件的拟定实施步骤报告成已经完成。
