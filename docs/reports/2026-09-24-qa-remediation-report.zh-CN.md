# 最近功能修复与端到端验收

状态：修复完成，完整检查与全部专用端到端夹具通过。未部署、未推送远端。

修复分支：`fix/qa-remediation-20260924`；基线：`a806e8c4232513ee49bf2dad92fb5dd0f5971778`。
实现位于隔离工作树 `/private/tmp/multica-qa-20260924-a806e8c`，原工作区的用户修改未纳入。

## 已实施修复

| 问题 | 修改与结果 |
| --- | --- |
| F1 人工来源丢失 | 普通创建与生命周期交接共用可信 acting-task 来源，保留人工来源、责任人和委派任务；拒绝伪造、终态和不一致身份。 |
| F2 失败后残留任务 | child、source/child metadata、审计评论、队列写入采用同一事务；提交成功后才发布事件与唤醒。 |
| F3 证据半写与超限 | 以 PostgreSQL 实际 JSONB 大小验证整对象 8 KiB，保留用户键，最多20条历史，只裁掉最旧历史；仍超限则400且零写入。 |
| F4 交接备注丢失 | Agent/squad、新建/复用均保留备注并返回真实队列ID；兼容任务复用，不兼容返回409并回滚。 |
| F5 错误评测放行 | correctness失败/hold不能pass；未知结果保持unknown；保留其他类别已有的安全停止合同。 |
| F6 技能分类与中文搜索 | 补齐两个官方技能登记，使15个内置技能完整分类；修正中文默认描述，保护用户定制副本。 |
| F7 Observer指令冲突 | 改为评论中提出补证/修复建议，由有权限负责人建任务；更新模板/技能版本，保留Observer API拒绝。 |
| 复核新增：并发选中旧child | 锁定后重新检查隐式重复任务的标题、项目、非终态条件；显式ID合同保留。 |
| 复核新增：复用旧应用授权 | 比较稳定的应用能力集合；权限变化409且零写入，会话URL轮换仍可幂等复用。 |
| 测试入口 | 独立API/Go数据库；正式Web构建；PID、提交、源文件、配置和BUILD_ID校验；只归一化Next生成的类型导入行；构建互斥；正确退出与清理；任务专用认证和跨域配置。 |
| 测试同步与资源 | 设置导航等待精确URL及激活tab；桌面断言跟随现有文案；非虚拟表格不注册虚拟滚动计时器。 |
| Agent测试时序 | 修正90ms空闲预算与50ms进程调度间隔过近的夹具；要求所有进度消息到达且持续跨越初始窗口，移除续期的负向探针必须失败。 |

原角色模板和onboarding超时在正式Web基线下通过，因此未扩大超时或修改这些业务路径。
原4500字符证据探针的“必定201”假设不符合整对象8KiB限制，正式回归分别验证能容纳时完整写入与超限时零写入。

## 验证证据

最终完整入口返回0，并完成下列独立专项验证。完整Web主套件来自一次运行，未将失败重跑的并集合并为通过：

| 验证范围 | 结果 |
| --- | --- |
| 完整 `scripts/check.sh` | 退出0：lint、typecheck、UI导出、TS、三套shell回归、隔离Go race、vet、正式Web构建与Playwright全部通过。 |
| 正式Web主套件 | 一次运行95通过、4专用夹具跳过、0失败、0 flaky、零重试，158.2秒；4项随后全部单独执行通过。 |
| 原失败Web用例的正式构建基线 | 11/11通过，24.6秒；角色创建/onboarding保留原断言。 |
| TS单测 | 702文件，8,379通过，无Vitest未处理异常。 |
| 静态检查 | 15/15任务通过，UI导出检查通过；既有非阻断lint警告另保留原日志。 |
| 生命周期专项race | 122顶层+91子测试通过，零跳过；包含事务故障/并发/来源/容量/权限。 |
| 最终handler/service/cmd/server三个完整包race | 2,909顶层通过，66跳过；含子测试4,938通过。 |
| 原Redis门控补测 | 精确64项全部通过，零跳过；专属无持久化Redis已清理。 |
| 完整Agent包race | 1,168顶层+606子测试通过；4个既有平台/权限条件跳过。 |
| Electron阅读器/设置 | 2/2通过，8.2秒；真实预加载和生产renderer，native服务隔离。 |
| Web热发布 | 1/1通过，65.7秒；60,086ms观察到新版本，同一页面/API进程，fresh client和失败恢复通过，feed已恢复。 |
| 真实设备认证 | 1/1通过，2.3秒；真实`/auth/device`、独立用户、首次引导、NoAccess页面及跨工作区API404。 |
| Electron热发布 | 1/1通过；60,090ms观察到新版本，原页面和API进程不变，fresh client/无效JSON/网络恢复通过，feed逐字节恢复。 |

首次完整入口停在Agent夹具90ms空闲超时，修复后完整Agent包及第二次入口Go阶段通过。第二次入口构建成功，但Next生成的类型路径切换触发指纹误判；已仅归一化该自动导入行，真实源码变化仍拒绝启动，脚本回归通过。第三次完整入口全部通过；首次失败与后续证据均保留，不把失败运行改记为通过。

原始证据保存在本工作树 `.gstack/qa-reports/2026-09-24-remediation/`，后端专题证据另见 `.gstack/qa-reports/2026-09-24/`。
这些目录属于本机验收工件，不随Git分发。报告保留失败原因、最终通过结果、专用夹具结果和环境身份。主套件环境为`check-20260924152120-91306`，Web BUILD_ID为`lqBUxQeZcY018FAcrD7_P`，API/Web源码指纹均为`563d37cecd503670471f15ea0e4ed90412c3faf56db38b1f06c73356faca0561`；Node22.23.2、Go1.26.7。Git提交前的源文件哈希保存在`verified-source-manifest.json`。测试覆盖99个仓库E2E用例，另加1个Electron热发布探针。

## 修改文件与简化

- `server/internal/handler/lifecycle_handoff*.go`、`agent_access.go`、`issue.go`：统一交接事务编排和可信来源校验。
- `server/internal/service/issue.go`、`issue_task_transaction.go`、`task.go`、`lifecycle_handoff*.go`：复用创建逻辑，拆开事务写入和提交后通知；不复制第二套创建流程。
- `server/pkg/db/queries/lifecycle.sql` 及生成文件：提供事务内锁定/重读查询；无迁移或容量扩张。
- `server/internal/service/builtin_*`、`server/internal/handler/agent_template_test.go`：模板指令、版本及副本保护。
- `packages/views/skills/`、`packages/views/locales/zh-Hans/skills.json`：补登记和默认文案，复用既有分类/翻译机制。
- `packages/ui/components/ui/data-table.tsx`、对应resize测试：关闭未启用的虚拟化机制，消除无效计时器。
- `scripts/check*.sh`、`scripts/dev-env*.sh`、`scripts/test-go*.sh`、`Makefile`：复用环境登记与归属检查，去除重复的脆弱启动路径和Bash/Make二次环境解析。
- `e2e/auth-device.spec.ts`、设置/技能/本地化E2E、`server/pkg/agent/codex_test.go`：补真实认证模式及专项覆盖，消除明确的同步错误。
- `.trellis/spec/`、内置技能source-map及本报告：同步维护合同与证据。

## 边界与剩余风险

- 不自动修改旧版本已创建的孤儿任务、缺失来源/备注记录或用户定制模板。需要修历史数据时，应先列出具体记录，不能推测人工来源或改写运行中任务。
- 未新增outbox，提交后通知不保证严格一次；已验证通知失败不推翻持久化结果，轮询能发现已提交任务，不承诺未测量的恢复时延。
- 8KiB限制保留；超限证据返回400，不能宣称任意大证据都可存储。
- 验收服务、Electron窗口和专用Redis均已清理；完整检查的Go数据库已删除，check环境API数据按24小时TTL保留以便检查。
- 不增加依赖、外键或数据库迁移；未调用真实付费模型、真实安装器，也未部署或推送远端。
- handler/service/cmd完整包中的2项仍未执行：真实GitHub可选集成测试、仓库原有未实现的token lookup数据库故障占位测试。Agent包4项为平台/权限条件跳过，均不计为通过。
- 端到端覆盖Web和Electron；移动端不在本次最近功能及根检查脚本的范围内。
