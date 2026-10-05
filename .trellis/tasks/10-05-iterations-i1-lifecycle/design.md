# 手动生命周期与任务归属技术设计

复用[父设计](../10-05-iterations-i1/design.md)和[唯一 API 合同](../10-05-iterations-i1/api-contract.md)，不另建不兼容 DTO/锁协议。具体所有权为父执行表 `lifecycle` 行；共享 SQL/router/sqlc 更改交 foundation 顺序整合，不覆盖其他任务。
