# 真实验收轮清理与保留

供应商明确官方客户端限制后停止新调用，未尝试其他模型通道。Gateway进程、2个角色运行容器均已停止；root relay及独占PostgreSQL容器、internal网络已删除。

3个任务库（P1真实、P1fake、O1fake）逐库pg_dump私有保存、sha256记录后，在0连接下DROP，无FORCE；独占集群和匿名卷删除，数据库口令配置删除。供应商bearer、actorheader与合成proxy token副本删除，未改用户原配置。

私有证据根 `/Volumes/artisan/code/2026/secrem-v2-real-evidence-20261120/` 权限0700；原始provider请求/响应/RPC/session、私有dump、失败历史和每轮注册保留，不能入仓库。脱敏分类及hash在research/provider-request-summary-20261120.json，DB保留hash在私有work/database-retention-cleanup.json。

恢复时需要明确允许SDK的配置、新私有运行环境和新注册；旧admission是历史准入，不自动授权跨源码或账号新运行。当前没有活跃provider进程或容器，替代路由尝试0。
