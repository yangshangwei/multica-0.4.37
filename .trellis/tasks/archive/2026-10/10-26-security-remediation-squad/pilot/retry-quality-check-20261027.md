# 重试独立质量检查（2026-10-27）

结论：真实模型执行、合成补丁和独立成员审查已有证据；阶段一仍未通过，阶段二未启动。`in_review` 是交付状态，不是安全验收结论。本次只读取仓库证据与相关源码，没有调用 API、启动容器、触发模型或重复运行已验证的契约测试。

## 已核对的证据

- `real-pilot-summary-20261027.json` 中四个父案例、八个成员子任务均为 `in_review`。八个子任务各有对应 assignee 的成功 task、同一 agent 作者和匹配的 `source_task_id`；其来源为 `direct_human`，证明操作者手动派发的独立审查，不能替代 leader 自主路由。
- `POLA-4` 的三份标称成员报告实际来自同一个 leader task，不能计入独立审查；`POLA-4`、`POLA-5` 初始实现均为 leader 所做。`POLA-13` 的失败证书任务结果为空，后续成功审查来自另一真实 task。旧失败和纠正记录均保留。
- `lab-diff-20261027.patch` 的 SHA256 与 `portable-replay-20261027.json` 一致，范围只有 `application.py`、`dependency.lock.json`、`workspace_reader.py`。保存的回放输出为原始六项四通过、两个预期断言失败，应用补丁后六项通过；测试、vendor 和供应链 fixture 未变。本次核对摘要、范围和输出，没有重跑回放。
- 最后路由复测的有效 Markdown 评论由真实 leader 发布，member task 的 `trigger_comment_id`、`delegated_from_task_id`、`source=delegation` 和作者身份相符，且 parent 指向当前触发评论。leader 完成与成员启动均记录在 12:07:54；证据支持单并发槽位交接，时间精度不支持更细的先后判断。
- 最后 member task 因匿名模型额度限制被取消，`result=null`，无成员返回与 leader 收尾证据。leader 在有效派发时记录 `no_action`，与源码要求的 `action` 不符；此前缺少方括号的评论没有产生该成员任务。
- 本轮 briefing 投影摘要报告新 leader 回合包含 Operating Protocol、精确链接和 v2 标记。它不是早期回合的历史快照，不能据此认定早期失败源于 briefing 丢失或模型忽视已收到内容。
- 最终清理快照记录容器 `exited`、无卷挂载、停止前原 namespace 的 pending/active 为 0、临时 daemon 认证副本已删除；仅清理新建无绑定 runtime，历史绑定保留。本检查未重新查询运行状态。

## 仍开放的门槛

1. 补齐真实成员返回、leader 收尾及正确的 activity outcome；后来的 `POLA-14` 历史结果不能代替被取消的委派结果。
2. 演练 `diagnostician → implementer → QA` 各自身份的实际交接；保留原补丁和误分流记录。
3. 更正报告中的基线摘要、测试子集数量、版本锁定与权限含义、分诊枚举误述。真实独立身份不保证报告内容准确。
4. 落实并验证 API/模型网络目的地范围。默认 bridge 出站仍未受限制；供应链缺证继续保持 `hold/unknown`，由 `pilot-build-owner` 补证。

## 机械核对

- 28 个 JSON 文件可解析、无重复键；18 条 JSONL 引用存在，`task.py validate` 通过。
- 仓库失败基线的 13 个文件摘要和完整清单不变，8 个 Python 文件 AST 有效。
- runtime 包装器通过 `sh -n`，运行指引与研究的 4 段 bash 通过语法检查；文件换行、尾随空白及凭据模式扫描通过。补丁中的标准上下文空格按 diff 格式保留。
- 当前源码的 Markdown mention、thread-parent 普通 agent 回合、显式 squad leader 回合和 activity outcome 契约与路由发现一致。
- TS/Go 全仓 lint、typecheck、E2E：此次只改任务记录，不适用，未运行。实际模型和回放结果来自已存证据，不冒充本检查重新执行的验证。

发现并交主会话修正了一处来源记录问题：本地 `pilot-instructions-v2.md` 比实际持久化正文多一个末尾换行，导致文件摘要与原 API 摘要不符。主会话通过只读 GET 核对原正文，移除本地多余换行，保留原已应用摘要和远端内容，并补充字节来源说明。独立复核后，2702 字节的本地文件、`source_file_sha256` 和原 `instructions_sha256` 均为 `230e81da8c48030d4a2cfef8db3e918aef95bcc62b1892fc429c3fa47c4dcb15`；没有通过改写历史摘要掩盖差异。当前没有遗留机械检查失败。
