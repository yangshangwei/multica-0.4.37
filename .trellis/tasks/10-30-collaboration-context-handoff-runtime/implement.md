# 可靠交接与召回实施清单

状态：尚未启动；承接首轮交付闭环和已确认父计划，不创建重复任务。

## 前置门槛

- [ ] 首轮 migration/sqlc/Go/TS、生产 Web/Desktop handoff 证据完整；E0 通过，E1–E6 有逐次结果或明确环境阻塞。读取最新 closure 报告，不沿用已失效的 502 判断。
- [ ] 根据实验观察重新核对 R1–R6；只调整机制和验证，不改写原预登记预测。
- [ ] 校验 implement/check JSONL；记录新的实现基线，在独立 worktree 激活本子任务。

## 顺序与验收

1. [ ] H1：纯 projection/size helper → 真实 DB provenance/authz → handler/schema malformed 测试。交接用 handoff/event ID；reader 用潜在接收语义。实现只读有界包，满足 H-AC1/H-AC6。
2. [ ] H2：先写 fake Provider 首次输入和重试漂移失败测试，再实现协商、冻结快照和直接派活；同批维护内部事件 records 过滤与 CLI/内置 skills。满足 H-AC2/H-AC7。
3. [ ] H3：先覆盖三来源竞态与 claim 前后分配，再增加必要关系/索引及所有 wake producer。保留现有 group order/proof 测试，满足 H-AC3。
4. [ ] H4：先验证序号幂等、冲突/乱序、撤权竞争、Stop 不受上传背压影响及 final/checkpoint 去重，再接入流式 delta。满足 H-AC4。
5. [ ] H5：先覆盖 201+、同时间戳、append/delete、跨 issue 游标、Unicode/最坏 JSON 转义和单条超预算，再开放 runtime/CLI 分页；旧 context 数组明确截断。满足 H-AC5/H-AC6。
6. [ ] H6：core malformed/cache denial → views 来源/缺失交互 → 生产 Web 与原生 Electron 接收方链路。满足 H-AC7，更新 docs 与 source-map。

每一增量独立检查、独立小提交；不为并行开发提前加入未使用抽象。不进入知识候选写入。

## 验证命令和证据

- Go：在 server 执行相关 `go test ./internal/service ./internal/handler ./internal/daemon/collaborationrun ./cmd/multica ./pkg/protocol -run <增量测试> -count=1 -v`；DB 场景必须证明实际 RUN/PASS，不能接受不可达导致的跳过。
- 静态与构建：`go vet ./...`、`go build ./...`；新 SQL 执行 `make sqlc` 并确认 generated 无未解释漂移，DDL 执行 fresh/upgrade/reapply 和可用回退矩阵。
- 前端：`pnpm lint`、`pnpm typecheck`、定向 Vitest。只在新增共享行为确实落地后扩大组件测试，不跨层重复纯矩阵。
- E2E：复用 collaboration handoff/records/command-history fixtures，使用生产 Web build；原生 Electron 的执行路径和退出码单列。
- 每份证据保留 source SHA、dirty source 范围/hash、命令、环境/数据库、测试名/数量、退出码和剩余限制。模拟与真实模型证据分开。

## 回退点

- H1/H2：关闭新能力协商可以停止新快照写入，但旧/新历史仍可读。
- H3：迁移失败停止后续启动，保留原始原因日志；不删除已分配来源或重置 ordinal。
- H4：停止 checkpoint 发送不妨碍终止证明，coverage 标记保留；不能靠丢弃已接收历史回退。
- H5/H6：保留旧字段/记录项，未知元数据只降级显示，不授予业务权。
