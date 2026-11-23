# 原17项：当前候选复验结果

**15 PASS / 2 BLOCKED。** 原15个失败用例均有本轮实际通过结果；两项安装组合本轮未准备当前构件，没有执行，不引用旧候选PASS关闭。

候选：`4c817dee9397a42ac903f419af7804b11d2d863e`。逐项使用文件＋完整标题，按最近实际终态归并；所有自动重试为0。测试适配与初次失败分别保留，来源见 [JSON映射](original-case-current-status.json)。

| 原用例键 | 本轮状态 | 当前测试 / 场景 | 选中完整运行 |
| --- | --- | --- | --- |
| `3dccc2100422` | PASS | `collaboration-account-desktop.spec.ts` — Desktop child links / records / restart | drawer-readiness-recheck-1 |
| `c919808d1d26` | PASS | `collaboration-account-desktop.spec.ts` — Desktop authors initial fields | legacy-test-repair-1 |
| `5a60ce5f71f0` | PASS | `collaboration-boundaries-web.spec.ts` — Smaller gaps / exact resolver | legacy-test-repair-1 |
| `158a87f90c8f` | PASS | `collaboration-delivery-web.spec.ts` — lost upload/create response / revoke | drawer-readiness-recheck-1 |
| `72c1ec89998d` | PASS | `collaboration-delivery-web.spec.ts` — rejected creation resubmission | legacy-test-repair-1 |
| `187ee1881c81` | PASS | `collaboration-editing-web.spec.ts` — source creation / field edits / discussion | legacy-test-repair-1 |
| `860dfeffcf00` | BLOCKED | `collaboration-installed-clients.spec.ts` — cached v0.6.7 CLI controls | 本轮未执行 |
| `8b009b887e43` | BLOCKED | `collaboration-installed-main.spec.ts` — installed Linux full Main | 本轮未执行 |
| `535e317ad170` | PASS | `collaboration-leave-web.spec.ts` — Leave / revoke / Stop / rejoin | password-leave-final-proof |
| `4971c07b93b6` | PASS | `collaboration-mention-journeys-desktop.spec.ts` — Desktop owner revoke / original key | legacy-deployment-final |
| `8e9c882cd544` | PASS | `collaboration-mention-journeys-web.spec.ts` — Web owner revoke / original key | legacy-deployment-final |
| `cd80f1b61840` | PASS | `collaboration-records-web.spec.ts` — Web child references / record pages | legacy-deployment-final |
| `f390d53c1162` | PASS | `collaboration-retained-attachment.spec.ts` — Web typed creation / private upload | password-collaboration-retained-attachment |
| `18e4a6485b4d` | PASS | `collaboration-retained-attachment.spec.ts` — Electron typed creation / private upload | password-collaboration-retained-attachment |
| `37ff666368df` | PASS | `collaboration-retained-attachment.spec.ts` — Web+Electron immutable file input / restart | password-retained-final-proof |
| `7b935d7ce8d5` | PASS | `collaboration-source-surface.spec.ts` — Web+Electron list/board/timeline | password-final-collaboration-source-surface |
| `QA-PRODUCT-001/ENV-1` | PASS | `collaboration-runtime-handoff-web.spec.ts` — startup memory v0 / live latest v1 | password-collaboration-runtime-handoff-web |

原437行coverage有14FAIL＋2BLOCKED，ENV-1另列在原summary，本映射合计17项。boundaries标题从“recipient lookup is exact-only”到“the exact recipient resolver stays read-only”的既定改名已显式映射；并未按相似标题随意合并。

两项BLOCKED的精确含义：本轮没有构建并配置用于该候选的Linux完整Main安装组合、保留v0.6.7 CLI组合及其对应运行前置。历史已有成功不能代表当前字节，也不推断本机runner不可能提供。

## 分列的旧门禁

- **管理筛选时钟回归：PASS，8项。** 当前views全文件输出已核验；views仍有3项别的新失败，因此不能把这一文件通过写成前端全部通过。
- **Runtime旧9项的canonical回归：PASS。** 当前13个唯一顶层测试的原始Go JSON均race PASS。初始390账本保留，独立补齐6顶层/18具名；组合396顶层/1014具名终态，0fail/skip。仅证明九项指定确定性回归，不代表完整后端或真实模型。

## 验收边界

上述15PASS只关闭原失败集合在当前候选上的这些具名复验，不是441项全量库存全绿，也不是单次make check通过。当前新增diagnostics/skill展示单元失败、canonical detail失败及平台/真实模型缺口独立保留；Web后续诊断PASS不替换原canonical失败。

本报告仅读取已完成raw、核对身份/状态/哈希并归并；未启动任何测试、服务、数据库或模型，也未改原始失败和旧报告。

最终浏览器ledger已交叉核对：68个本轮业务身份为66PASS/2FAIL，本表15个PASS逐项相符，两安装项不在实际执行库存中。两个canonical detail失败保留，后续诊断另计。
