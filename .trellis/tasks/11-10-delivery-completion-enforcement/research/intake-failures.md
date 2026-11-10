# Intake command failures

- 首次在新 worktree 读取本会话 autopilot-state.json 时路径不存在，工具报告 setup failure；状态文件属于主工作区的 .omx/state/sessions/01a1c4df-4264-78f2-9cdd-ecdc407fa93c/。已用显式存在性检查读取主工作区原文件，确认仍为 planning。此为读取路径错误，不是产品行为 red 或验收失败。没有删除历史证据或改动旧验收状态。
