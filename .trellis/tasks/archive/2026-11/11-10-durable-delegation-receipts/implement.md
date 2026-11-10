# 执行与所有权

- [x] 核对最新HEAD、归档/父计划/源码，用户已授权规划与实施；相邻独立worktree，保存主工作区dirty哈希。
- [x] PRD/design已收敛；最小包为可选幂等普通评论创建与恢复，不扩到编辑/quick-action。
- [x] A trellis-implement：server/internal/handler/comment*.go、必要service、SQL/schema/generated/migrations、router.go及migration注册/测试。拥有产品后端和handler测试，不编辑新router集成测试、CLI/brief/skill文件。
- [x] B trellis-implement：server/cmd/server/durable_delegation_integration_test.go；真实JWT/PAT/task身份与重启、撤权/并发、队列身份。仅此测试文件。
- [x] C trellis-implement：server/cmd/multica相关comment CLI及测试、server/internal/cli/client.go相邻测试（单请求显式header，禁止transport自动重放）、squad_briefing.go/protocol测试、runtime mention段及测试、两项builtin skill/source maps。不得改A/B文件。
- [x] 主会话：分配专属PG容器/每包独立数据库，记录每轮原始失败，冻结最终源码后普通/race验收、build/vet/sqlc/guard检查；独立trellis-check。
- [x] 主会话：spec、任务/父计划/交接、Lore本地提交，保护主工作区本地集成；不push/deploy、不启动真实模型。仅本包归档，父任务保持in_progress。

DB所有权：A使用receipts_handler_*；B使用receipts_router_*；主会话final/review新库。容器multica-durable-receipts-20261110-pg独占，127.0.0.1:61771，用户receipts、本地专用密码receipts-local-only。严禁连接5432或历史库。外部TMPDIR/HOME由主会话分配；全部日志保留。agent不递归spawn、不提交、不覆盖其他owner。
