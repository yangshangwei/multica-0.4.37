# 分拣台欢迎页实施计划

1. 在 `packages/views/triage/triage-page.test.tsx` 或同域测试补充能区分首次空白、日常空态及数据未确认的回归用例。验证用例在产品改动前捕捉行为缺口。
2. 在 `packages/views/triage/triage-page.tsx` 接入未决数量与轻量历史检查，接好欢迎区的弹窗回调和日常空态导航。复用 `packages/core/triage/queries.ts` 的查询选项。
3. 新建 `packages/views/triage/triage-welcome.tsx`，必要时使用一个局部样式文件。更新 `packages/views/locales/{en,zh-Hans}/triage.json` 和 `packages/core/docs/slugs.ts`，接入内置分拣台文档。
   同步扩展 `e2e/triage.spec.ts` 的首次空白、创建、CSV、文档及主题/尺寸验收，维护 `.trellis/spec/views/frontend/triage-welcome.md` 的查询与焦点契约。
4. 运行 `pnpm --filter @multica/views test triage locales/parity.test.ts`、相关文档链接测试、受影响包的 typecheck 和变更文件的 ESLint，检查 `git diff --check`。重点检查队列在数据刷新和操作后的切换。
5. 用浏览器在正常路径验证创建和导入弹窗、创建后的队列、审核后的日常空态和文档入口。在宽屏及窄屏检查中文、英文、浅色及深色截图。用 Impeccable detector 与 visual-verdict 记录结果，必要时合并修正后复验。
6. 运行 Trellis 的独立质量检查，记录实际验收结果和交付限制。只提交本任务的文档、代码和相关规格。

用户已确认上一轮给出的方案并明确要求建档、写需求及设计文档后开发。按上述范围实施。
