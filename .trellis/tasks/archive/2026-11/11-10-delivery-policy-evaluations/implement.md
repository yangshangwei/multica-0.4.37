# 执行及所有权

- [x] 核对c3bc9abd1无后续提交、用户列出全部归档/父计划/spec/Session149，36文件冻结一致。
- [x] 新worktree codex/delivery-policy-20261110；主目录1052已有变更文件hash留存；专属PG/bootstrap与外部canonical tmp。
- [x] 收敛政策管理权限，主会话审阅PRD/design/implement并start（本轮用户明确授权实施无需再问流程许可）。
- [x] A trellis-implement：backend handler、SQL/schema/generated、migrations及cmd/migrate注册、handler DB测试；router.go新路由、内置skill/source-map。不得改B文件。
- [x] B trellis-implement：仅server/cmd/server/delivery_policy_integration_test.go，真实认证/真实打包持久事实/旧入口兼容；与A固定字段。
- [x] C trellis-implement：仅server/cmd/migrate/delivery_policy_test.go及server/internal/migrations/delivery_policy_test.go，实际迁移/保留/并发回滚及静态合同。
- [x] 主会话：最小更新workspace_delete_manifest_test.go保留审计分类（实际基线red已留存）；协调专属每包数据库，所有命令经private run.py保存独立编号原始日志及hash，冻结源后final普通/race；build/vet/sqlc/gofmt/static。
- [x] trellis-check独立读实际调用链与日志/hash，不将静态或mock当完整证据；具体问题交回owner修复，最终冻结重新核对。
- [x] 主会话更新专用spec/索引/计划/父任务/交接，Lore本地提交、归档最小包；保护主工作区安全快进集成。父M1及六业务保持in_progress。

禁止借用历史容器/数据库、真实模型、新依赖、推送/部署/外部通信。子代理不递归spawn不提交。每人不覆盖其他owner修改。普通/race各独立库，不跨轮累计覆盖。维护回滚测试仅专属fixture schema。细节见research/environment.json与API-contract（若补充）。
