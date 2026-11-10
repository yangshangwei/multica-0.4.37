# 执行顺序

- [x] 核对基线、归档及当前实现；独立worktree；建立子任务。
- [x] 主会话复核研究与设计，启动任务。
- [x] Trellis implement A：产品handler及SQL必要锁/生成、路由、内置技能合同；handler DB回归。不得改router新测试文件。
- [x] Trellis implement B：真实认证router验收，仅新delivery_evidence_integration_test.go，复用已有真实打包fixture；与A协商请求合同。
- [x] 主会话分配每包专属数据库，原始日志编号保存，禁止真实provider；最终冻结源码后普通/race重验、build/vet/gofmt/sqlc。
- [x] Trellis check：独立审查实际锁/身份/版本/回执与验证证据，发现阻塞交回owner。
- [ ] 主会话更新spec/计划/父任务状态，按Lore本地提交；不push/deploy。归档本增量但保持父路线开放。

文件所有权：A拥有server/internal/handler/delivery_evidence*.go、必要server/pkg/db/queries与generated、router.go、新API相关内置skill文档。B仅server/cmd/server/delivery_evidence_integration_test.go。主会话拥有任务/spec/计划/验证脚本和git。所有agent不覆盖其他人修改、不递归spawn。
