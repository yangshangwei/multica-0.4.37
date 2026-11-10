# Intake command failures

- 首次在新 worktree 读取本会话 autopilot-state.json 时路径不存在，工具报告 setup failure；状态文件属于主工作区的 .omx/state/sessions/01a1c4df-4264-78f2-9cdd-ecdc407fa93c/。已用显式存在性检查读取主工作区原文件，确认仍为 planning。此为读取路径错误，不是产品行为 red 或验收失败。没有删除历史证据或改动旧验收状态。

- 规划提交前 staged diff 检查发现 prd.md 末尾多一空行（`new blank line at EOF`）。同一 shell 单元后续 commit 未因该非零状态停止，产生 cdadbe4e4；随后复现并保留 research/staged-diff-failure.json，删除多余 EOF 空行后重新检查。此为文档格式失败，不是产品行为 red；后续相关提交命令逐步检查返回码。原规划提交保留，不改写历史。

- 2026-11-11 当前会话准备集成时，四份本轮研究文件在业务基线1ce4e8122尚未跟踪，读取HEAD blob返回不存在；已将其按新增且main/incoming字节一致处理，未向主目录写入。父task.json文本3-way仅next_action出现双方修改，已用字段级合并保留原父字段/children并采用本任务最新验收后的next_action；纯预计算冲突不是产品失败。旧历史失败不改写。
