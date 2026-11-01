# 最终联合候选（验证进行中）

候选 `7f8ca8725` 起始工作区干净，来源指纹、框架／launcher／CLI／配置／renderer 构件和 E2E 树已登记到 `registration.json`。这是验证开始时的登记，**不是最终验收通过，也不是付费启动授权**。

同源 CLI 和 production renderer 构建通过；sqlc 使用本地缓存的固定版本生成，无漂移。完整 `make check` 已启动，Go 每包使用独立新克隆库并保留原始失败。旧 C3 维护服务已停止且数据库保留。后续门控 password/Web/原生、知识开关开关两态、安静窗口 RK1 fake/race、安装升级与账号门分别待验。

框架继续 runnable=false，实际真实调用0。所有历史失败、默认库迁移事故恢复边界、平台及预算缺口保留；本次只做本地操作。
