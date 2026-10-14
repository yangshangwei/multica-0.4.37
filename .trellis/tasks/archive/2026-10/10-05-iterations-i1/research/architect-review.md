# Architect 评审（独立只读）

2026-10-05，代理 i1_arch_review。首次 REQUEST CHANGES；架构主体成立，要求明确 today/handoff 跨午夜派生日期锁定及事件锁后时间采样。

已修订：API start_preview 显式返回 reference_date/effective_start_date/effective_end_date/timezone 并纳入 hash；跨午夜 stale。design §3 采用全部锁取得后 clock_timestamp()/注入时钟，禁止 transaction_timestamp；sequence权威，时钟回拨记录 sampled_at并钳制业务时刻；processed_at不冒充commit时刻。test-spec新增两类竞态。

最强反对意见：workspace fence 覆盖所有普通任务写，是吞吐与接线成本。综合：保留保守线性化，writer inventory、真实锁序和普通写吞吐是FG硬门槛；性能不合格再评审锁粒度，不能偷偷绕开。

复审结论：APPROVE（工程方案）。两项时间阻塞已修订并列入测试，无新增规划阻塞。仍须通过 writer 全覆盖、锁序竞态及普通任务吞吐 FG 门槛；未执行产品测试。
