# 技术设计：合并后完整测试与业务影响验收

## 验证边界

测试分为三层：

1. **提交边界**：当前 `HEAD=ee037d4ee`；用 `git diff` 证明产品树与已验证候选
   `52162f19a` 一致。新增的归档记录不参与产品测试。
2. **执行边界**：优先使用干净的候选 worktree，所有 API/Web/renderer/数据库使用
   `scripts/check.sh` 分配的任务独占环境。根 worktree 的已有 dirty/untracked 内容只读，
   不作为候选输入。
3. **业务边界**：按近期 commit 触达的模块聚合结果，而不是只按文件是否有单测：
   UI/views → capability auth/handler/service → daemon/context/knowledge → projects/navigation
   → 跨层生产 Web/Desktop/Playwright。

## 数据流和运行顺序

```text
HEAD/candidate equality
        |
        v
diff --check + lint/typecheck + package tests
        |
        v
targeted Go race/vet + targeted TS/E2E
        |
        v
isolated API + production Web + built Desktop renderer
        |
        v
full make check (fresh DB/ports, one worker, zero retries)
        |
        v
business-domain report + retained logs/hashes
```

`make check` 是最终跨层门禁，负责迁移、隔离数据库、生产构建、renderer origin、API/Web
进程归属、Playwright 和失败清理。针对性检查先行，用于在长门禁前快速定位近期变更影响。

## 近期变更到测试范围的映射

| 变更边界 | 代码/测试入口 | 业务风险 |
| --- | --- | --- |
| UI input tokens/borders | `packages/ui/**`, `apps/web/app/brand-variant-cascade.test.ts` | 表单边界、主题/可见性回归 |
| Collaboration actions/content | `packages/views/collaboration/**`, batch action, rich content | 受限动作、字段编辑、来源/富文本状态 |
| Task capability | `server/internal/auth`, `handler`, `service`, `cmd/server` | 授权快照、runtime owner、拒绝与不可绕过 |
| Daemon/context | `server/internal/daemon/**`, `cmd/server/rk1k_*` | transcript drain、终态、实验停止与证据 |
| Projects/navigation | `e2e/projects-p1-lifecycle.spec.ts`, `e2e/global-scope-navigation.spec.ts` | refresh/revocation、请求替换后的 canonical route |
| Verification harness | `scripts/check*.mjs/sh` | native renderer origin、隔离与清理 |

## 兼容性与回滚

- 不改变任何产品、数据库或测试文件；本任务只新增验收记录。
- 所有测试命令记录固定 commit、工作目录和环境 manifest；长门禁失败时保留日志，不重置
  或清理来源 worktree。
- 若候选 worktree 被占用或依赖缺失，不在根目录临时修改 `.env`、node_modules 或产品
  文件；报告为环境阻塞并继续可执行的静态/包级检查。

## 结果分类

- **PASS**：命令退出 0 且业务断言/环境身份满足对应 spec。
- **FAIL**：产品断言失败、编译失败、服务异常或测试退出非 0。
- **BLOCKED/ENV**：前置依赖、端口、数据库、浏览器或显式 gate 缺失，未执行到业务断言。
- **SKIP/CONDITIONAL**：测试按仓库既有 gate 跳过；必须列出触发条件和未覆盖范围。
