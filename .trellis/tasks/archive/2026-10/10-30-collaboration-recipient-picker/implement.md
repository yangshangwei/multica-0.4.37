# 实施计划：协作接收人选择器

## 前置检查

- 保留并审查 `claude/collab-recipient-picker` 工作树中已有的未提交 server/core 实现，不覆盖其他任务改动。
- 先用 `git diff --check`、相关单测和 typecheck 确认现有增量基线；handler 集成测试必须切换到完整迁移的隔离数据库。
- 读取并遵守 `collaboration-recipient-picker` continuation contract、server/core/views spec 和 cross-layer guide。

## 执行顺序

1. **后端增量收口**
   - 检查 `/recipients/search` handler、service、SQL、generated sqlc、环境变量和已有 resolver 的兼容性。
   - 补齐/修复 strict body、cursor、权限二次复核、related scope、recent、members-only、限流和错误投影测试。
   - 用完整迁移的隔离 PostgreSQL 重跑 service/handler race；如 SQL 变化，串行生成 sqlc 并检查漂移。
2. **Core 契约收口**
   - 复核 Zod schema、ApiClient session fence、infinite query key、AbortSignal、section dedup 和旧草稿兼容。
   - 补异常响应、旧 server、空页、重复账号、context 切换和 stale result 测试。
3. **共享选择器组件**
   - 新建 `packages/views/collaboration/account-picker.tsx`，复用已有 picker/Dialog/Popover primitive。
   - 实现触发器、self/selected/recent/all 分组、搜索防抖、分页、精确查找 fallback、错误重试、键盘/焦点/Escape、外部标记和成员-only。
   - 补 component tests，使用真实 UI primitive，不用 div mock 替代焦点行为。
4. **三个入口接线**
   - 改造 `creation-form.tsx`、`workflow.tsx`、`field-inputs.tsx`，删除旧的手动输入/resolve 按钮路径，保留精确查找作为 picker 内动作。
   - 维持 creation draft、workflow intent、`member:<uuid>` 属性引用和现有提交幂等语义。
5. **文案、配置与文档**
   - 更新 `en` / `zh-Hans` collaboration 文案，跑 parity。
   - 检查 `.env.example`、docs、docker compose 和离线升级说明的一致性；不新增未批准的后台设置页面。
6. **端到端验收**
   - 增加 Web E2E：新建默认本人、分页/搜索/最近、精确兜底、邀请/转交、members-only、撤权/过期响应、窄屏和键盘操作。
   - 做一次视觉检查和浏览器可访问性检查，记录截图、构建命令和测试提交。
7. **全量质量门**
   - 运行相关 Vitest、views/core typecheck、lint、Go race/vet、sqlc drift、E2E 和 `make check` 可执行范围。
   - `trellis-check` 独立复核 spec、跨层数据流、权限边界和未接入口；修复后复跑受影响检查。
   - 更新任务 progress/verification，必要时更新 spec；按 Lore Commit Protocol 提交。

## 验证命令

```bash
pnpm exec vitest run packages/core/api/collaboration-recipient-search.test.ts packages/core/collaboration/recipient-sections.test.ts
pnpm --filter @multica/core typecheck
pnpm --filter @multica/views test -- collaboration
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -race ./internal/service -run 'CollaborationRecipientSearch|CollaborationRecipient'
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -race ./internal/handler -run 'SearchCollaborationRecipient|ResolveCollaborationRecipient'
go -C server vet ./internal/service ./internal/handler ./cmd/server
git diff --check
pnpm typecheck
pnpm test
make check
```

E2E 必须使用生产 Web 构建和带完整迁移的隔离数据；不把当前缺少 `issue_collaboration` 表的默认测试库结果当成产品通过。

## 风险、回滚与交付门

- **高风险**：列表范围和 `external` 字段可能泄露来源成员关系；任何放宽都必须有权限测试和独立复核。
- **中风险**：选择器状态、旧草稿和授权变化跨 core/views/server；保留 resolver 作为 fallback，先能回切 UI 再发布。
- **性能风险**：一期接受中小规模顺序扫描；若 p95 超过 200ms 或规模超过 5 万账号，暂停扩展并另立索引任务。
- **交付门**：未完成三入口接线、真实数据库 handler 验收、键盘/窄屏 E2E 和独立 `trellis-check` 前，不得 `task.py start` 进入实现后宣称完成。
