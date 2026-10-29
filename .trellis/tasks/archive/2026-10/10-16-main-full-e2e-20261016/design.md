# 测试设计

以当前 main 为固定被测版本。使用官方 make check 的 task-owned 环境，静态检查、TS、Go、生产 Web 和全量 Playwright 按顺序执行。认证模式特定测试在独立配置中补跑；原生 Electron 使用独立 profile 和可追溯 renderer。主会话负责环境和全量运行，独立 check agent 盘点模式门槛与补充验收，另一 check agent负责最近功能覆盖核验。

证据保存在 .omx/reports/main-full-e2e-20261016（被忽略目录）及机器临时路径；构建期间冻结源码和 Trellis 文档写入，防止来源校验变化。最终将汇总写回任务。

失败不自动修改产品；确认来源正确后独立重跑，必要时保存诊断证据。测试创建的数据仅在任务数据库中清理，不操作用户现有服务和数据。
