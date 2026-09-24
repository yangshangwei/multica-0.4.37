# 技术设计

五类治理主题以 skill/小队路由为主，不直接增加五个角色：契约兼容由 architect/code-reviewer/release-engineer 组合，威胁建模与供应链由 security-reviewer/release-engineer 组合，产品结果由 product-analyst/progress-reporter 组合，灾备由 reliability-engineer/release-engineer 组合。每个 skill 使用统一证据表并声明触发、交付、验证和人工审批。

契约兼容必须覆盖 parseWithFallback、OpenAPI、桌面旧客户端和插件边界；供应链检查锁文件、来源、构建产物和 SBOM 可得性；灾备检查 RPO/RTO、备份恢复和降级，不以“可回滚提交”替代数据恢复证据。
