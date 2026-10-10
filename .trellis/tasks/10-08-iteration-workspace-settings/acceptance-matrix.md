# Final acceptance evidence matrix

Status: all criteria passed. The full baseline suite and the current combined Web/Electron run have separate source identities in verification.md. Historical failed runs remain available.

| Criterion | Observable requirement | Canonical evidence | Final result |
| --- | --- | --- | --- |
| AC-01 | 桌面端和 Web 均能从设置找到“迭代”，并能用 `?tab=iterations` 直达；未启用时仍可发现设置。 | settings-nav/settings-page tests; iteration-settings Web/Electron routes | Pass; see verification.md |
| AC-02 | 无 `FF_ITERATIONS_I1` 或旧值为 false 时，新工作空间仍未启用；管理员可在页面启用，已有启用状态不会被迁移覆盖。 | handler iteration_settings tests; settings capability assertions | Pass; see verification.md |
| AC-03 | 普通成员、机器凭据、失去管理员权限的旧页面无法绕过服务端改变启停/时区；跨工作空间不可读写。 | handler/service authorization tests; settings read-only, revocation and rejected-enable retry regressions | Pass; see verification.md |
| AC-04 | 开启后列表入口出现，可创建迭代；本身不生成周期或任务、不启动执行；其他客户端收到状态更新。 | settings same-mount E2E; sidebar and realtime cache tests | Pass; see verification.md |
| AC-05 | 切到关闭时先展示完整影响及原因；取消仍启用；确认后一次性结束/取消/清理，任务与执行保持不变。 | lifecycle/closure transaction tests; full disable E2E and same-page feedback regression | Pass; see verification.md |
| AC-06 | 关闭影响超过当前后端 2,000 条任务上限时无任何变更，提示先整理迭代；并发变化须重新预览。 | iteration lifecycle preview/apply limit and conflict tests | Pass; see verification.md |
| AC-07 | 禁用后可以从设置查看保留历史，旧链接仍工作；重新启用不恢复旧周期或任务归属。 | settings response-loss E2E; frozen history and re-enable no-revival assertions | Pass; see verification.md |
| AC-08 | 常规设置唯一编辑共享时区；迭代页展示保存值并跳转，启用提交该值且陈旧确认被拒绝；旧周期的日期与快照不变。 | planning-timezone tests; bilingual single-editor E2E; explicit saved-zone detail regressions | Pass; see verification.md |
| AC-09 | 启停请求提交后响应丢失，可凭相同请求恢复结果，不重复关闭、重复通知或覆盖后续启停。 | core command/access tests; original request recovery in Web/Electron E2E | Pass; see verification.md |
| AC-10 | 旧后端缺失能力接口时显示不支持，提供重试；真实撤权不伪装为旧版本。 | core API compatibility tests; unsupported and revoked settings UI tests | Pass; see verification.md |
| AC-11 | 中文/英文、键盘开关和对话框操作、深色及窄窗口无溢出；所有关键状态有可读反馈。 | locale parity; real UI screenshots, keyboard and width checks | Pass; see verification.md |
| AC-12 | 设置只展示已经支持的业务；分拣台的迭代分配能力、任务选择器和共享项目时区跟随变化更新。 | manual-mode settings; triage capability invalidation and shared-timezone tests | Pass; see verification.md |
