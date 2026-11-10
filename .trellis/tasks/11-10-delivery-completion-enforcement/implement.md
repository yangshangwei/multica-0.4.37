# 执行计划（待业务合同收敛）

1. [x] 核对当前业务分支 HEAD、前序提交祖先、新 worktree 与主工作区保护快照。
2. [x] 建立 planning 子任务，保留父任务 in_progress。
3. [x] 独立研究归档、原始日志、冻结清单、Session149–151，核对当前源码和已交付边界。
4. [x] 独立研究全部 done writer、隐含终态项目变更、排除入口、锁图和可复用接口。
5. [ ] 按 D1–D7 明确业务决定，落盘用户回答；不得把推荐当批准。
6. [ ] 确定最小可验收实现包，完成 PRD 收敛、最终设计、测试矩阵及非空 implement/check context manifests。
7. [ ] 使用 Trellis implement 子代理，按文件所有权拆分；行为改动先保留可证明 red 的实际测试。
8. [ ] 仅启动本包专属 PG，按 DB 包和最终普通/race 轮次建独立新库。禁止借用历史容器/数据库。
9. [ ] 真实认证路由覆盖实际 tar 字节/SHA、当前权限、各实际状态事务与持久事实；unknown、失权、到期、竞争和 commit 故障均不能放行。
10. [ ] 保留每次失败并标明行为 red / 夹具或设施失败；最终冻结源文件和具名测试库存。
11. [ ] 独立 Trellis check 审查并复算日志、源码哈希和全部入口覆盖；主会话最终 build/vet/sqlc/format/static 及适用测试。
12. [ ] 主会话更新 spec、六业务计划、父子任务和 journal，以 Lore 协议提交本包范围，安全本地集成并核对原 dirty 路径。
13. [ ] 仅归档已独立验收的子包；不关闭 M1/M2/M3 或真实模型验收门。停止本包资源并保留数据库/日志。

## 所有权

- completion_source_audit：只写 research/completion-source-audit.md 与 completion-source-manifest.json。
- archive_evidence_audit：只写 research/archive-evidence-audit.md/json。
- 主会话：任务合同、决策登记、spec、业务计划、父子状态、journal、集成与最终验收。
- 产品实现/测试 lane 在合同确定后再分配，不允许在研究阶段修改产品。

## 验收解释

本阶段仅做仓库/证据核对与规划，不把静态检查当新增运行验收；尚未运行本包 DB、普通/race、真实模型或浏览器测试。历史包已通过的计数只引用原冻结版本，不跨轮相加为新增覆盖。
