# 执行计划

1. 独立工作树读最新 execenv/adapter 与 Codex schema，记录配置优先级、文档收集及字节截断依据，完成设计评审。
2. 先写 fake-only 预算和启动拒绝测试，覆盖中文与多文档、显式不足值、重复键/profile/CLI、文件变化和失败不调用。
3. 在注入后的真实边界实现预算和任务私有配置写入；不改共享配置，不添加无界常量。
4. adapter/execenv 整包、定向 race/vet/gofmt，普通测试 CLI 防护；独立复核配置读取顺序与恢复路径。
5. 单独申请有界真实 Codex 验证参数后，用标记入口观察尾部，记录 requested/observed 模型、调用账本、输入哈希、脱敏结果及会话 sha256。
6. 同步运行时配置规范与手工 workaround 文档，atomic conventional + Lore 提交，关联安全小队父任务。

本轮交付规划；无产品实现、无账号调用。安全小队子任务二预算不自动覆盖本任务。
