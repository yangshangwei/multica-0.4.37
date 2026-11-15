# F3 验收矩阵：单一父任务预算根（observe 最小首包）

日期：2026-11-14 续写（文件名沿用 2026-11-13 规划批次，供既有链接解析）。状态：**仅 expected 与计划；没有任何一行已运行**。未获 F3 实施授权，本文件不是通过证据。

配套：[最小合同](f3-minimum-budget-contract-20261113.md)（下称“合同”）、[来源总表](f3-source-map-20261113.md)、[usage 分源](f3-usage-source-map-20261113.md)、[执行/claim 分源](f3-execution-source-map-20261113.md)。需求编号 F3-01–F3-14 定义在 [requirements.md](../requirements.md) 的 F3 节。

## 1. 读法与证据规则

- 金额单位一律 `tick = 10^-10 USD`，下文数字均为 ticks 的手算值。oracle 由本表手算给出，不得由被测 reducer/helper 的输出反推，也不得用“两侧调用同一 helper 相等”当证明。
- 记号：`A`=actual_known，`E`=estimated，`R`=reserved，`H`=unknown_hold，`U`=unknown 集合（带原因），`L`=limit，`rem`=remaining。`rem=null` 表示不可给出数值。
- 层：`pure`（无 DB 的 reducer/编码/价格计算）、`db`（任务自建数据库上的事务与并发）、`auth`（真实 Auth/handler/router 链）、`claim`（真实 claim/finalize 事务）、`daemon`（测试自建 fake 可执行文件 + 本地 journal，不解析或执行用户安装的 agent CLI）、`cli`。
- S1 行是首包验收；S2 行是未来 enforce 的预先登记，**不属于首包出口**，只用于防止 S1 的数据形状堵死 S2。
- 每个 DB package、normal 与 race 各用新建环境，保存精确用例清单、源码身份与零 skip 证明（参照 F2 的做法；DB 不可达时 `ok` 不算通过）。
- 禁区不变：不调用真实 Provider、不操作 POLA-1、不改共享项目策略、不做账单对账、不发布/部署/Hosted CI。

## 2. 需求 → 用例映射

| 需求 | 内容 | 用例 |
| --- | --- | --- |
| F3-01 | 一个普通父任务终身至多一个根；并行建根只成一个 | B01 B02 B03 |
| F3-02 | 子孙在创建/绑定事务继承根，归属持久保存 | B04 B05 B06 |
| F3-03 | 重试、重领、重开回到原根且不重置累计 | B07 B08 B09 |
| F3-04 | 移动、改父、换项目、删除不退款、不换根、不双归属 | B10 B11 B12 B13 B14 |
| F3-05 | 父层汇总与既有聚合不产生第二笔消费 | B15 B16 |
| F3-06 | A/E/R/H/U 分离；unknown 不折 0，不无证据释放 | B17 B18 B19 B20 B21 |
| F3-07 | 整数计量、十进制字符串、冻结价格版本 | B22 B23 B24 |
| F3-08 | 累计/增量、重复、迟到、更正 | B25 B26 B27 B28 B29 B30 |
| F3-09 | 整批原子收据；原 key 恢复；commit 未知不另开 | B31 B32 B33 |
| F3-10 | 影子预留的建立、转移、释放与取消/超时/失联 | B34 B35 B36 B37 B38 |
| F3-11 | 当前权限、撤权、旧版本拒绝、最小披露 | B39 B40 B41 B42 B43 B44 |
| F3-12 | observe 不改业务执行结果；记录 would_refuse | B45 B46 B47 |
| F3-13 | 覆盖声明：not_applicable / coverage_gap / pre_activation_unknown | B48 B49 B50 |
| F3-14 | 兼容、回退、无 FK、并发索引、F2/M1 不变 | B51 B52 B53 B54 |
| S2 预登记 | 强制准入、无上界拒绝、旧 daemon 拒发 | E01–E06 |

## 3. 根与成员（F3-01–F3-05）

| ID | 层 | 场景 | Expected |
| --- | --- | --- | --- |
| B01 | db | 同一父任务两个事务并行启用根 | 恰一个 `budget_root_id`；另一方返回同一根（同 key 同意图）或 conflict（不同意图）；不存在两行 |
| B02 | db | 父 P 与其子 C 并行各自启用根 | 成员 fence 下串行：后提交者发现 C 已在 P 的成员范围或 P 已含有根成员 → `root_conflict`；绝不出现 C 同属两根 |
| B03 | db | close 后再次“新建”根；父删除后对同一父或其已绑定成员再建根 | 拒绝；身份槽终身占用；reopen 只能是同一根且 policy revision 递增、A/R/H 原值保留 |
| B04 | db | 已有根的 P 下新建子 C1，再建孙 G1 | C1、G1 在各自创建事务内写入成员行，继承来源与绑定版本可读；不依赖之后读取 `parent_id` |
| B05 | db | 启用根与“新建子任务”并发（phantom） | 无论先后，子任务最终恰有一条指向 P 根的成员行；不存在“已建子任务但无成员且无 gap 记录” |
| B06 | db | 启用时 P 已有 3 个子孙、其中 1 个有在途旧协议 task | 3 个成员一次冻结；在途 task 记 `pre_activation_unknown`（原因=in_flight_legacy），启用后 A 从 0 起但终身总额标 unknown |
| B07 | claim | 同一 issue 服务端重试产生新 task UUID | 新 attempt、同根；旧 task 的 A=40 保留，新 task 报 25 → A=65；L 不变 |
| B08 | claim | finalize 失败后同 task 重领（generation+1） | 旧 attempt 的 R 不因重领消失：有“未派发证明”才释放，否则转 H；新 generation 得新 attempt |
| B09 | db | 业务状态 done → 重开 → 再执行 | 根状态与 L、A、R、H 均不因业务重开变化；不产生 reopen 事件 |
| B10 | db | 成员 C 清空 parent / 改到无根的 Q 下 / 换项目 | 成员行不变，之后执行仍计入原根；读取显示“当前位置≠预算归属”；A 不减少 |
| B11 | db | 成员 C 改到另一个有根 R2 的父任务下 | 管理操作拒绝矛盾绑定；业务改父本身 S1 不被阻断，但记录 `would_refuse:root_conflict`；C 的支出只进原根，R2 的 A 不变 |
| B12 | db | 删除成员 C（A=30, R=20 未决） | 留 tombstone；根 A 仍含 30；R=20 不释放，按 B36/B37 规则收敛；C 的子孙仍归原根 |
| B13 | db | 删除父 P | 根 `tombstoned`；账目与成员保留；之后旧链执行继续记入并标 `would_refuse:root_deleted`；不能为存活成员另建根 |
| B14 | auth | 另一工作空间的管理员用原 key / root id 读或写 | 404 等价的最小拒绝；不泄漏根存在性、金额、成员 |
| B15 | pure | 子调用 s1=40、s2=60；父层上报来源证明为 rollup=100；父自身独立调用 p=15 | A=115；rollup 只作对照事件、贡献 0；若 rollup 来源不能证明是汇总 → 进入 ambiguity，不计入 A，U 增加一项 |
| B16 | db | 同一消费同时存在于新事件、旧 `task_usage` 行与 hourly 桶 | 根投影只读新事件；旧行/hourly 的任何重算、删除、改归属不改变 A |

## 4. 计量与占额（F3-06–F3-07）

| ID | 层 | 场景 | Expected（手算） |
| --- | --- | --- | --- |
| B17 | pure | 预留 100；partial actual 30 | A=30, R=70, H=0；非 A=30,R=100 |
| B18 | pure | 接 B17，随后失联 | A=30, R=0, H=70；总额 unknown；`rem=null`；不得输出“已知实际=100” |
| B19 | pure | 接 B18，最终可信证据：总成本 80 且无未报调用 | 追加 +50 → A=80；H 终结；释放 20；R=0,H=0。同一收据重放：A 仍 80，不再释放 |
| B20 | pure | 接 B18，最终证据总成本 120 | A=120, overrun=20, H=0；不裁剪到 100，不拒收 |
| B21 | pure | 两次调用：c1 token=100 费用缺失；c2 token=50 费用=30 | A=30；U 含 c1（原因=cost_absent）；不得显示“全部已计费”；与现有 fresh-fallback 合并行为的反例对齐（usage 分源 §5） |
| B22 | pure | 金额 presence：缺失 / 明确 0 且有来源依据 / 0 无依据 / 负值 / 币种非 USD | 分别为 unknown(cost_absent) / A+=0 且标 known_zero / unknown(zero_without_basis) / 拒绝 / unsupported_currency；五者不合并 |
| B23 | pure | 估算：input 1,000,001 token，冻结费率 3 USD / 1,000,000 token | 精确值 30,000,030,000 ticks；预留向上取整后仍 30,000,030,000；改为费率 1/3 USD 每百万、token=1 时精确值 3,333.33… → 预留 3,334，E 的展示值按冻结舍入版本固定且可重放；全程无 float64 |
| B24 | pure+db | 价格表升版后重放旧事件；调用方自报更便宜的 price version | 旧事件的 E 与原值逐位相同；自报版本被忽略或拒绝，服务端版本生效；实报 A 不随改价变化 |
| B24a | pure | 对外编码：`"9223372036854775807"`、`"1e3"`、`"1.5"`、`-1`、JSON number `9007199254740993` | 第一个接受（边界内）；其余拒绝；A+ 增量溢出 int64 时拒绝整批而非回绕 |

## 5. 事件语义与恢复（F3-08–F3-09）

| ID | 层 | 场景 | Expected（手算） |
| --- | --- | --- | --- |
| B25 | pure | cumulative 同 scope：v1=100, v2=150 | 贡献 100 后 +50，A=150；两份原始快照都保留 |
| B26 | pure | 重发 v2=150（同身份同内容）；随后迟到 v1=100 | 原收据；A=150 不变；迟到项记为 duplicate/stale，不倒扣，不按到达时间排序 |
| B27 | pure | v3=130，无更正依据 | 隔离；A 仍 150；U 增加 `sequence_regression`；不取 max 宣称可信 |
| B28 | pure | v3=130 且显式 supersedes v2、来源有更正权、链匹配 | 追加 −20，A=130；100/150 原始记录仍可读；同更正重放不再 −20 |
| B29 | pure | delta：d1=40, d2=60, 重复 d1；scope 内混入 cumulative 事件 | A=100；混入事件拒绝（mode 在 scope 内固定） |
| B30 | pure | 同一 provider session 跨两个 invocation，累计 80 → 200；基线可证 / 基线缺失 | 可证：i1=80、i2=120，合计 200 只入一次；缺失：i2 记 ambiguity，A 只含 80，U 增加一项，不把 200 记两次也不猜 |
| B31 | db | 一批 3 个事件，第 2 个违反约束 | 整批不提交、无“全部成功”响应；与现有逐行 warn 后 200 的行为相反，回归用例显式对照 |
| B32 | db+auth | 提交成功但响应丢失；以原 key 重发相同内容 / 不同内容 | 相同 → 原收据且 A 不变；不同 → conflict，A 不变 |
| B33 | db | commit 结果未知时查询：返回 issued / pending / 查无 | 均显示 unknown 并给原查询方式；不据此新建根、attempt、invocation 或 key |

## 6. 影子预留生命周期（F3-10）

| ID | 层 | 场景 | Expected |
| --- | --- | --- | --- |
| B34 | claim | 领取最终化成功 | attempt + 影子 reservation + 原操作收据与 claim 同事务提交；任一写失败则该领取失败并单列为持久化失败，不记为“额度不足” |
| B35 | claim | finalize 响应丢失，daemon 重领（generation+1）并重新 finalize | 现状没有“按原 claim 查询”，重领会重新发 token（合同 §11 R2）。Expected：产生新 attempt 与新影子预留；前一代 attempt 标 `superseded_unproven`，其 R 转 H 而不释放（原 daemon 可能已启动）；两代 attempt 用服务端签发的 `budget_attempt_id` 区分，回调不带该 ID 时记 `coverage_gap:attempt_unbound` |
| B35a | claim | finalize 事务失败后原行 requeue（未写出任何响应字节） | 预留与 attempt 随同一事务回滚，不留孤儿；之后的领取是第一个 attempt |
| B36 | db | 取消 / 超时 / 进程退出，仅有生命周期事实 | R 转 H；U 增加对应原因；不释放。附带“可信最终用量 + scope 完整 + 不可再执行证明”时才结算并释放余额 |
| B37 | db | 仅有：队列行消失、HTTP abort、TTL 到期、当前运行数=0、重新 claim | 逐项断言均**不**释放；H 保持 |
| B38 | db | 最终化/消费与取消并发 | 同一 reservation 恰一个终结贡献与一份收据；取消不覆盖已消费事实；重复执行结果相同 |

## 7. 权限、版本与披露（F3-11）

| ID | 层 | 场景 | Expected |
| --- | --- | --- | --- |
| B39 | auth | 建根/调额/close/reopen：owner、admin、普通成员、mat_ task token、daemon 凭据、携带 human ID 的机器凭据 | 仅前两者且具备当前根任务编辑权时成功；其余拒绝且无副作用 |
| B40 | auth | 调额携带过期 `expected_policy_revision`；mode 关→开后复用旧 epoch | 冲突拒绝；不取最新值代为续权；epoch 不复用（防 ABA） |
| B41 | auth | 操作者在提交前被降级 / 移出工作空间 | 事务内复核失败 → 拒绝；先前已提交的原操作仍可被当前有权者按原 key 读到，并标示 stale |
| B42 | auth | invoke 被撤后：新预留请求 / 已发生调用的晚到用量 | 前者拒绝；后者在 telemetry 身份仍有效时入原根；daemon 也被撤 → 拒绝，对应金额保持 unknown |
| B43 | auth | Reporter / Reviewer / custom task / mcn_ 读根、列根、读收据 | 全部拒绝；F2 exact GET、人工采集、终止撤回的既有用例结果不变 |
| B44 | auth | 鉴权失败、404、冲突的响应体与头 | no-store；不含隐藏根、其他任务、provider 会话或金额；源删除后只返回最小经济 tombstone |

## 8. observe 边界与覆盖声明（F3-12–F3-13）

| ID | 层 | 场景 | Expected |
| --- | --- | --- | --- |
| B45 | claim | L=100，A=90，两个执行并发各影子预留 10 与 10 | 根行串行：两者都被记录、业务执行都继续；恰一个标 `would_refuse:limit`；S1 的 task 状态、调度顺序与未启用预算时一致 |
| B46 | claim | 根 closed / tombstoned / 存在无上界 unknown 时到来的新执行 | 执行不受阻；各记一条带原因的 would_refuse 观察；`rem=null` |
| B47 | db | 未启用根的任务树；预算配置损坏 | 前者零预算写入、行为与现状逐项一致；后者是显式错误而非静默 off，也不阻断业务执行 |
| B48 | pure | 根范围内同时存在：run_only autopilot、独立 collaboration/host run、插件外部调用 | 列为 `not_applicable`（机制级排除，带名称）；不计入 coverage 分母，也不显示为 0 成本 |
| B49 | daemon | 纳入执行内一次调用没有可辨认的用量（适配器不支持） | `coverage_gap` + U；根标“总额 unknown”；不得因排除而显示 complete |
| B50 | cli | `show` 输出 | 已知 A、E、R、H 分列；U 逐原因计数；`pre_activation_unknown` 与“启用后已知值”分开；`rem` 在有 unknown 时为 null 而非 `L−A` |

## 9. 兼容与不回归（F3-14）

| ID | 层 | 场景 | Expected |
| --- | --- | --- | --- |
| B51 | db | schema 检查 | 新表 0 个 FK / cascade；每个索引独立单语句 CONCURRENTLY 迁移；条件对象用幂等 DDL |
| B52 | db | 关闭观察开关后 | 停止新观察；既有账目与原 key 只读仍可用；金额不降级为 0；共享项目策略零写入 |
| B53 | daemon | 旧 daemon（无预算协议）执行纳入任务 | 执行照常；该执行记 `coverage_gap:legacy_protocol`；旧 `task_usage` 上报路径与现有用量页面行为不变 |
| B54 | auth+cli | M1-S1 完成授权与 F2 报告快照的既有具名用例 | 结果与 `463b201fa` / `fafb27b1d` 验收记录一致；F3 不新增对它们的写路径 |

### 9.1 源码复核后新增（合同 §11）

| ID | 层 | 场景 | Expected |
| --- | --- | --- | --- |
| B55 | claim | quick-create task（`issue_id` 为 NULL，`context.parent_issue_id` 指向已绑定成员）执行并在完成后 LinkTaskToIssue | 若 D6 选“纳入”：finalize 时按服务端保存的 context 父任务归入原根，新建的子任务在其创建事务内成为成员；若 D6 选“不纳入”：列 `not_applicable:quick_create` 且该次支出不显示为 0。两种选择都不得出现“子任务已是成员但创建它的那次执行无记录也无缺口” |
| B56 | db | 删除有 A=30 的成员 issue，现有 FK 级联物理删除其 `agent_task_queue`、`task_usage`、`task_token` 行 | 预算的成员、attempt、事件行全部仍在（它们不带 FK，保存的是 ID 副本）；根 A 仍为 30；取消 task 先于删除事务提交这一顺序不产生释放 |
| B57 | db | 删除父 P，直接子任务被 `DetachDirectChildIssues` 清空 `parent_issue_id` | 子任务的成员行仍指向 P 的根并保留原父 ID；之后执行归原根 |
| B58 | auth | 预算管理请求的 `X-Actor-Source`：人工会话、个人 PAT、`task_token`、`cloud_pat`、`mdt_` daemon 凭据、未知/缺失来源 | 采用允许名单：只有明确识别的人工来源通过；未知与缺失来源拒绝（现有 `RequireHumanActor` 对未知来源放行，不可直接复用） |
| B59 | db | 启用根时成员范围深于 10 层；范围内存在环 | 事务内递归遍历不受现有 10 层查环上限限制，深层成员全部绑定；发现环则拒绝启用并给出原因，不写入部分成员 |
| B60 | db | 未取工作空间 fence 的 issue writer（待 E2 清点：autopilot create_issue、onboarding、协作结构 writer、工作空间拆除）在已有根的父任务下创建子任务 | 清点完成前，这类路径创建的子任务若未绑定，必须被对账发现并记 `coverage_gap:unfenced_writer`；清点完成后改为逐路径断言已绑定 |

## 10. S2 预登记（不属于首包出口）

| ID | 场景 | Expected |
| --- | --- | --- |
| E01 | L=100，A+R+H=90，并发两个各需 10 的准入 | 恰一个成功，另一个 `budget_exhausted`；任何交错下 A+R+H ≤ L 于准入时成立 |
| E02 | 存在无上界 unknown 或只有估算“上界” | 拒绝新准入并给原因；不以 `L − known A` 放行 |
| E03 | 旧 daemon / 不支持预算协议的启动入口 | 不下发 enforce launch；全部 claim 入口与 provider 启动入口有无绕路证据 |
| E04 | 已发 launch 后撤权、关闭根、改 policy | 新调用许可被拒；已发生调用的费用仍按原根收敛 |
| E05 | 启用前义务（pre_activation_unknown）未处置 | 不得进入 enforce；禁止删根重建换起点 |
| E06 | 提供方无硬封顶 | 文案与接口只承诺“准入时未超发”，不承诺最终 actual ≤ L |

## 11. 首包出口与当前状态

首包出口 = B01–B60 全部具名通过（normal 与 race 分开）+ 合同 §10 的 E1–E4 证据 + 独立 check 复核。E01–E06 与合同 E5 属于 S2。

当前状态：0 / 62 行运行（B24a、B35a 各计一行）。本表只冻结 expected；任何一行在实施中发现 expected 本身有误，应回到本文件修订并记录原因，不得改实现或 fixture 去迎合旧数字。
