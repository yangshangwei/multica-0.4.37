# UI/CLI 完成提示阶段交付复核

结论：**本消费者增量通过，可按精确清单提交**。仅覆盖普通单事项 status-only Web/Desktop/CLI 提示，提示不授予完成权限；v1、完整 M1、M2/M3 与真实供应商验收仍独立。

## 已修复

- Core 将部分结构化 404/405 错当旧路由：补齐 `error_code/resource/permission_code/delivery_code`，保留明确拒绝。
- 显式 Retry 复用旧 revision：先读取当前工作空间的最新事项，再请求新提示；旧结果对话框先关闭，避免关闭后续结果。
- 对话框显示后所属组件卸载仍能确认：请求控制器保留至本轮结束，卸载后的确认/重试不再写入。
- 重复确认及在途写入过早解除单飞：一次消费确认，各入口 continuation 等真实 mutation 的 `onSettled`。
- 协作源事项误入普通提示 API：共享 gate 保留原协作流程，不扩大本轮提示范围。

上述缺陷先有失败回归，再修复；4 条 Core 分类、3 条 gate 以及1条协作源回归的原失败日志均保留。新增真实入口测试证明 mutation 未 settled 前第二次完成不再发 hint/PUT；modal 测试证明重试先关旧结果。

## 本轮验证

| 检查 | 结果 |
|---|---|
| Core 定向 Vitest | 2 文件，22 通过 |
| Views 定向 Vitest（gate/actions/detail/table/source/modal） | 6 文件，131 通过 |
| Core 与 Views typecheck | 通过 |
| Core 与 Views 包级 lint | 通过；Views 既有27条 warning保留，0 error |
| 完整 `go test ./cmd/multica -count=1` | 通过 |
| `go vet ./cmd/multica` | 通过 |
| Builtin模板/严格YAML/working-on-issues/creating-agents合同 | 通过 |
| gofmt / 精确文件 diff-check / 双语提示 key parity | 通过；11个key一致 |

中间新增测试曾因对 Proxy API 使用 `spyOn` 失败，已改成模块 API mock；中间 lint 在测试编辑期间出现解析失败，最终稳定文件完整 lint 与 typecheck 均通过。所有失败记录保留，不计作产品最终通过。

## 精确交付范围与证据

`hints-check.json` 与 `hints-source-manifest.json` 列出28个产品/测试/内置技能文件及最终SHA256。包含经主会话确认的 run-confirm `onSuccess` 转发及 creating-agents 非授权语义同步。未提交、未归档、未更改任务状态；主会话负责提交。

主会话更新的 `.trellis/spec/server/delivery-completion-hints.md` 消费者小节与本次实现一致；`server/index.md` 只需纳入completion hints行，其余预存变化不属于本包。

本轮未运行真实Provider、生产数据库、全仓E2E或完整平台安装验收。历史通过数字没有替代本轮检查。批量、拖拽/move、创建即完成、混合字段、独立协作/R38提示、授权UI和后续研发路线继续留在后续任务。

## 暂存前空白补检

主会话暂存后发现3个原未跟踪文件有额外EOF空行，普通 `git diff --check` 此前未覆盖它们。现仅移除 `packages/core/api/delivery-completion-hint.test.ts`、`packages/views/issues/actions/completion-hint-gate.test.tsx`、`packages/views/issues/actions/completion-hint-gate.ts` 的多余末尾空行，保留一个换行；两个JSON清单SHA256已同步。无行为变化，未重跑测试；主会话重暂存后执行最终 `git diff --cached --check`。
