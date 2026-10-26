# 阶段一试点结果

状态：`blocked_pending_model_config`（2026-10-27）。任务保持 `in_progress`，阶段二未启动。

## 本轮结果

已按用户上轮选择恢复原 daemon/runtime，在隔离容器中验证注册、heartbeat、任务领取和真实 OpenCode 启动。8 个既有角色仍使用原 UUID、模板来源、自治等级、私有 Access 和 skills，没有重绑或覆盖 workspace-owned 内容。

首个真实演练已从“runtime 离线”推进到模型调用，但匿名 OpenCode 返回 `Free usage limit reached`，本机 `opencode auth list` 显示 `0 credentials`。备用匿名模型传输探测也失败。没有取得成功的角色评论、分流或独立 QA，不能宣称试点通过。

模型阻塞明确后未继续派发其他案例。检查原 daemon 的四个工作空间 namespace 均无 pending/active task 后，停止本轮容器并移除临时开发凭据副本；宿主已认证 profile 保留原状。只清理了本轮自动发现时新建且没有绑定 agent 的猎户座 runtime，保留所有历史 runtime 和角色。运行时恢复已验证，最终回读容器为 `exited`、原 runtime 为 `offline`，见 `evidence/cleanup-20261027.json` 和 `evidence/runtime-after-stop-20261027.json`。

## 环境和来源

| 项目 | 实际值 |
|---|---|
| 工作空间 | 北辰 `d6eaf4b9-07bc-4aa4-b3ee-5a4bc733949c` |
| 小队 | 安全漏洞修复小队（试运行）`ac1a40c2-a655-4453-b86d-745796a454bc` |
| 项目 | 安全修复试点 2026-10-27 `e9f1b898-709a-4aee-9b75-5d72cf55e950` |
| 原 daemon | `019fef71-c14f-757e-9ad2-a9750d306456` |
| 原 OpenCode runtime | `39443bc2-1bf7-4515-8bdd-4684a41d64c9`，本轮曾回读 `online`，owner 不变 |
| 容器 | `multica-security-pilot-20261027`，无 host HOME、Docker socket 或卷挂载；本轮结束时已停止 |
| 身份 | daemon 为 root；provider 为 `node` UID 1000，实测不能读取 root 的开发 CLI profile |
| daemon / API commit | daemon 从 `61ff581d2` 构建；实际 API `/health` 为 `564e3c51c`，不把 API 冒充本轮 checkout |
| CLI / 模型 | OpenCode `1.18.21`；daemon fallback `opencode/big-pickle`；agent.model 保持原空值 |
| 实验室 | 容器内 `/work/lab`，分支 `pilot`，基线 `3c00fabd4415a6497a90fda491fd49129d758183` |
| 目录绑定 | 普通项目资源 `local_directory` + `in_place`，绑定原 daemon；leader 使用独立协调目录 |
| 内容版本 | `evidence/roles-20261027.json` 保留模板版本、指令摘要；`skill-content-versions-20261027.json` 保留 6 个去重 skill 摘要 |

容器提供了独立文件系统；其默认 bridge 出站访问仍未限制，网络最小权限条目保持未完成。角色共用容器内的 UID，不把角色指令或自治等级当作文件系统沙箱。没有注入模型账号凭据或生产凭据。

## 案例与真实执行

| 案例 | Issue | 当前结果 | 下一步 owner |
|---|---|---|---|
| 组件/依赖 | `POLA-4` | 已建未分配 backlog；合成失败基线已确认，未完成角色路由 | 操作者恢复模型后分配小队；安全/迁移审查先行 |
| 程序代码 | `POLA-5` | 已建未分配 backlog；授权失败基线已确认，未完成角色诊断/修复 | 操作者恢复模型后分配小队；独立诊断、实现、QA |
| 其他安全风险 | `POLA-6` | 已建未分配 backlog；供应链材料有意缺证，保持 `hold/unknown` | 安全审查及 `pilot-build-owner` 补证 |
| 生产升级桌面演练 | `POLA-3` | 真实领取/启动成功，模型拒绝；无成功交接评论，仍为 `todo` | 操作者配置可用模型后重跑同一 issue |

`POLA-3` 的三个任务均为终态：

1. `01a1790f-907a-7e0d-9510-c3fc31880671`：启动进程失败。root 创建的 `0700` task 临时目录不允许 provider UID 1000 写入；容器包装器已修复所有权并通过专门回归检查，root profile 仍不可读。
2. `01a17914-2ec8-7c0a-915b-2b4de2013179`：修复后的显式 rerun 到达模型服务。provider 日志报告匿名额度不足，3 分钟上限后失败。
3. `01a17916-ff9c-793f-9a39-e9b0d4b9c847`：上项的一次自动 retry，同样额度不足并在 3 分钟上限后失败。

没有成功的角色工具调用或安全交付。系统错误评论不是角色审查意见。备用 `opencode/hy3-free` 仅做无工具传输检查，返回 `UnknownError` / `err_42a1de2a`，不是另一个安全试点结果。

真实任务、评论、activity 见 `evidence/production-escalation-*.json`；模型日志的脱敏结论见 `evidence/model-errors-20261027.json`。不删除失败记录，不用合成评论补造路由证据。

## 验证

- 合成实验室：宿主准备与独立 `trellis-check` 均确认结构有效、8 个 Python 源文件可解析、13 个基线文件摘要一致。
- 容器内结构命令 `python3 -B validate_fixture.py` 退出 `0`。
- 容器内 `python3 -B -m unittest -v test_dependency test_authorization`：6 项，4 通过，2 个预期断言失败（标签长度限制、跨工作区拒绝且无读取审计），退出 `1`；尚无修复后全绿证据。
- 容器权限回归：root 创建的 task 临时目录对 node 从不可写变为可写，daemon 私有 profile 仍不可读，退出 `0`；宿主 HOME、Docker socket 不存在于容器。
- `cd server && go test ./internal/service -run 'Test(SquadTemplates|AgentRoleTemplates)' -count=1`：通过。
- `task.py validate`、运行时包装器 shell 语法和任务材料静态检查通过。此次没有修改产品代码，不运行全仓 TS lint/typecheck/E2E，也不宣称这些检查通过。

## 上轮证据及纠正

上轮已成功创建小队、更新指令并添加 7 位成员，回读 `member_count=8`。跨 runtime 的普通 agent copy 虽保留指令/skills，却缺失 `autonomy_level` 和 `template_key`；这些副本已在上轮移除并归档，原成员关系已恢复。用户随后明确选择恢复旧 runtime，本轮沿用此选择。

上轮角色/小队 service 窄测、模板列表 handler 测试通过；两项创建 handler 测试曾报 `runtime not found in this workspace`。这些 DB-backed 测试使用自己的测试工作空间与 runtime fixture，不能与真实桌面 daemon 是否在线互相印证。本轮未重跑这两项，也没有修改产品测试来掩盖失败。

## 续跑条件和阶段二门槛

1. 指定可用于本试点的模型配置；无需在任务评论、仓库或命令参数传递 API key。本轮已异步询问配置来源，尚未获得答案。
2. 恢复隔离容器及其私有开发认证配置，核对所有原 namespace 的队列；验证 API/模型目的地范围，重跑 `POLA-3` 并回读真实交接输出。
3. 依次分配 `POLA-4`、`POLA-5`、`POLA-6`，从评论/activity/任务结果验证分诊、兼容性审查、诊断、实现与独立 QA。补丁只写隔离实验室，保留仓库内失败基线。
4. 汇总实际角色冲突、误分流、重复交接和缺证 owner。只有四例证据满足阶段一验收后，才另行启动阶段二实现；当前不新增内置模板、角色或产品代码。

## 本轮检查点

试点材料与模板来源规范已保存于本任务目录和对应 code-spec。独立 `trellis-check` 核对了真实任务、系统评论、最终清理状态及模板创建源码，修正了“默认私有”与用户凭据/工作空间 daemon token 的发现范围表述。最终材料检查覆盖 43 个任务文件、16 个 JSON 文档、8 个 Python AST、JSONL 引用、shell 语法、源码摘要、空白和凭据字面量，均通过；这组准备检查不改变阶段一受阻或阶段二未启动的结论。
