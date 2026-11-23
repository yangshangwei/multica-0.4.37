# 执行计划

1. 独立工作树重新核对最新基线，完成实际凭据分类及 Update/Delete 调用图，评审设计后激活任务。
2. 用 dbfx/testutil.Call 写失败回归，记录机器 owner/admin 与真正人类管理员的对照，含附件-only 路径与副作用计数。
3. 在两个管理员例外的真实权限边界实施最小修复，复用判据；保留自身评论路径。
4. 跑新鲜数据库上的鉴权/评论/能力整包与定向 race、go vet/gofmt；普通测试使用 CLI 防护。独立安全复核所有绕过路径。
5. 更新 task-api-capabilities/security-boundaries 等实际拥有规则的规范和源映射，按 conventional + Lore 提交，回填与安全小队父任务关联。

本轮只交付规划，未运行产品测试，未宣称漏洞已修复。
