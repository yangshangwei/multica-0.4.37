# 修复边界与证据设计

原始证据只读位于 /Volumes/artisan/code/2026/multica-0.4.37。新工作区 /Users/artisan/.codex/worktrees/main-acceptance-fixes/multica-0.4.37；分支codex/main-post-merge-acceptance-fixes。
88项精选publication manifest已逐项hash匹配；原memory api.log和私有campaign目录存在。逐lane再验证实际原始证据，不能以摘要代替缺失日志。

## Root cause groups and hypotheses
1. D01 已复现产品权限缺陷：角色分支缺human principal。复用既有task及身份判据；完整真实凭据正反矩阵。
2. D02 真实DB表锁竞争：持锁者未知。独立DB、pg_locks/pg_stat_activity时间线捕获；区分background/autovacuum/test泄露；保持NOWAIT安全语义。
3. D03 依赖与工具链扫描：支持Go1.26补丁与本机1.27分开；核实官方修复版本，现有依赖最小更新，无新增依赖。
4. D04/D05 执行前置：Host缺transport/context和时间顺序；Namespace缺probe实际原因。先脱敏诊断，再按证据最小修复；不可从另一安全配置外推。
5. D06-D09 fixture语义漂移：以当前可访问导航、workspace菜单和拒绝区域为准；保留全部业务断言。
6. D11 旧CLI时限/清理：完整10次操作及清理，不改240秒、不以9次relay推断成功。
7. D10 外部GitHub403：采集安全响应头与状态后分类，不借用用户token。

## Ownership
评论handler及测试；service锁；依赖/构建安全配置；daemon/namespace/transport；E2E导航与Main/CLI，各自独立写集。父任务负责证据、资源分配、构建与集成。
各子任务先保存边界和回归计划，经内部审查开始；后续发现跨域变更由父任务协调。

## Rollback
保留未提交小diff；失败数据库和历史日志不删除。资源只按新任务owner精确停止；任何恢复操作有receipt。

## Internal review gates
Parent design accepted in research/plan-review.md. Parent separately reviewed comment, db-lock, security and runtime child plans and accepts their bounded implementation. Client plan remains under preparation.
Every actual acceptance run consumes a frozen candidate with base SHA, dirty bytes/modes and relevant new files, tool/config/build hashes and dependency resolution. Moving-source builds/tests are preliminary diagnostics only. Before/after hashes and named terminal inventories are required.
Parent allocates a fresh database per Go package and browser lane, verifies identity, budgets cluster connections and retains CLI guard. API-specific configuration must not enter ordinary Go tests.
Before macOS package preparation, preserve original multica:// default and full registered URL set. Restore only task-owned registration and independently verify exact prior state after cleanup. HOME alone is insufficient.
Before final integrated runs, reconcile defect→exact command/identity/configuration matrix. Historical pass counts never become new candidate passes. Platform limitations describe this available environment only.
