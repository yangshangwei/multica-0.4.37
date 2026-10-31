# 首轮修复交付续作：2026-10-31

确定性验证及正式 E0 已通过；E1 遭 Provider 上游 429/503 后超时，E1–E6 行为验收未完成。本子任务保留 `in_progress`，不将环境阻塞归档为实验成功。

## 基线与改动

- 产品集成源码：`4c833e737efb6c997e5cb56c74c43e9237c91956`。原 8 个修复、附件预算和迁移 746 已在此基线；本次没有改变产品业务代码。
- 实施分支：`codex/collaboration-context-resume-20261031`，独立 worktree `/Volumes/artisan/code/2026/mc-context-resume-20261031`。
- 实验框架提交：`b748535f0`，三个 `host_context_experiment*.go` tagged 测试文件。整体 SHA-256：`d5a682bac34e01ab5293fb372bae7d259413f420f151a85c231421146d7633db`。
- 修复两个 E2E 夹具的旧接收人输入框选择器，共用当前账号选择器 helper；补一条原生 Electron 定向信息返回测试。保留原有业务/权限断言，Web 仍拥有完整状态矩阵。
- 已找回此前用户对父计划和知识权限的明确批准，记录到父任务，避免重复询问。runtime 子任务细化了方案与验收，尚未实施产品功能。

## 验证结果

| 范围 | 结果 |
| --- | --- |
| 迁移 | prefix/direction 3 tests；fresh through 746；带历史数据 745→746；746 down/up/reapply，数据与 epoch 断言全部通过 |
| sqlc | 118 个生成文件无漂移；三个手写 companion 文件按文件头识别，不误作生成物 |
| Go | 当前 recipient service/handler 41 个顶层测试、165 个子测试通过，零跳过；新增框架 fake-provider、三轮/三组接力完整入口、source-root、常规 host 回归及 tagged vet 通过 |
| 前端静态检查 | 全仓 lint（已有 warnings，无 errors）、typecheck 通过；变动 E2E 的严格 TS/定向 lint 通过 |
| TypeScript 单元测试 | 1,050 文件、12,930 tests 通过；五个 Turbo 任务全部执行，零缓存 |
| 生产 Web | 定向信息交接与离线交接 2 tests 通过 |
| 原生 Electron | 定向信息返回与离线交接 2 tests 通过，零跳过/自动重试；包含接收方请求人/正文/默认状态和最终处理人 |
| E0 | 精确 `TestHostProviderAccountSmoke` 通过，34.57s；CLI 2.1.316，darwin/arm64；实际 launch/termination/proof、当前 CLI context read、outcome 与账本 ACK 均由测试断言 |

完整命令、环境、构建/测试 hash、SQL 断言和日志在 [确定性验证报告](research/2026-10-31/closure-verification.md)。E0 在 `research/2026-10-31/e0-attempt-01/`。这些检查不证明打包安装器或其他 OS 原生行为。

## 真实实验与失败分类

原预登记未改动，运行前补充见 `research/experiments-run-addendum-2026-10-31.md`。两个源码 arm 使用同一框架；E1 在原定 `c58f79874`，after 产品源码仍为已通过 E0 的 `4c833e737`。

| 项目 | 有效结果 / 无效尝试 | 证据与结论 |
| --- | --- | --- |
| E0 after | 1 次通过 | CLI 2.1.316；测试独占 Provider session 观察到 `claude-opus-5-5` |
| E1 首次预检 | 0 次模型调用 | Git 相对目录错误在版本探测前拦截；保留 `experiments-2026-10-31/` |
| E1 trial 1 | 0 个有效行为样本，1 次实际 Provider 尝试 | 读 context 1 次、memory 1 次；无 memory PUT、无最终答复；`claude timed out after 5m0s` |
| E1 剩余轮次及 E2–E6 | 未运行 | 完整框架在无效 attempt 后自动停止，未为凑齐结果继续消费账号额度 |

E1 的测试独占 Provider 日志显示 retry 1–7 中 **6 次 HTTP 429、1 次 HTTP 503**；不是旧 502 判断的沿用。分类为 `environment_provider_rate_limit_and_unavailable`。原框架的 `provider_failure_unclassified` 保留不覆盖，补充诊断在 `research/experiments-2026-10-31-series2/E1_memory_first-baseline/provider-diagnosis.json`。

同目录保存 source/frame/model 请求元数据、完整脱敏 fixture HTTP、host prompt/brief、原始 run/launch/termination/proof 和结果。模型观察来自该测试的私有 session 元数据；未拷贝私人会话正文。记录中的 51,141 input tokens、702 output tokens 是已观察 usage，费用未知，不估算成本。

## 框架复核与保留的失败

独立审查修复并验证了：未读取场景却计通过、E3 擅加只准一次 BETA、多个启动时 prompt 覆盖未知、Git 包目录误判，以及复用固定 run ID 导致第二轮/第二跳证据目录冲突。最后两项由完整入口 fake preflight 复现后修复。场景文本与原框架保持一致，E1/E4/E6 仍需语义裁决。

审查记录见 `research/harness-review-2026-10-31.md`。Go 退出码从不代替语义判定；单次 E0 通过也不等于后续模型供应商持续可用。

## 问题→证据→限制

| 原问题 | 本次闭合证据 | 剩余限制 |
| --- | --- | --- |
| A1 附件 runtime 预算 | 原最小修复和矩阵已集成，保留 2026-10-30 helper/service 证据 | 该选择器源码未在本次改动；不把旧证据改写成新运行 |
| A2/A3 评论字节与本地链接 | 原修复及回归保留；当前全量 TS 与相关原生流程通过 | 没有扩大原链接检测承诺 |
| B1 迁移重复前缀/回退缺证据 | 745→746、down/up/reapply、epoch/历史保留矩阵已完成 | 已发布迁移仍不可随意重编号 |
| B2 重新授权文案 | 已集成；当前全量 TS 通过 | Switch-only 边界不变；不声称所有机器组合已验证 |
| C1 成员步骤 | 原 role/step 修复在源码中 | E3 尚未运行；结构化派活/结果输入由 runtime 子任务承接 |
| C2 正式 smoke | 本次 E0 真实通过 | E1 后续供应商出现 429/503，不能推导稳定可用 |
| C3 接收方可见性 | 当前选择器 + Web/Electron request/note/status/handler 的实际验收通过 | 打包安装器、其他 OS 矩阵不在本次证据内 |
| 实验记录可靠性 | 持久化原始数据、完整入口 fake 复核和 E1 无效停止均可审计 | 真实 E1–E6 仍无有效行为结论 |

## 后续与工作区归属

1. 环境恢复后从缺失的正式实验继续，保留此无效 attempt 与原预测；不要重复有效样本来筛选成功，也不要擅自扩大超时或调用次数。
2. 按 runtime 子任务的结构化交接、冻结、wake provenance、checkpoint、runtime 分页和共享 UI 增量推进。其确定性契约与真实模型行为验证保持分开。
3. 知识候选权限已确认，产品实现仍依赖可靠来源/授权合同；不自动写技能。
4. 主工作区另有并行流程的 runtime 规划改动；它们不属于此次代码/证据提交，不能覆盖或代为提交。

验证使用的 API/Web/renderer 已停，两个测试独占数据库已无 FORCE 删除。原数据库、账号配置、未归属的文件及并行流程保留。
