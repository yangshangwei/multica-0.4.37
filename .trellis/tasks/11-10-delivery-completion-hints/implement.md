# 实施与验收计划

1. [x] 核对业务HEAD、归档与来源源码，保护1052个主工作区路径；创建独立worktree及子任务。
2. [x] 主会话形成PRD/design/implement与清单；用户已有实施授权，无需再次问流程许可。skip未来决定不改变本包边界。
3. [x] trellis-check规划复核；独立schema readiness及checked_at语义已收敛。
4. [x] A trellis-implement所有权：delivery_completion_hint.go、delivery_completion_hint_test.go、router.go中该route、creating-agents SKILL及source-map。必要修改旧helper须先向主会话报告。禁止改B文件。
5. [x] B trellis-implement所有权：server/cmd/server/delivery_completion_hint_integration_test.go。真实认证/受控制品/持久事实、旧完成兼容，复用现有测试fixture。禁止改A文件。
6. [x] 主会话独占环境创建/迁移/最终测试，专属容器、每包每轮新库；所有命令独立run ID及原始失败保留，不借用历史环境。编译/测试以私有runner记录。
7. [x] trellis-check全范围源码、测试、授权、真实日志及manifest独立复核；机械问题可修，设计问题返主会话。
8. [x] 最终冻结后handler/router普通及race，旧policy/evaluation/evidence/artifact/approval及status回归，build/vet/sqlc/format/builtin contracts；明确具名和顶层计数，不合并普通/race。
9. [ ] 主会话更新spec、计划、父子任务、交接日志；Lore限定范围提交/归档，保护主目录安全本地集成；专属PG零连接后停止、DB/日志保留。

所有子代理知道并行共享代码，不能覆盖其他owner、不递归spawn、不commit、不调用供应商。独立协作和全部完成强制门禁不在本包实现范围。没有前端修改不运行或冒称前端/浏览器验收。父M1与六业务保持in_progress。
