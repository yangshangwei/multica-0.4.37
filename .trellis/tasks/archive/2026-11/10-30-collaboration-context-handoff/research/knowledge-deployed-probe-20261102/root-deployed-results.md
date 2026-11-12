# 两个实际部署开关旅程

产品源码E6 `e6e7fe07d`，probe/fake源码`5bd1ecf13`。两个独立API/专库/生产Web串行执行，各1pass/0fail/0skip，worker1/retries0，真实Provider0。

OFF以真实批准候选正常执行一次fake Host，实际stdin无知识段、无知识会话隔离flag，knowledge claim/preparation/authorization事件各0。ON实际stdin只含该批准candidate/version/contentSHA，启用会话隔离flag，三类事件各1。两态均检查精确owner输入/run、单次物理启动、成功settlement、已授权reaped终止/proof及sealedledger；关闭daemon后无stderr/errors。

原始附件私有保留；公开只保留身份、内容/输入/构件哈希和机器结果，见root-deployed-results.json。输入正文与凭据不提交。此前parser红绿及静态证据仍保留。该结果关闭部署harness范围，不冒充最终候选、真实模型、UI审批、安装或多账号证据。独立最终复核待完成。
