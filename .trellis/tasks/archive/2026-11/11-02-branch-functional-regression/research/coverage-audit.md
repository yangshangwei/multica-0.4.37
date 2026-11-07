# 覆盖与证据对账审计（执行中快照）

审计角色仅读取代码、任务材料和公开证据，未运行测试、连接数据库、操作 API/服务、控制进程或读取私有凭据。本文件和 `report-draft.md` 是本角色仅有的写入范围。

E2E 对账快照：**2026-11-02T01:28:24.624198Z**。Go 整包派生使用同一阶段已完成的公开结果。主会话仍在执行后续任务，本审计不作完整验收判定。

## 已核对事实

### 库存身份一致

- `coverage.csv` 有 439 行、439 个唯一 case ID、439 个唯一 project/file/line/title 身份，涵盖 148 个 spec 文件。
- 与 `research/playwright-inventory.txt` 的 439 个收集身份逐项相等；所有文件存在。
- 默认项目名是 chromium；部分用例自行启动真实 Electron。项目名本身不能充当实际平台证据。

### 原始结果能映射库存

读取已完成 Playwright `run.json` / `results.json`，按 run 开始时间排序，在内存中重新对账，未执行或修改正式 reconciler：

| 口径 | 快照结果 |
| --- | --- |
| 已完成 Playwright run | 47 |
| 实际逐例执行数（含重跑与原始跳过） | 571 |
| 执行状态 | PASS 471 / FAIL 11 / NOT_RUN 89 |
| 按 439 身份的最新有效结果 | PASS 426 / FAIL 1 / NOT_RUN 12 |
| 身份未匹配或匹配不唯一 | 0 |
| 解析失败的 run 元数据 | 0 |
| 所有已完成 E2E 的业务候选 | `7af9ab9905c98c77bb091223ebe687c639475bb4` |

原样完整轮独立保留 **349 PASS / 1 FAIL / 89 SKIP，exit 1**。上表的最新逐例结果与原完整轮结论是不同口径，不能相互覆盖。

### 测试补丁与重跑可分辨

- 早期三个 E2E run 尚未单独记录 patch SHA；它们保留业务候选、隔离源码根、配置哈希、API 身份、Web Build ID、命令、时间与原始结果。
- 后续两份结果绑定初版交互补丁 `85b0ec…`；其后 42 份绑定加强后的 `6378d5…`。这不是 44 份独立业务版本。
- memory 第二用例行号由 108 移到 109、再移到 110。当前 reconciler 只对 `harness_dirty_paths` 中明确修改的同文件按唯一完整标题/project 回退，避免把任意行号变化静默视为同一测试。
- 最新 memory 补丁在展开历史后再次检查 Restore 不可用，修复了旧检查可在折叠状态下通过的证据弱点。原清除写、理由验证、版本、权限与窄屏断言保留；原两次失败仍可定位。

### Go 整包与单例分开

原 `go-result.json`：82 包，73 包含具名测试，九包无具名测试；五包失败；顶层 10,325 PASS / 23 FAIL / 35 SKIP，子例 10,487 PASS / 1 SKIP。

只将以下成功完整包证据替换对应包的首轮视图：

| 包 | 完整复验证据 |
| --- | --- |
| cmd/server | `evidence/go-cmd-server-replay/full/` |
| internal/cli | `evidence/go-home-replay/cli-full/` |
| internal/daemon | `evidence/go-home-replay/daemon-full/` |
| internal/daemon/execenv | `evidence/go-home-replay/execenv-full/` |
| internal/handler | `evidence/handler-full-freshfixture-1/` |
| internal/auth（Redis 条件开启） | `evidence/auth-redis-full-1/` |

按上述每包完整证据派生：**73 个含测试包均有成功完整包结果、九个无具名测试包；顶层 10,362 PASS / 21 SKIP / 0 FAIL，子例 10,487 PASS / 1 SKIP。** 不纳入单独五项 handler 成功或后续 targeted 条件补跑，不改写原首轮数据。Auth 首轮实际有 14 条 Redis 相关跳过，51 项全过关闭这 14 条。

handler 新整包实际为 2,863 PASS / 0 FAIL / 7 SKIP；五个原失败名称均通过，源码补丁与集成记录一致。独立审阅已给 PASS WITH NOTES，生产 150 秒门槛、授权与 stale/offline 负例保留。

## 收尾前必须处理的对账风险

### A1：汇总表落后于原始结果

快照时 `coverage.csv` / `e2e-summary.json` 仍是 **415 PASS / 2 FAIL / 22 NOT_RUN，559 次执行**；原始完成结果已推进到上述 426/1/12。两个旧 lane-close quota 失败已经补跑通过，最新单一失败变为 retained-attachment 第三例注册 429。

这是执行中账本滞后；正式报告应先重新对账，再固定报告快照。`password-batch/progress.json.failed` 保留历史失败 run，不代表其对应当前用例全部未解决。不要直接用该数组统计当前缺陷。

### A2：条件矩阵与文字进度有旧状态

- `conditional-matrix.csv` 的 `GO-HANDLER-FULL=FAIL` 尚未纳入已成功完成且集成的整包复验。
- `RUNTIME-LINUX=NOT_RUN` 尚未纳入 `password-21-collaboration-cross-client-execution-2` 的实际成功结果；更新时仍须限定为 namespace/测试 Provider，不等同 Linux 桌面安装。
- `progress.md` 的旧段落仍称 Go 测试文件未同步、密码批次暂停；实际集成记录及后续 run 已更新。收尾同步正式报告、进度、PRD 验收框与执行清单。
- 知识 OFF/ON、Mac 完整 Main、Linux 安装/升级等轴仍需各自实际结果；单一库存身份的 PASS 不能自动关闭多个部署/平台轴。

### A3：保留 handler 历史因果边界

旧失败行已由 TestMain 清理，不能重建选中 runtime 的原 `last_seen_at`。原首个实际测试时间与 runner 开始时间不同；Go 编译耗时不能算作 fixture 存活时间。

新运行第一条刷新前年龄为 **107.824 秒**，低于生产门槛，因此该日志没有直接观测到过期。当前整包通过证明补足合法在线前置的测试补丁可用，不能单凭它把旧五项失败全部定性为已证实的心跳过期，或将旧轮改判为通过。若再次发生，应采集失败瞬间的具体 runtime 行与队列/缓存状态。

### A4：memory 折叠触发原因仍未建立

原截图和 ARIA 明确显示历史收起，补展开后完整清除流程通过。产品组件本身没有“closed 改变便关闭历史”的直接 effect；祖先重挂载或导航/查询转换的具体触发尚未确认。报告应写观察到的状态及测试适配，避免称其为既定产品要求，或宣布已排除所有状态保留问题。

### A5：原始跳过须按原因保留

原 E2E 的 89 SKIP 是不同认证、执行门、构件与账号条件的混合，很多正在补跑。原 Go 跳过同时含平台分支、Redis、pg_bigm、性能/外网 opt-in、wire 导出与已有未实现断言。CLI 未知字段 round-trip、Webhook DB-error 注入等明确空白不能统一写成平台不支持，更不能记 PASS。

框架在 setup 阶段失败的用例没有完成业务断言。retained-attachment 快照最新 FAIL 是 `password registration failed: 429`，应保持原始失败，并在限流自然恢复后用新 run 复验，不能通过重置进程或阈值清零。

### A6：构件预检与旧结果不能代替新执行

复用包记录明确构建来源 `639bf6ef8`，三棵 tracked Git 目录与本候选一致且关键哈希匹配。该事实允许准备当前代码等价构件的本地验证，仍需新的实际 Main 运行。目录包、实际包管理器安装、升级、签名/公证、OS 原生会话分别声明；不得从旧 Mac/Linux 成功报告搬用结论。

### A7：清理与当前执行状态必须真实

本快照中未见最终统一清理记录。后续报告不能提前声明服务已停、执行门已恢复或任务可归档。密码专库的 namespace 门由 false→true 显式准备过，记录要求结束时恢复原布尔值、epoch 继续递增。保留失败库与日志，只处理本任务进程，不能停止最初不匹配的默认环境。

## 本快照尚未执行完的库存身份

`E2E-107`–`110`：Host installed/runtime/squad/switch；`E2E-112`：Linux full Main；`E2E-113`：部署知识开关；`E2E-156`–`161`：原生/网页 handoff、ENV-1、召回撤权、选择边界、源视图和关闭后重开。最新 FAIL 是 `E2E-155`（retained-attachment 第三例）。编号对应原始 439 库存，非新增需求编号。

## 审计结论

库存身份、现有原始结果与测试补丁之间没有发现不可解释的映射缺口；原失败与修复后成功均保留。当前不具备完整验收通过结论：主要待办是完成后续运行、刷新过时账本、逐项关闭条件轴并保存清理证据。此审计不修改正式账本，不批准旧 launcher、外部发布或任何真实 Provider 额度。
