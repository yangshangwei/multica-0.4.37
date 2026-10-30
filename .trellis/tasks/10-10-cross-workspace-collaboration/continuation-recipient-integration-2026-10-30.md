# 接收人选择器集成与真实验收（2026-10-30）

## 集成分支

- 基线：最新本地父分支 `codex/cross-workspace-collaboration` at `85b98c3bb`
- 集成分支：`codex/collab-recipient-picker-integration-20261030`
- 合并提交：`0041aa4b2`
- E2E 修复提交：`4aa77e08e`
- 工作树：`/Volumes/artisan/code/2026/mc-recipient-picker-integration-20261030`

选择器实现、父任务契约、归档任务记录和测试均已合入；合并过程无冲突。

## 真实接收人验收

使用独立 PostgreSQL/API/Web 环境手动启动生产 Web 构建与 API，真实 Playwright 规格通过：

- 规格：`e2e/collaboration-recipient-picker-web.spec.ts`
- 结果：**1 passed**
- 覆盖：本人行、最近协作、姓名搜索、响应字段隐私、Escape、窄屏布局
- API：`/api/collaboration/recipients/search` 返回 200，真实用户/工作空间/协作历史由测试创建
- 修复内容：E2E 使用 `NEXT_PUBLIC_API_URL` 作为 check 环境 API 回退；Portal 选择器使用页面级 listbox；自己行断言使用“Assign to me”

## make check

`ENV_FILE=.env.worktree make check` 的静态、TypeScript、Go race/vet、生产 Web build 和 API/Web 启动阶段均通过。Go 全包 race 使用仓库默认长耗时 service 测试，最终通过。

完整 Playwright 阶段报告 **78 个既有全站规格失败**，主要页面落在 onboarding welcome 状态，涉及大量与接收人选择器无关的旧规格；接收人专项规格已单独通过。失败结果保留在本工作树的 `test-results/.last-run.json`，check 环境日志在 `/Users/artisan/.multica/dev/envs/check-20261030020744-86827/logs/`。

宿主机没有 `psql`/`pg_isready`，验证使用了只读 Docker PostgreSQL wrapper 将本地连接转到 compose 的 `postgres` 服务；没有修改应用代码或生产数据。

## 下一步

当前集成分支已清洁并具备合并证据。全站 Playwright 的 onboarding/fixture 问题应另立 QA 环境任务处理，不应回滚或修改接收人选择器实现。
