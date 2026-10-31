# 协作上下文交接、召回与沉淀实施计划

> 用户已确认本计划、权限边界及六步推进顺序（见 prd.md）。首轮确定性与 E0 门槛已经核验，当前执行 runtime 集成；模型行为结论保留未完成状态。

2026-10-31 续作以各子任务的最新清单和证据为准。下述 745 迁移、8 个修复集成等步骤保留原始顺序，已完成项不要重复实施；可靠交接的细化合同见 `../10-30-collaboration-context-handoff-runtime/{design,implement}.md`。

## 当前六步推进账本

1. 已记录 `e3620721e` 上的 runtime WIP 快照、各文件 hash 和负责人；隔离集成目录为 `/Volumes/artisan/code/2026/mc-runtime-integration-20261031`。快照是未验收输入，不能标为已交付。
2. runtime 主流程和启动注入流程分别完成有边界的交接，由本会话增量集成；路由、客户端、协议、实际启动输入、human API/query/view 和共享文件冲突逐项核对。
3. 先完成缺失的协议、checkpoint、recall、handoff 和 UI 定向测试，再运行 sqlc、Go build/vet、lint/typecheck/test、独立复核和生产 Web/原生 Desktop。service/handler 全套在同一测试 DB 上串行，浏览器 DB 与 Go DB 分离。
4. 真实模型在编码工作停止且上游恢复后独立串行运行。原实验继续 E2a trial 2、剩余 23 次；新 runtime 的真实证据单独绑定新源码和登记，不能挪用旧实验结论或预算。
5. 交接来源/版本/授权契约稳定后完成知识候选设计及实施：人工确认、纠错、过期、撤回和后续注入过滤。当前可做只读前置研究，暂不启动候选写入代码。
6. 按父任务 AC0–AC9 审核证据并完成任务状态。Windows、安装包与多账号额度溯源属于关联交付矩阵；未具备证据时保留明确缺口，不默认发布。

当前各 lane 的来源、快照、交接与验证保存在 runtime 子任务 `research/integration-coordination-2026-10-31/`，外部运行中的文件不得整文件覆盖。

## 执行顺序和依赖

### 0. 创建隔离交付环境和证据账本

Owner：首轮收尾负责人。依赖：规划批准。

1. 从待交付基线创建独立 worktree，记录基线 SHA、工作区状态、Go/Node/pnpm/sqlc/CLI 版本。
2. 将主工作区未提交改动视为外部输入；不 cherry-pick 或 reset 主工作区。
3. 建立任务内 evidence/runs/timestamp，保存 stdout、stderr、exit code、HEAD、环境摘要；禁止以 /tmp 作为唯一证据。
4. 修正 10-30-collaboration-fix-closure 的 PRD、design（如需要）和 manifests 后再 task.py start。

回退点：删除新 worktree，不接触主工作区。

### 1. 先修迁移命名并验证数据库迁移

Files：server/migrations/745_collaboration_owner_reconsent.*.sql（改为空闲前缀）；相关迁移测试/文档引用。

1. 运行 go test ./internal/migrations -run '^TestMigrationNumericPrefixesStayUniqueAfterLegacySet$' -count=1 -v，确认当前失败证据。
2. 仅在迁移未发布的前提下改为下一个空闲数字；若任何部署已记账，停止重命名，改写前向幂等迁移并保留历史 basename。
3. make sqlc，检查 git diff --exit-code -- server/pkg/db/generated server/pkg/db/queries；若 SQL 变化确实需要生成，提交 generated 产物。
4. 对空库、已有旧 schema、同版本重复 up/down 分别运行实际 migration runner；核对列、trigger、ledger、schema_migrations。两个 745 不作为“迁移被跳过”，但 lint 必须绿。

验收：迁移 lint PASS；fresh/upgrade/reapply 均有数据库名、HEAD、输出；没有外键/cascade/非 concurrently index 违规。

### 2. 修复附件选择的实际机器预算

Files：server/internal/service/collaboration_runtime_preparation.go、相关 query/协议 helper、collaboration_runtime_preparation_test.go、.trellis/spec/server/collaboration-preparation.md。

1. 把选择函数参数改为实际 runtime limits；先解析、去重并验证所有引用附件。
2. 若任何引用超过 FileCount、PerFileBytes、TotalInputBytes 或 workspace/Git 保留后的空间预算，返回 runtime_preparation_limits，不裁引用、不预约、不 claim。
3. 对未引用已交付附件按既有新到旧排序逐个尝试；只有不超过所有剩余预算才补入；可选历史文件可全部省略，不能阻塞纯文字。
4. promotion、claim、freeze、never-launched refusal 共用同一选择器；小队成员按各自 runtime 能力计算。
5. 保留“清单外附件不可读”和首次 claim 冻结。

测试先行：添加小 runtime 上限、总量、单文件、Git 保留、引用超限、引用丢失、小队不同机器矩阵；先运行失败，再实现。

验收：runtime_preparation_limits/unavailable 原因稳定、无半截副作用；原 33+历史回归仍 PASS。

### 3. 收尾 B/C/A 定向实现与 UI 证据

依赖：1、2。

1. 按 B 报告修正 config invalid 的文案/动作映射，优先复用 server can_consent 和不可用原因；不改变旧目标 Switch-only、旧 run、hold、explicit Resume 屏障。
2. 按 C 报告在已有接收方 handoff e2e 的 return 分支增加正文/请求人可见断言；已有 component 测试只增加正常 status select 值，不复制业务矩阵。
3. 保留 role/step、authorizer、local-path、escaped-comment 的现有回归；补服务层 64 KiB+1 不落库。
4. A/B/C 各自复审最终 diff，结论保存到任务 research。

验收：A/B/C 报告均为独立通过或明确剩余限制；Web/desktop 共享组件与 desktop 原生入口的证据分开。

### 4. 最终 HEAD 交付验证

依赖：1–3。按顺序运行：

    go test ./internal/migrations -run '^TestMigrationNumericPrefixesStayUniqueAfterLegacySet$' -count=1 -v
    make sqlc
    git diff --exit-code -- server/pkg/db/generated server/pkg/db/queries
    go vet ./...
    go build ./...
    go test ./cmd/multica/ -count=1
    go test ./internal/daemon/collaborationrun/ -count=1
    go test ./internal/service/ -run 'Collaboration' -count=1 -timeout 40m
    go test ./internal/handler/ -run 'Collaboration' -count=1 -timeout 40m
    pnpm lint
    pnpm typecheck
    pnpm test

随后按项目环境运行 make check 或等价拆分，包含生产 Web + Playwright；所有 DB 测试必须确认测试名确实出现，不能接受不可达数据库静默 exit 0。

验收：每一项记录实际 HEAD、数据库、环境、命令、测试名、exit code；出现失败继续修复或记录阻塞，不提前进入付费实验。

### 5. 硬化实验框架（不消耗模型额度）

Files：mc-context-exp/server/internal/daemon/collaborationrun/host_context_experiment_test.go，迁移到交付任务的 research/experiment branch 后再审阅。

1. 运行前补充登记，不改原预登记预测；提交实际 source HEAD/framework hash/model/provider/CLI/config/thinking/service tier。
2. 让框架从构建/环境读取并校验 commit，不接受手写伪造；固定 MULTICA_CONTEXT_EXPERIMENT_TRIALS=3，E6 按三对接力计。
3. 保存完整实际 prompt/brief hash+受控原文、顺序 request/response（不存 Bearer）、status、run/request ID、outcome/proof、tool call/result；保留 provider record 的 partial 标记。
4. 每个 trial 写 durable started/finished/incomplete 记录，失败 verdict 非零；评分规则预先写入 run manifest，不能靠 go test PASS 代替。
5. 用 fake provider/HTTP fixture 验证 E1–E6 记录格式、stale merge、权限拒绝、截断和中断分类。

验收：假 provider 测试能区分 environment/product/model failure；正式 smoke 仍只在双 gate 下运行。

### 6. 正式 E0–E6 串行实验

依赖：4、5；用户已有授权，仍需在已确认计划后执行。

E0 前置：

    cd server
    MULTICA_RUN_REAL_AGENT_SMOKE=1 \
    MULTICA_HOST_SMOKE_PROVIDER=<claude|codex|opencode> \
    MULTICA_HOST_SMOKE_EXECUTABLE=/absolute/provider \
    go test -tags=agentintegration ./internal/daemon/collaborationrun \
      -run '^TestHostProviderAccountSmoke$' -count=1 -v

成功证据必须包含 provider/model/CLI 版本、实际 launch、termination、business context read、outcome、ledger ACK。401/429/503/CLI 无输出/transport 中断单列环境记录；不把无效样本算成功或失败。

E1–E6 严格 E1 → E2a → E2b → E3 baseline/after → E4 → E5 → E6，单场景单 trial，不与其他模型工作并行；E1/E2a/E2b/E4/E5 各 3 次，E3 前后各 3 次，E6 三对两跳。保存原始目录和最终汇总，任何 E5 canary 命中阻塞发布。

### 7. 建立首轮问题映射和合入门槛

生成 research/problem-fix-evidence-matrix.md。问题映射必须包含：

| 问题 | 修复提交/后续改动 | 证据 | 剩余限制 |
| --- | --- | --- | --- |
| A1 附件预算 | 7bb077987 +预算选择器 | 定向 Go/DB、最终集成 | 未引用历史可省略；每 runtime 独立预算 |
| A2 JSON 转义 | d84f217a9 | parser+service+integration | 不增加解码后内容上限 |
| A3 local link | b34701756 | pre-HTTP CLI test | 只覆盖协作 run |
| B1 migration | migration rename | lint+fresh/upgrade | 已发布 migration 不可重命名 |
| B2 reconsent | 0ef4c7858, 4c2140b54 | authority/UI/old token | NULL boundary/部分配置仍 Switch |
| C1 role/step | e6ed3d1c3 | prompt/service/e2e | delegation 正文和模型遵从需 E3 |
| C2 smoke | fd3836da2 | authorizer + exact E0 | 账号/额度环境可能阻塞 |
| C3 handoff form | 4b0cbfde7 | component + recipient e2e | Desktop 原生证据单列 |

合入门槛：8 提交在最终分支；A1/B1/B2/C3 的必要收尾完成；migration/sqlc/Go/TS/e2e 证据齐全；E0 通过且 E1–E6 有有效结果，或明确环境阻塞且不宣称产品通过；主工作区改动保持原样。

### 8. 后续子任务：可靠交接与召回

依赖：首轮合入门槛。按 design.md 实现并分小提交：

1. checkpoint event 与撤权/停止 race、幂等、大小/数量预算；
2. handoff envelope 的 handler/role/intent/final answer/member result/readers/wake causes；
3. delegation 正文、mention 规则和前序结果结构化输入；
4. 评论 cursor/total/truncated/bytes/author capability；
5. prompt trust boundary 和 malformed/schema/old client 兼容；
6. Web/Desktop handoff、截断提示、来源/版本/验证指针。

每步先写纯函数/Go service 回归，再做 DB/handler，最后做 Web/Desktop/e2e；无真实模型依赖的契约先完成。

### 9. 后续子任务：知识候选逐层沉淀

依赖：R1–R4 的 handoff envelope 和用户确认的目标权限。顺序：

1. 候选状态/来源/适用范围/验证/版本/纠错/过期/撤回数据契约；
2. issue-scoped propose API 和权限矩阵；
3. project/workspace confirm API，写入现有 revisioned context，保存 candidate provenance；
4. UI 审阅、差异/来源/撤回/过期显示；
5. 目标层和来源权限变化的后续注入过滤；
6. 技能层只写 ADR/研究，不接自动写入。

## 失败恢复和回退

- 迁移/SQL 错误：停止后续验证，保留旧迁移文件和失败日志，使用新前向迁移；不 force reset 已发布 schema。
- 附件选择器错误：关闭新补位 feature flag，保留引用附件拒绝和旧清单历史；不得静默删引用。
- 实验环境失败：保存完整 invalid attempt，停止付费重试；不修改预登记预测。
- handoff/candidate 新路径错误：关闭对应 feature flag，旧 comments/memory/run read path 继续工作；不删除历史。
