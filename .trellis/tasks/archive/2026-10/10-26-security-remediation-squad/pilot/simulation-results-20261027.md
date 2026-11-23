# 安全小队脚本模拟结果

结论：**模拟方案可行，四个案例均已跑通真实产品流程并进入 `in_review`。** 执行器没有启动真实 Codex/OpenCode，也没有模型调用出口；路由决定、报告和补丁选择由脚本预设，队列、任务凭据、评论作者、成员回传、leader 回访和状态流转使用实际产品代码。

本轮按用户“不调用真实的运行时，模拟一下”的要求执行。它验证流程可行性，不证明真实模型会遵守指令、正确判断风险或独立审查；原真实模型验收与阶段二准入仍未通过。

## 四个案例

| 案例 | 实际执行 | 结果 |
| --- | --- | --- |
| `POLA-19` 供应链缺证 | leader → 安全审查员 → leader；另验证缺少 Markdown 方括号的 mention 不创建成员任务 | 3 次成功执行；保留 `hold/unknown`，补证 owner 为 `pilot-build-owner` |
| `POLA-20` 合成依赖公告 | 分诊 → 安全 → 迁移 → 实现 → QA → 代码复审，每步由 leader 派发和回访 | 14 次成功执行（包含一次恢复协调）；依赖子集 3 项测试通过 |
| `POLA-21` 工作区授权 | 安全 → 诊断 → 实现 → QA → 代码复审 | 11 次成功执行；诊断保留 3 项中 1 项预期失败，独立 QA 最终 6 项通过 |
| `POLA-22` 生产升级桌面演练 | leader 写人工/incident 交接，记录禁止动作 | 1 次成功执行；没有 Operator 任务或生产操作 |

合计 **29 次成功任务执行**，12 个成员阶段都具有真实的委派和 leader 回访关系。所有案例最终保持 `in_review`，没有由小队标记 `done`。不同任务身份代表执行职责分离，不代表不同模型完成了独立推理。

## 证据核验

- 从实际 API 回读 task、comment、timeline，并与运行日志交叉核对 `author_id`、`source_task_id`、`delegated_from_task_id`、触发评论、`precise`、workspace、`action/no_action`。不接受仅在正文中自报角色或任务 ID。
- 四个案例的所有必要成员阶段都有独立任务、本人结果评论和一次对应的 leader 返回。派发记 `action`，最终收口记 `no_action`。
- 实验室从未修改的仓库 fixture 复制。六项原始测试为四通过、两项预期断言失败；先回放依赖修复，再回放授权修复，最终 QA 六项通过。
- 最终只改变 `application.py`、`dependency.lock.json`、`workspace_reader.py`。测试、vendor、公告和供应链 fixture 的原始字节保持不变。补丁来自此前已记录的合成修复，不能描述为本轮模型生成。
- 执行容器没有真实 agent CLI；角色 UID 为 1000，不能读取 root 的开发认证配置。容器无外网路由或外部 DNS，API 网关拒绝 `/model/*`，没有模型凭据。

## 检查结果

- **24 项现有后端契约测试通过，零跳过。** 使用独立、已迁移的临时 PostgreSQL；逐一核对 JSONL 中预期测试的 run/pass 集合，并启用 ambient agent CLI 拒绝包装器。没有使用运行中应用的数据库。
- **14 项模拟工具测试通过。** 包括协议、路径、任务身份、修复回放和证据校验；独立复核还覆盖 12 个证据破坏样例，错误作者、手动派发、缺少回访、错误来源任务和错误 `no_action` 等均被拒绝。
- 独立 Trellis 复核通过，修正了证据校验器对来源任务、回访归因和模拟边界的漏检。未改产品代码；全仓前端 typecheck/E2E 不适用于本轮任务内脚本，未运行。

## 保留的失败与恢复

两次早期失败没有删除，也没有计入成功数：

1. 首次 leader 把评论文件写到 `/tmp`，实际 CLI 按文件边界拒绝。模拟器改为写入当前任务工作目录，并补充先失败后通过的回归测试；新小队触发完成了三段往返。
2. 首次迁移报告 POST 在 30 秒后超时。回读确认未保存成员结果；网关日志同时记录了不可达的 IPv6 上游。随后限定 IPv4 解析，并给外层进程保留 CLI 自身的超时诊断余量，由新小队触发继续缺失阶段。具体延迟根因未被独立证明，不能把这次恢复写成已定位的产品缺陷。

这些记录可用于验证恢复过程，不能靠忽略失败制造“全程无错误”的结论。此前真实 Codex CLI 的一次 `READY` 和 SDK 的两次 401 拒绝仍是独立历史，不参与模拟通过判定。

## 可复现材料

- [模拟工具与运行说明](simulation/README.md)
- [独立复核](simulation-quality-check-20261027.md)
- [证据交叉校验结果](evidence/scripted-simulation-20261027/validation.json)
- [原始动作日志](evidence/scripted-simulation-20261027/simulation-actions.jsonl)
- [最终文件摘要](evidence/scripted-simulation-20261027/final-source-hashes.json)
- [最终合成补丁](evidence/scripted-simulation-20261027/final-lab.patch)
- [临时环境清理](evidence/scripted-simulation-20261027/cleanup.json)

历史角色的共享绑定没有改动。此前创建的八个私有试点副本用于本次模拟，保留真实模板/自定义 coordinator 来源差异；模拟项目、小队、问题与执行记录留在开发工作区。临时执行容器、测试数据库、网络和凭据清理完成后，镜像与任务内材料仍可重建演练。
