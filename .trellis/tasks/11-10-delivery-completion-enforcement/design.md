# 设计边界（规划中）

## 现有复用点

- 项目 append-only policy、独立 policy revision CAS 和 project 锁是当前政策事实来源。
- 既有固定观察器验证受控制品的 spec、artifact、receipt、PR 已观察 head 及当前生产者/审批人/调用者权限；不新增平行观察器或执行引擎。
- 现有身份、lifecycle、状态 catalog、迭代记录与协作操作回执继续拥有各自事务内的事实。
- 原持久评估的私有读取和不可变历史合同保持独立；最终完成不能把旧 applicability 快照当授权。

## 事务边界

普通 update/move/batch、CreateInTx、WriteIssueStatus、协作 ChangeLifecycle 与 R38 都有实际写入事务。门禁必须参与调用者的原事务，不允许在事务外先检查再无条件写状态，也不允许用另一事务提交的“current”充当本事务准入。

当前观察和权限 helper 位于 handler；service 层不能反向依赖 handler。待源码审计确认最小复用接口，选择事务局部下沉或有明确锁与权限契约的窄注入，不复制观察算法。

状态 catalog 锁必须先于 issue；还需核对 workspace、身份、project/policy、制品审批/执行、PR link SHARE NOWAIT、协作用户/组织/W01 锁图。锁争用产生明确非成功，不以自动重试掩盖不确定提交。

## 业务边界待定

`research/decision-register.md` 的 D1–D7 尚未收敛，本文件不预先选定授权 schema、skip 字段、有效期数值、终态编辑策略、协作豁免或 public error codes。产品实现不能根据待选建议展开。

若 D4 要求已 done 移项目重验，范围还包括 project 删除的批量 detach 和协作 source editing，不限于普通 move。

## 分包原则

优先交付可独立证明的完成授权/事务准入核心，再覆盖各个真实 writer，最后开放 enforce 的政策准入。具体包界以最终业务合同和源码审计确定；未覆盖入口时禁止允许其绕过强制政策。仅基础能力交付必须准确标注，不关闭父 M1。

## 兼容与保留

off/observe 原观察、持久恢复和显式提示行为需要回归。历史事实、失败和不确定结果不得更新成通过。新数据库设计如必要，遵循无 FK、每个 CONCURRENTLY index 单独迁移、保留证据的 downgrade 拒绝及全程停止准入；目前未选定或编写迁移。
