# P1 端到端测试细化计划

2026-10-06。用户要求细化此前 8 条端到端用例。原 8 条是复合业务链，不等于仅有 8 个断言，但多个行为共用一个成功前缀，任一早期失败会阻断后续证明。本轮将其拆成独立可运行、可定位失败的场景，并补真实 UI 恢复与权限边界。

## 改动前基线与范围

- 干净基线 `421ca49eb`，产品实现 `31534d844`；前轮 8/8 实跑和失败修复证据保留在 browser-verification.md 及 browser-evidence/。
- 只调整测试、最小测试 fixture 与验收文档。不修改功能来迎合测试。发现真实缺陷时先记录根因和失败证据再修复。
- 复用 TestApiClient、真实认证、现有 P1 fixture、production Web 与真实 Electron renderer/preload；不用真实 agent/daemon，不增加依赖。
- 单元/数据库层已经有完整纯规则和权限矩阵，不逐格复制到浏览器；E2E 重点是实际用户操作、跨层接线、失败恢复和端差异。

## 实施拆分

| 文件 | 独立行为与新增边界 | 所有权 |
| --- | --- | --- |
| projects-p1-goals.spec.ts | 模板预览/取消、选择追加、重复章节、描述 CAS 本地合并与服务端版本采用 | goals/health lane |
| projects-p1-health.spec.ts | 完成/取消拆分、空集合、全终态、五视图分别排除候选、接受只准入 | goals/health lane |
| projects-p1-risk.spec.ts | 准确下钻、个人过滤隔离、跨版本继续、持续提示、空后缀、从头刷新与失败保留 | goals/health lane |
| projects-p1-progress.spec.ts | 发布/预览、草稿恢复、幂等重试、修订、验收、执行证据、通知与来源失权 | progress lane |
| projects-p1-lifecycle.spec.ts | 完成警告、状态与执行独立、删除、时区权限、能力/网络恢复、窄屏键盘 | parent |
| projects-p1-desktop.spec.ts | 原生导航、风险、快速预览、草稿恢复、历史与中文布局分别验证 | parent |
| projects-p1.spec.ts | 保留独立 30 样本双页收敛测量；迁出的复合测试不重复计数 | parent |

目标约 40 条以上，以有区别的行为和准确断言决定最终数量，不以 test.step 数或重复数据膨胀数量。每个 test 有稳定编号、自己的前置数据、明确操作/预期和清理；用 API 建立先决状态时，不把该 API setup 计作 UI 覆盖。

## 验证与完成条件

1. 原断言逐项映射到新测试，避免拆分时丢失 private evidence、历史不变、候选排除、零执行、错误恢复等关键证明。
2. 严格 TypeScript 检查及 Playwright 收集通过；每条测试独立发现、无串行前缀依赖、无skip或掩盖问题的retry。
3. 源码冻结后使用本工作树归属的生产构建，实跑全部细化用例；失败按真实原因修复，保留失败与最终结果。
4. 检查用例名/编号、实际计数和覆盖表一致，区分“拆分已有覆盖”和“新增覆盖”，更新当前验收说明并保留原 8 项历史证据。
5. I1 真实迭代、iOS 视觉、真实 agent 账户和生产部署仍不属于本轮。
