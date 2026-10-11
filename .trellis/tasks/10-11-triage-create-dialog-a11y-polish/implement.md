# 实施计划：待分拣任务创建弹窗优化

## 顺序

1. 更新 triage/编辑器/UI 组件的可访问属性与 label 关联，保持现有 API 默认值兼容。
2. 增加候选模式文案和中英文错误提示，补齐标题、描述、日期、URL 的字段状态。
3. 调整创建弹窗滚动结构、picker 触发器外观和关闭按钮触控/本地化属性。
4. 为纯校验/字段错误映射与创建表单关键状态补充有意义的测试；不复制实现细节测试。
5. 运行格式化、TypeScript、相关 Vitest 与 Impeccable detector，检查 diff 不包含无关改动。

## 预计文件

- `packages/views/triage/triage-create-dialog.tsx`
- `packages/views/triage/triage-fields.tsx`
- `packages/views/issues/components/pickers/property-picker.tsx`
- `packages/views/issues/components/pickers/{priority,project,assignee,label}-picker.tsx`
- `packages/views/editor/title-editor.tsx`
- `packages/views/editor/content-editor.tsx`
- `packages/views/editor/content-editor-accessibility.test.tsx`
- `packages/ui/components/ui/dialog.tsx`
- `packages/views/locales/{en,zh-Hans}/triage.json`
- `packages/views/triage/triage-create-dialog.validation.test.ts`
- 相关测试文件（按现有测试结构决定是否新增）

## 验证命令

```bash
pnpm --dir packages/views exec eslint triage/triage-create-dialog.tsx triage/triage-fields.tsx triage/triage-create-dialog.validation.test.ts
pnpm --dir packages/views exec vitest run triage
pnpm --dir packages/views typecheck
pnpm --dir packages/ui typecheck
/Users/artisan/.agents/skills/impeccable/scripts/impeccable detect --json packages/views/triage/triage-create-dialog.tsx packages/views/triage/triage-fields.tsx packages/ui/components/ui/dialog.tsx
```

## 风险与检查点

- 共享 picker trigger API 可能被多个上下文使用；所有新增 props 必须可选，默认渲染保持不变。
- `DialogFooter` 的负 margin 依赖 DialogContent padding；改为 flex 时需确认 footer 边框和圆角仍与共享组件一致。
- 中文 locale 已有候选字段 key，英文 locale 需保持 parity；运行 locale parity 或 views 相关测试。
- 当前工作区有其他未提交改动；只编辑预计文件，验证时使用路径限定命令。

## 完成记录

以上步骤均已完成。字段隔离、上传、共享选择器冷启动和自定义触发器回归已补齐；完整命令、证据和验收边界见 `progress.md`。
