# M1-S1 Backend Resume Evidence

日期：2026-11-12。范围：P1 候选证据读取、完成预览、现有授权消费路由；未增加完成 writer、授权服务、migration 或 Provider 调用。

## 实际修改

- `server/cmd/server/delivery_completion_preview_integration_test.go`：将错误 workspace selector 的预期从 403 改为 404。issue loader 在 handler 执行前用 404 `workspace_access_denied` 隐藏跨 workspace 的事项存在性；这与 `delivery_completion_hint_integration_test.go` 的既有合同一致。
- 其余服务、handler、SQL、生成代码与路由实现均为此前 M1-S1 工作区改动；本次未改动其实现。

## 当前验证

每次 DB 测试使用 runner 分配的独立本机数据库，迁移退出码 0、schema ledger 为 781；没有使用默认数据库。run JSON 与完整 JSONL 在 `research/test-environments/`，下面同时记录数据库、退出码与日志 SHA256。

| 运行 | 命令筛选 | 数据库 | 结果 | JSONL SHA256 |
| --- | --- | --- | --- | --- |
| handler-green-3 | `TestDeliveryCompletion(Preview|Candidates)|TestDeliveryEvidence` | `m1s1_handler_green_3_d1a51f08` | 76 passed / 0 failed / 0 skipped | `82f3873337d3aeba2a01c0ef2fa450dc7231a54655aaa4a942ddbeb277be6289` |
| handler-race-3 | 同上，`-race` | `m1s1_handler_race_3_597dd0bc` | 76 passed / 0 failed / 0 skipped | `625f8c9de36381b342a99d6a4491cb48d4dd0d0b257617eff4a9c9fb4defc139` |
| router-green-4 | `TestDeliveryCompletionPreview` | `m1s1_router_green_4_4432cb1f` | 4 passed / 0 failed / 0 skipped | `9e401037ae151ed1667ef363be8eefb87f1af55c1f2699199f453cad279ba477` |
| router-race-3 | 同上，`-race` | `m1s1_router_race_3_1bee0523` | 4 passed / 0 failed / 0 skipped | `8da37ca186e384739defab528fcf7e23755949577f4254763c7f0c78d65c88b4` |

其他通过检查：

- `go -C server build ./internal/handler ./internal/service ./cmd/server ./pkg/db/generated`
- `go -C server vet ./internal/handler ./internal/service ./cmd/server ./pkg/db/generated`
- `git diff --check` 针对 router 集成测试文件；`gofmt -d` 无输出。

用例覆盖候选分页与隐藏记录边界、200 条扫描上限、当前 artifact authority、严格输入、任务/项目/策略绑定、凭据期限、无业务写、拒绝分支；router 通过 legacy/password 身份执行候选读取、预览、授权、完成、receipt 恢复、重开和 workspace/human 边界。新读路由不签发授权；授权与完成仍由既有事务能力执行。

## 保留的失败及判定

- `handler-red-1` 是实现前的 TDD 红阶段：新 handler 测试因实现符号尚不存在而编译失败；该记录保留，不是产品回归或行为测试通过。
- `handler-green-1` 在 fixture 准备时报 `artifact adapter unavailable`，没有成功进入 handler 行为覆盖；归类为本地受控制品测试环境失败，记录保留。
- `handler-green-2` 为 75 passed / 1 failed。唯一失败 `TestDeliveryEvidenceProjectBinding` 在要求无争用的成功读取时收到 409 `artifact_authority_busy`。现有合同将数据库 NOWAIT lock contention 作为拒绝，不能改成通过。使用新鲜独立 DB 重跑后，`handler-green-3` 与 `handler-race-3` 均通过该用例及整个筛选矩阵；原失败不改判。它未在当前隔离重跑中复现，不能据此宣称所有并发环境下永不争用。
- `router-green-3` 为 3 passed / 1 failed。失败是本次新增测试错误地将 issue 路径上的跨 workspace selector 预期为 403；既有 issue loader 按产品合同返回 404 `workspace_access_denied`。只修正该测试预期；失败原始 JSONL 保留，后续 `router-green-4` 与 `router-race-3` 均通过。
- `handler-race-1` 与 `router-green-1` 因 private temp 路径存在 macOS `/var` 非规范 symlink ancestry 被受控制品根校验拒绝，状态为 `not_run_environment_replaced`；未把它们计为测试。更新过的 runner 使用 canonical temporary roots，后续全新分配的数据库通过。

失败资料位置：

- `test-environments/handler-red-1-tests.jsonl`
- `test-environments/handler-green-1-tests.jsonl`
- `test-environments/handler-green-2-tests.jsonl`
- `test-environments/router-green-3-tests.jsonl`
- 对应 allocation JSON 未覆盖或重写。

## 当前源码绑定

测试记录中的 `source_head` 都是 `ade2298e24c3e99aa1982f9c6ddcbb1dc3bd29d3`。HEAD 不含未提交改动；以下 SHA256 绑定本轮检查的精确工作区文件字节：

| 文件 | SHA256 |
| --- | --- |
| `server/internal/handler/delivery_completion_preview.go` | `24388feb9bd9e575939aa74fad715002bec874ea8aa899d01aafaac1f306453b` |
| `server/internal/handler/delivery_completion_preview_test.go` | `6f9333f4a32212c6bd5fb104f54aea61c960d506f3b79edd38b40fecad41d0c8` |
| `server/internal/service/delivery_completion_preview.go` | `a33096e9fedec4367b45b5152fbe92a484a5c62d5a97dc4094f406f39d348ef1` |
| `server/internal/service/delivery_completion_preview_types_test.go` | `2be315837fde09a772da563c2b7c1f8c58e27e6ecb67187ea98e82e8bc5f470d` |
| `server/internal/service/delivery_evidence.go` | `fbd43b3cbed9c3f30ffea900e667f7c9bbb0893353138798676edafd12d5ef52` |
| `server/pkg/db/queries/delivery_evidence_candidates.sql` | `a23ff4d0272d7e8f765fbd8019ad6283cec58a59c4607a113bb351e77a2e027a` |
| `server/pkg/db/generated/delivery_evidence_candidates.sql.go` | `feb70151b6e9072c9dcad12d832d3ffb7ca1bafba61f0b938976705db231cad4` |
| `server/cmd/server/delivery_completion_preview_integration_test.go` | `638ef47e772ab3389d1c2d63e81f06557bbc56a7ec31f6cd1a3b168f07392622` |
| `server/cmd/server/router.go` | `6a1ed2da327ea5dc3a435c121244471d74c6818e54ac2378a6317d3890b920c5` |

## 限制

以上证明本地 deterministic fixture 和实际 router 路径，不证明 Web/Desktop UI 接线、CLI、真实 Provider、Hosted CI、部署或发布。现有 `skip_supported=false` 保持既有验收限制。本 backend slice 仍需主任务统一的独立检查与跨端验收。
