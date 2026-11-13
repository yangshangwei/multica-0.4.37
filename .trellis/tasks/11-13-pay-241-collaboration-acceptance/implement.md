# 分阶段执行草案

当前阶段：planning。用户已确认S1首包；实际执行计划见 [S1子任务](../archive/2026-11/11-13-collaboration-agent-identity-baseline/implement.md)，其最终摘要已获用户批准，子任务现已验收归档。父任务不直接启动产品实现。

## 0. 规划与基线

- [x] 创建规划任务，核对当前规范、源码和同日PAY-1演练。
- [x] 保留24用例、完整目标、旧证据分类与当前NOT_RUN状态。
- [x] 明确七类状态、in_review专用语义、人工验收权和职业角色差距。
- [x] 提出首包及后续独立包，记录真实支付/发布/模型的验证边界。
- [x] 用户选择S1范围；首包PRD/design/implement在子任务收敛。
- [x] 创建S1子任务并配置上下文；没有新增后续未定案子任务。
- [x] 用户已批准最终S1摘要，仅激活该子任务。总任务不因开始首包而宣称全量已交付。

## 1. 已确认首包 S1

依赖：子任务最终方案批准；当前工作树差异重新核对。以下为概要，子任务B01–B10及具体文件归属是本包实施依据。

1. 读取 `trellis-before-dev` 与各层规范，记录基准HEAD、dirty文件、验证环境、测试Provider和二进制来源。只分配明确文件范围，不覆盖并行任务。
2. 为Agent作者公开投影新增失败回归：人类author合同不变、具体Agent可信归因、不可读来源不泄露、旧响应回退和畸形数据。
3. 使用 `trellis-implement` 子代理完成服务端/API/schema/共享视图最小改动，主会话协调边界。客户端子代理必须知道其他代理也在工作，不回退他人代码。
4. 复用canonical三账号闭环，补具体Agent展示及完整身份关联断言；对现有权限/重试/冲突/断线测试复用已有测试，避免重复测试同一规则。
5. 在任务自有DB/API与production Web运行；Desktop共享代码运行定向检查，声称Electron实际验证前必须获得对应renderer证据。
6. `trellis-check` 独立检查当前完整差异及跨层协议；修复必要缺陷，更新受影响规范。真实业务缺口保留到各自包。
7. 输出首包测试结果、变更文件、兼容性及未测项；提交只包含本包文件并使用Lore意图/验证记录。未经授权不发布生产或关闭业务任务。

建议文件归属：服务端评论DTO/投影及其测试；`packages/core/collaboration` schema及测试；`packages/views/collaboration` 评论展示与测试；canonical E2E及相关规范。具体文件在实施前查明后分配，不预先扩大到整个模块。

## 2. 后续包顺序

- S2：先定审批角色/最终验收/责任人与阶段表达，再写状态和权限矩阵测试，实施版本绑定建议/审批/流转；必须包含服务端绕过、撤权和旧批准重放。
- S3：独立补超时/取消/重复结果、事件补读/通知、并发及阻塞字段；优先覆盖现有未证实P0。
- S4：在S2合同稳定后接入真实支付沙箱、PR/CI、发布/回滚。真实部署由人工批准，测试工具不能冒充批准者。
- S5：S1–S4必要依赖通过后运行100独立用户与回写延迟验收；真实模型耗时/质量与平台受控测试分列。
- 最终：汇总八项交付物，按完整24用例及未解决缺陷判断，不汇总成虚假的部分通过率。

首包选择不会删除S2–S5或原需求；S1子任务已建立，其余包待各自范围收敛后再建立。

## 3. 验证命令候选

规划阶段没有运行以下命令；进入实施前核对脚本与真实测试名称。DB命令必须设置任务自有测试数据库并满足 `database-test-isolation.md` / `verification-environment.md`，禁止借用旧演练DB。

```bash
pnpm --filter @multica/core test collaboration/operation.test.ts collaboration/command.test.ts api/ws-client-account.test.ts collaboration/run-handoff-queries.test.ts
pnpm --filter @multica/views test collaboration/workflow.test.tsx collaboration/delivery-panel.test.tsx
pnpm typecheck
pnpm lint
```

后端从 `server/` 对 `internal/collaboration`、实际改动的 handler/service 包运行命中的测试和 `go vet`，有事务/并发改动时加对应 `-race`。不把 `-run` 当隔离手段，不默认触发真实Agent。

完成password API、production Web、独立部署/二进制绑定后，复用：

```bash
make env-exec ARGS="-- pnpm exec playwright test e2e/collaboration-multi-account-closure.spec.ts --workers=1 --retries=0"
```

全量测试按改动风险与仓库要求执行；记录实际命令、退出码、跳过原因和新旧失败归属。无新变化或疑点不重复扩测。

## 4. 各包完成证明

| 包 | 可关闭该包的证据 | 不允许的推论 |
| --- | --- | --- |
| S1 | 作者投影/兼容/权限回归、三会话当前版本证据、真实缺口报告 | 24用例全通过、支付修复、全部职业门禁已实现 |
| S2 | 审批矩阵正反路径，精确版本/撤权/回放，所有写入口无法绕过 | 已部署生产或业务支付已正确 |
| S3 | 故障注入原始失败与恢复结果、重复/丢失检测、竞争写入证据 | 真实模型可靠性和100用户性能已证明 |
| S4 | 沙箱支付/履约日志、仓库与构建身份、人工批准和实际部署/回滚证据 | 测试账号审批可代替正式产品签字 |
| S5/总验收 | 完整样本/性能分布、缺陷处置、完整24用例和角色验收结论 | 未测或部分验证可以折算为PASS |

## 5. 本轮验证

仅检查规划文件、CSV/JSONL结构、来源路径、用例完整性和任务planning状态。产品代码、测试、真实Provider及环境操作均未执行；此说明不代表产品验证通过。
