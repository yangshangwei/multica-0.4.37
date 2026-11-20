# 查重与基线（规划时点）

创建前用 `get_context.py` / 各 `task.json` 核对 13 个本地活动任务，按两个原文件名、失败行为、普通状态乐观更新及迭代历史检索任务范围、`.omx/plans`、`docs/plans`。未找到为 #37 或 #38 实施这两条交互修复的在办任务。此结论仅限本地；不是外部工单查询结果。

- **#37 相邻但非重复：** `11-12-agent-squad-followup-roadmap/implement.md:54` 已规定仅 bound completion 关闭乐观完成，普通编辑保留。PAY-241 S2 研究材料中出现两个 spec 的快照路径，并非认领标题。
- **#38 历史规格：** `archive/2026-10/10-05-iterations-i1-history` 和 `10-05-iterations-i1-verification` 均 `completed`，不是处理 UUID 地址替换期间用户操作的活动任务。`archive/2026-11/11-02-branch-functional-regression` 也是历史验收。
- **已有归属：** `11-17-quick-create-blocked-feedback/implement.md:57` 明列 #51、#52 的文件，`:71` 要求隔离整文件验证；这是范围关联，不是通过证明。`11-13-pay-241-s3-recovery/research/information-recovery-client-intake-20261118.md:19` 对应真实 DTO、Core、Views 与双端夹具。和旧运行不同，此时工作树已有未提交的 `information-schema.ts` 与 `packages/core/collaboration/index.ts:7` 导出；未验证当前收集结果，不借用或改写这些改动。

`source-commit` 和当前 HEAD 均为 `72c207f72`；原基线**另含当时的未提交改动**。将当前文件与 `.artifacts/e2e-20261118-203130/source/` 逐字节对比，以下 12 个相同：两个原 spec；Core `issues/{queries,mutations,canonical-id,cache-coordinator}.ts`；Views `issues/components/{issue-detail,issue-detail-route}.tsx`、`issues/actions/use-issue-actions.ts`、`iterations/iteration-participation.tsx`；Web `platform/navigation.tsx`、`app/[workspaceSlug]/(dashboard)/issues/[id]/page.tsx`。这不是整棵工作树一致的证明。实施前再次检查目标路径、关联在办变更和被测构建，禁止用旧 `source/` 或干净 HEAD 覆盖现状。
