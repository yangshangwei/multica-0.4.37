# 初始化实施计划

用户已在完整目录方案之后明确要求新建任务、设计和开发，沿用该范围。先完成独立可验证工作，依赖运行时合同的部分不伪造接线。

## 顺序与归属

- [x] 新建任务与独立worktree/branch，保留原工作区及测试数据。
- [x] 固定用户范围、既有版本目录设计和运行时依赖研究快照。
- [x] 独立实现 server/internal/materialrepo：20文件嵌入catalog、稳定摘要、严格树检查、create-only候选及合并proof；仅此包由目录实现者写入。
- [x] 核对schema与所有模板、完整例子和缺失/冲突矩阵；确保不创建业务记录、不新增依赖。
- [x] 独立check复核目录内核，修复发现，运行go test/race/vet。
- [ ] 等待并引入前置11-27-git-material-real-e2e-acceptance的runtime operation/receipt合同，确认支持repository_initialization目标。
- [ ] 扩展project/root级持久化、维护者审核、idempotency与ready证明；一个owner管理SQL/migration/generated。
- [ ] 接入inspect/publish/review/merge/verify运行时阶段，复用既有队列，不新增App或凭据。
- [ ] 接入普通ready=true发布、后台领取、后续合并/方案启动门槛；保留草稿和历史。
- [ ] 共享core/views初始化状态、完整变更、审批/重试、双语与旧响应兼容。
- [ ] fake runtime跨层集成及Web/Desktop验证；真实测试仓验收记录单列且保留全部数据。
- [ ] 更新spec，最终检查、按Lore提交并记录会话；所有条件满足后才归档任务。

## 验证与证据

内核：`go -C server test ./internal/materialrepo -count=1`，`-race`，`go -C server vet ./internal/materialrepo`。只读取测试构建的tree/文件，无真实Git账户调用。

后端：为service/handler/router分别分配并核对专属测试数据库；前置SQL合并后再迁移，运行受影响初始化与普通物料拒绝/恢复矩阵，make sqlc，go build/vet。不得使用用户现有数据库作测试库。

前端：定向core schema/query与views组件测试；core/views lint/typecheck；集成后生产Web+Desktop。视觉验证依照repo要求保存证据。

验收矩阵：空结构、完整兼容、只有README、冲突AGENTS、不同schema版本、symlink/submodule、丢正文/部分树、遍历路径、已存在目标变化、额外变更、错误权限/旧批准、重复请求不同正文、失联恢复、配置/runtime改动、ready之前草稿/发布区别、旧数据保留。

## 接线阻塞条件

当前runtime execution改造尚在另一会话设计中。若其代码未落地，本任务保持in_progress，已完成工作按范围报告；不称完整feature完成，不用手写DB或固定服务端provider替代所选runtime。

## 回退

独立包无运行副作用。后续接线回退停止新初始化/维护任务，保留所有历史与远端事实，不逆向删除已初始化仓或测试数据。普通流程不能降级绕过尚未核验的规则门槛。
