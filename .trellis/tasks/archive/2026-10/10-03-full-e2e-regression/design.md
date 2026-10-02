# 测试设计

复用 make check 的隔离环境、现有 TestApiClient 和 Playwright。基础通道使用 legacy auth，专项通道使用 password auth 和 platform-admin 功能开关。并行通道不得共享数据库、端口或生产 Web 构建目录；需要第二份构建时采用独立快照。Electron 使用项目自带隔离 profile fixture，模型服务使用确定性本地 provider。

覆盖证据按通道保存到 .gstack/qa-reports/2026-10-03-full-e2e/。原有验收记录只作定位线索，不作为本次通过证据。失败先复现及诊断，再做最小修复与回归。
