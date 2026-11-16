# 项目知识自动沉淀：当前 ROOT 最终验收

批准范围的 P0–P6 已完成，最终独立检查接受 AC1–AC12 的全部 14 行，0 阻塞。当前验收源为 `ed6f6ff015cfe40bcb20eeaee9504d60ce2fb3f3937732b5d8e389663aa156a5`，对应 HEAD `1a5e537b…` 及其工作树联合输入；10,409 个文件／32 个工作空间源码链接。该证据不改写为任意后续 HEAD 的全量验证。

## 最终行为与验证

| 范围 | 实际结果 |
| --- | --- |
| 生产构建 | API、CLI、迁移工具、Web、Desktop 均通过；Web BUILD_ID `D8M3fs-jQU9KZq5kh81eG`。1,681 个构建产物和 8 个执行工具哈希已独立核对。 |
| 两端完整旅程 | Web 与实际 Electron 各 1 PASS，0 FAIL／SKIP／RETRY。验证正式验收、自动系统采纳、后续 daemon 精确输入、重开暂停、重验恢复和新执行。 |
| 无项目／纯人工 | 各自零自动工作、零自动候选、零新增模型请求和工作空间写入。 |
| 重验与历史 | 同 candidate/version 的 validity 1→2，accepted 3→6／close 2→4；旧续跑零新增 provider，原历史保留。 |
| 相关检查 | Go6 normal/race 每模式 546 named PASS；TAG3 每模式 17。Core／Views 222、Desktop 74、两个 Desktop tsc／6 文件 lint 通过，按完整实际输入限定复用。 |
| 新回归 | Git 索引夹具 8 PASS；P6 原生入口夹具 5 PASS，原失败与回滚证据保留。 |
| 所属资源 | 四服务、两个 daemon、四个 fake provider／Playwright 进程组无残留；四端口无监听，Browser DB client／other 连接均 0。所属 Go6 Redis container／network 一次清理，61471 无监听。 |

## 本轮收尾改动

- `server/cmd/server/collaboration_knowledge_host_composite_test.go`：按当前协议常量严格检查唯一协商版本，并继续验证 legacy／无版本请求不能降级已接纳的私有运行。
- `e2e/fixtures/project-knowledge-electron.mjs`：捕获原配置回调的完整 payload，保留 schema／API／WS／app 值，发一次 `configured` 真实 IPC 回包，覆盖 Server 公布 Web 地址的解析路径。
- `.trellis/spec/server/verification-environment.md`：分别绑定工作树原始字节和 Git clean 对象，保留权限／路径／工作空间链接及失败回滚要求。
- `.trellis/spec/web/frontend/e2e-run-environment.md`：记录 API-only 原生配置、单次回包、Server 地址及真实 renderer 验收的边界。
- 本任务的实施计划、状态、进度与审查记录：更新为当前 ROOT 最终接受，保留旧 TAG2、首次索引接入和 Redis post 分类器失败；未将它们改判为通过。

私有源码接入复用已接受的执行引擎，只增加当前 13 项改动的参数绑定。没有新增依赖，没有修改通用 Electron 夹具或覆盖 `VITE_APP_URL`，也没有规范化 Windows BAT 原始字节以绕过校验。

## 验收边界

当前 CORS／v2／严格 legacy 边界和生产 Web／原生 renderer 已通过。真实模型语义质量、安装版 Main／installer／updater 和其他平台保留独立验收，未在本次声明中通过。所有模型与 Provider 为受控本地夹具；未使用真实账户模型，未创建 PR、推送或部署。

本地工作提交为 `86690a919`；任务已标记 `completed` 并归档至 `archive/2026-11/11-14-project-knowledge-automation`。会话记录按后续日志步骤登记。并行 Desktop／LAN 和 PAY-241 改动按原负责人范围保留；本任务没有把它们作为自身提交的代码改动。

## 主证据

- [最终独立 AC](/Users/artisan/.multica/acceptance/knowledge-automation-persistent-20261116-495c4b90d6/p6-final-AC-ed6f-independent-28ef88d78bf2/review.json)，SHA `77c40b60931e279f750bf2f9a5887b5eddcd125bda14a2c47a159f3829e13dfd`
- [完整生产生命周期](/Users/artisan/.multica/acceptance/knowledge-automation-persistent-20261116-495c4b90d6/p6-browser-25c0cd1e/native-final-lifecycle-420c94463314/result.json)，SHA `1dbfc9d9e76baf9eecf2ec070bc150aa6b3706470f77d5d43fb1564a44074dcc`
- [实际两端旅程](/Users/artisan/.multica/acceptance/knowledge-automation-persistent-20261116-495c4b90d6/p6-browser-25c0cd1e/test-fe2b94c693b8/result.json)
- [生产构建](/Users/artisan/.multica/acceptance/knowledge-automation-persistent-20261116-495c4b90d6/p6-browser-25c0cd1e/build-8fb46c00bb66/build.json)
- [Go6 Redis 清理独立闭包](p6-go6-redis-cleanup-postaudit-reconciliation-20261116-native.json)
- [索引换行回归复盘](p6-source-index-normalization-retrospective-20261116.md)
