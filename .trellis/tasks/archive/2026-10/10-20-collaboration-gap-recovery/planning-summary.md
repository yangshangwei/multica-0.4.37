# 10-20-collaboration-gap-recovery 规划终审摘要

日期：2026-10-21。当前任务保持 `planning`，本文件用于最新实施范围审阅；产品实现和父任务完整验收尚未完成。

## 要解决的问题

先消除调度与停止证明恢复中“固定失败前缀占满每轮名额”的阻塞，再补齐原计划遗漏的私人资源提供、`@all` 和取消指令历史，接续运行端文件/provider、有限代管和完整验收。需求继续使用父任务 R01–R58、M01–M11、AC01–AC97。

## 实施顺序

| 顺序 | 交付内容 | 主要验收 |
| --- | --- | --- |
| S1 | 服务端 waiting/wake/candidate 有界公平扫描；daemon 候选和停止证明恢复轮转 | 17+ 候选失败前缀、单对象错误隔离、9+ proof 前缀失败/锁忙、重启与原证明重放 |
| S2 | 参与者明确提供自己的私人资源，creator/handler 采纳，各实际 owner 独立授权 | 三账号、两个独立空间，组织者无执行空间成员权的完整路径 |
| S3 | 人类 `@all` 和显式 AI 提及；可跨端查看的不可变取消指令历史 | 通知差集/去重、取消原文和精确来源、取消与 Stop 的竞争边界 |
| S4 | 明确附件或固定 Git commit 准备、环境账号及 provider/OS 能力准入 | 每个支持组合的真实隔离、完整收尾与结果证据 |
| S5 | disabled creator 的有限管理办理，双禁用重开时由有效参与者接手 | 独立 password/admin 环境、事务原子性、角色和候选竞争、双端表单 |
| S6 | 原 97 项完整验收和平台接线 | 每项唯一主验证层、具体测试/命令/环境/结果及明确未运行范围 |

S1 的 service/SQL 与 daemon/proof 可分工；S2 与 S3 可并行。S4/S5 等待控制、授权和生命周期基础稳定。生成 SQL、共用 schema、路由/API barrel、locale 及构建环境由协调者统一整合。

## 终审修正

- F1：将序号不可变限定到已消费快照、已取消/已停止输入及历史 membership；原 R26/M06 Switch 仍能把尚未入组的 waiting 输入移到新队长通道末尾。增加 Switch 后取消的准确身份及历史保留回归。
- F2：先选择当前优先等级，再在同等级内公平轮转。给低等级机会前，完成完整作用域的 reservation 优先检查和有界回绕；增加游标已越过优先者、页外优先者、新 reservation、多 poller 和重启回归。

## 边界与已知限制

沿用同一 Server、Web/Desktop 与配套 CLI/daemon 的范围，不增加依赖。资源提供不等于授权，组织者不能代签；`@all` 不邀请或授权 AI；取消 waiting 不升级成 Stop。原 nonce、proof、receipt 和未知进程容量屏障保留，证明到达不会自动恢复队列。

默认创建/执行发布开关保持关闭。真实 provider 账户 smoke 和发布各需单独授权；不能满足完整隔离的 provider/OS 组合保持明确 unsupported。父任务在全部合同具备实际证据之前保持进行中。

## 规划验证与工作区基线

97 个 AC 的编号、原文、来源行及唯一主验证归属已对照父任务。`implement.jsonl` 15 项、`check.jsonl` 16 项通过 Trellis 验证；实际 native context 能完整装载全部引用和三个规划文件，没有截断。独立终审与最终校验结论记录在 [planning-verification.json](planning-verification.json)。

本次续接时的 40 个 status 条目（57 个展开文件）已保存仓库外文件副本、SHA-256、HEAD 和 staged/unstaged binary diff，位置为 `/tmp/multica-gap-recovery-20261021-h6phxwo9/`。本轮规划未更改这些既有产品改动。2026-10-20 的 24 项基础测试为历史证据；本次未运行产品测试，失败前缀回归仍须在 S1 实现前新增并确认失败。

## 实施入口

用户针对本摘要确认实施后，复核基线并执行 `python3 .trellis/scripts/task.py start .trellis/tasks/10-20-collaboration-gap-recovery`，先进入 S1。实现由 `trellis-implement`、独立检查由 `trellis-check` 承担，按切片保留验证证据和可逆提交。
