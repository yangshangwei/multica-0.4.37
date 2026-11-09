# 执行计划

状态：执行完成。四个最终场景全部通过，首次测试断言失败及修正后重跑分别保留；报告见 artifacts/qa/collaboration-multi-account-20261109/report.md。

## 1. 计划与源码基线

- [x] 读取协作文档、隔离规范和现有测试。
- [x] 记录本次范围、三方拓扑、主线及补充矩阵。
- [x] 最终计划获审阅批准后执行 `python3 .trellis/scripts/task.py start .trellis/tasks/11-09-collaboration-multi-account-qa`。
- [x] 保存 HEAD、dirty 路径及相关源码哈希；不整理或提交其他既有修改。

## 2. 环境与测试准备

由主会话分配隔离环境；实现子代理只负责新的闭环 spec/fixture，不创建或销毁数据库。

- [x] 为本任务生成私有 allocation/ENV_FILE/registry，验证候选数据库不存在或已明确归本任务，再创建并迁移。
- [x] 只读核对 PostgreSQL 连接余量，限制任务 API 池，串行运行共享门测试。
- [x] 编译明确的模拟工具，记录哈希：

```sh
go -C server test -c -o <task-bin>/host-daemon ./internal/daemon
go -C server build -o <task-bin>/multica ./cmd/multica
go -C server build -o <task-bin>/host-provider ../e2e/fixtures/collaboration-host-provider.go
go -C server build -o <task-bin>/memory-provider ../e2e/fixtures/collaboration-memory-provider.go
```

- [x] 以 task-owned password API、production Web 和构建后的 renderer 启动；核对实际 DB/API/WS/端口/PID/源码身份，不直接重用旧任务环境。
- [x] 提供显式变量：`DATABASE_URL`、`COLLABORATION_E2E_API_URL`、`NEXT_PUBLIC_API_URL`、`PLAYWRIGHT_BASE_URL`、`MULTICA_DEPLOYMENT_ID`、`COLLABORATION_ELECTRON_RENDERER_URL`、`COLLABORATION_HOST_DAEMON`、`COLLABORATION_HOST_PROVIDER`、`COLLABORATION_HOST_CLI`、`E2E_PASSWORD_AUTH=1`。
- [x] 使用本任务 Playwright 配置或显式加载器避免默认 dotenv 回退；设置单 worker、零自动重试、独立 output/report 路径。

## 3. 主线与补充执行

- [x] 新增 `e2e/collaboration-multi-account-closure.spec.ts`，必要 fixture 限于同名文件与本任务模拟 Provider 支持。
- [x] 由独立检查代理审阅：业务状态不靠 SQL 制造、三方授权链正确、五次实际执行、报告不包含凭据。
- [x] 先 list 核对选择范围和前提，再在已核实的 task 环境执行：

```sh
pnpm exec playwright test e2e/collaboration-multi-account-closure.spec.ts --config <task-config> --workers=1 --retries=0
pnpm exec playwright test e2e/collaboration-host-squad.spec.ts e2e/collaboration-host-switch.spec.ts --config <task-config> --workers=1 --retries=0
```

- [x] 对上下文专项单独选择 memory-provider 后运行 `e2e/collaboration-memory-runtime.spec.ts`；普通 host/squad 测试仍使用 host-provider，不混用。
- [x] B01–B11 优先放入新增主线/辅助场景或复用 password-compatible 用例，不直接在 password 环境执行 legacy 测试。
- [x] 对所有 skip 检查原因；已配置 host 能力缺失必须报失败或阻塞，不能记为通过。
- [x] 失败先保留截图、请求/运行身份和日志，再最小复现。只修正本次测试设施；产品问题记录后继续不依赖它的检查。

## 4. 汇总与收尾

- [x] 执行新测试文件适用的 lint/typecheck；记录调用命令、命名用例结果、失败/跳过数。
- [x] 检查运行前后源码身份，解释必要生成文件变化，不把其他会话的变化计入本次。
- [x] 停止本任务进程、核对恢复测试门；保留证据和数据库，记录环境保留位置。
- [x] 输出中文 `report.md`、`results.json`、截图与日志索引；静态发现不能替代动态结果。
- [x] 独立审阅报告数字与证据，一致后返回用户。完成标准是诚实的测试结论与可复现问题，不以修改产品强求全绿。

## 写入范围

- 本任务目录及 `artifacts/qa/collaboration-multi-account-20261109/`。
- 新的闭环测试、必要 fixture；如需改已有 fixture，先确认无并发冲突并尽量新增包装避免改动。
- 环境文件/凭据为私有目录、权限 0600，不纳入报告和 Git。
- 不修改产品代码、不提交已有脏工作区、不部署、不调用真实付费 Agent。
