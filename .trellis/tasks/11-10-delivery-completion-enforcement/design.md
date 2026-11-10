# 完成门禁技术设计

## 单一事务核心

采纳 research/transaction-design-20261111.md 的 service 直接调用方案：将现有唯一观察器及制品当前权限/凭据校验移到 server/internal/service；保留 RuntimeLookup 指标归因，不增依赖/第二观察器/handler 反向依赖。不使用全局可选注入，避免 91 个协作构造点遗漏。

HTTP 继续拥有严格 JSON、路径解析与真实 header deny-only 检查；核心接收来自实际认证上下文的显式凭据值，issuer 和 consumer 分型。原 artifact/hint/evaluation 使用该唯一核心，保留旧返回、私有 namespace、历史不变及 commit 后才成功。

PrepareDeliveryMutationInTx 借用原 tx，在已持有的 catalog/issue 等原锁之后进行当前来源/目标项目、政策、授权、双方主体/凭据、制品/PR 锁与绑定检查，不写业务状态。FinishDeliveryMutationInTx 绑定同一个 tx，复核数据库时间与实际 after，消费授权并记录原完成回执；错误必须回滚整个 writer。服务不得 Begin/Commit/Rollback/retry 调用者事务。schema 缺失、未知 mode、nil 必需依赖拒绝。

## 显式授权和恢复

新授权是人类的明确验收动作，以固定观察器作一次新实时验证；不把原 evaluation key 当许可。授权创建用原人类+workspace+UUID key 幂等，绑定完整 request digest；冻结 issuer 当前真实 credential metadata（无明文 JWT/PAT）、目标/项目/政策 revision、目标状态、精确证据、消费者、期限与完成合同版本。保留的 issuer 快照只能从锁定服务器记录读取并显式校验，不构造认证上下文冒充其身份。

当前 human/task/webhook/admin 消费者使用本次实际认证及原 writer 权限，不能因授权扩大角色。普通完成附授权 ID、原操作 UUID、expected revision；原 consumed 请求在完整当前访问权下先恢复对应回执，不能滑入 done→done 改字段。改变原意图冲突；未知结果只 GET 原记录，无接管、自动重发或新 key。恢复只展示最小完成事实，不披露 issuer 私有 observation 或 credential metadata。

未消费授权可由原 issuer 或当前源 workspace owner/admin 有理由撤销，撤销同样受当前权限约束；已消费不可撤销成未完成或再次消费。授权期限取显式请求、24h、证据及 issuer 有限凭据期限的最早值。旧凭据撤销/密码或 binding 版本变化使未消费授权不可用，renewed credential 不改写旧快照；读取原历史事实与新准入分离。

精确字段、SQL 和 public error shape 在各实施包冻结接口中记录；禁止引入万能 JSON scope 或字符串 system 身份替代明确的 union。

## 完成对象和项目变更

采纳 D4a/b：严格解析原持久状态和新 catalog 目标；仅非 done→done 是新完成。enforce 新完成允许实际状态及可选排序变化；其他有效变化先保存再取证，实际相同字段不拒绝。done 后非项目编辑不重验、不改历史。

来源/目标 project 来自锁定 before/candidate，按稳定顺序 NOWAIT 锁定；移出 enforce/解绑必须真实当前 owner/admin 本人及理由并持久留证，仍叠加原结构权限。任一侧 enforce 的已 done 必须先重开，迁移与完成分开。CreateInTx 在父项目继承后查实际政策并拒绝 enforce done；project 删除的原锁内拒绝当前 enforce，需事先显式 CAS off，原含协作 issue 拒绝保留。

## 协作、R38 和源适配

现有协作 run 没有普通 agent_task_queue，不能给它伪造普通 task；复用 R51 合法普通 child 路径。授权同时绑定目标 parent/协作 revision/lifecycle、同源同实际项目的 exact ordinary child/revision/parent 关联及其真实 artifact。调用同一个 observer 验证 child 制品，明确它没有变成 parent 制品。issuer 需要当前源制品读取权及目标协作交付意图权限，跨账号 consumer 仅保留协作权限。

外部交付绑定 exact pending available submission；self variant 冻结结果/依据/附件/源评论/draft 规范摘要；admin variant 冻结真实 admin/org/原 key/目标/status/理由及当前 submission 或明确 absence。普通协作发起人本人完成，R38 通过自己的当前 disabled-creator/platform/org 规则，不伪造 accepted。

拆开 prepareCompletion 的只读意图准备与 recordSelfDelivery 写入：先读意图并过 gate，再写自身结果和实际 lifecycle/Stop/decision/receipt，最后消费。不能在先写 self-result 后提交一个 gate refusal。R38 在原 changeLifecycle 前后 prepare/finish，双账本与消费同 tx。原 operation-key 恢复不新消费。

## 系统消费者与锁

GitHub/VCS 贯穿真实验签、连接/installation、本地 UUID、单调 trust revision、repo/PR/head/事件身份，原 aggregate 仍独立必要。重连、secret/凭据轮换使 trust revision 变化；精确连接在消费事务 SHARE NOWAIT 复核，不以 source 字符串或最新任意 grant 代替。0 个/多个不明确匹配授权拒绝。任何保留的验签回调只能校验这次原请求，不能替代观察器或 nil 放行。

GitHub 全局 HMAC secret 目前是进程启动配置，非热更新契约；实现须绑定本次实际当前配置/授权时信任事实。更换部署配置需要停止旧进程准入并排空在途请求，不能宣称新进程 flag 保护旧进程。原 actor 始终 system，issuer 另留责任事实。

保留各 owner 锁前缀、W01/iteration、排序用户与 project 的规则；新锁使用 NOWAIT/短 timeout。四张 PR/link 表 SHARE NOWAIT 继续持到原事务结束。Prepare 和 Finish 均用 DB clock；commit ambiguous 不重试。具体凭据/锁图见 transaction-design 研究并在实现接口冻结时复核。

## D6/D7 与 skip

采纳 research/policy-compatibility-design-20261111.md 的逐项结果协议和所有客户端修复。结果按输入次序，首失败停止，先前事务保留，通知只聚合确定提交项；部分拒绝非 2xx、unknown503、缺绑定428。只提供逐项完成回执恢复，不添加全批账本。客户端成功解析/错误解析都验证结果与请求对应，不得把未知/截断当全成功；缓存重取真实状态。

固定观察算法版本不变；授权合同独立版本，mode=enforce 最后开放，CAS 新修订使旧未消费授权失效。合法回退追加 off/observe 保留历史，未知 schema/模式不放行。v1 skip_supported=false、waivable_checks=[]，所有硬检查不可豁免；明确拒绝 skip，不建无用 skip 表。D2 通用合同保留，不宣称可用 skip。

## 数据库与回滚

追加 schema，不改历史 766–780；授权身份/原观察不可变、消费/撤销/项目迁移留证，无 FK、每个并发索引单独迁移。readiness 检查确切有效索引形状及所需 schema，不以迁移台账替代。删除后保留授权/消费/撤销事实；缺当前资源拒绝公共读取。downgrade 首步在停准入维护期拒绝任何保留新历史或 enforce 政策，不能删除历史来换取回滚。

## 分包与风险控制

P0 行为保持的核心下沉，先跑现有真实制品/观察/评估/提示回归，后移动实现；P1 授权/消费核心与迁移；P2 普通/创建/项目/批量客户端；P3 VCS 与协作/R38；P4 最后开放 enforce 并完整冻结独立验收。任一先行包不等于全门禁，任务与 M1 在 P4 之前不关闭。跨包接口、文件所有权和具名证据由主会话管理。

## P1/P2 冻结接口补充

采用 research/authorization-contract-20261111.md 和 research/batch-protocol-contract-20261111.md 的精确 DTO、immutable authorization + 单条 terminal fact、original operation UUID、变更 digest 与逐项响应。授权与 webhook 保留 scalar exact identity，不用可执行任意 scope JSON。补充：新 Auth-only 授权路径遇与实际 source 冲突的 workspace 选择 header 应明确拒绝，不通过删除冲突 header 改写调用者意图。

所有完整借用事务校验器仅接收 pgx.Tx，并在内部 db.New(tx)，不能由调用者同时传入事务外 query pool。现有细粒度只读/读锁 helper 可接收 q，但不能独立充当完整准入。

P2 客户端可与 P0/P1 后端并行：D6 wire 已冻结且不依赖账本实现；它只改错误/计数/缓存，不开放 enforce 或新验收 UI。P1 的迁移编号781–788保留给授权/terminal及明确 readiness/回滚；后续trust修订和项目调整记录在主会话协调后分配，避免冲突。

授权排序字段的精确解释：ordinary target_position=null 表示请求没有显式 position 覆盖，不禁止既有 UpdateIssue 在状态切换时自动排到新列顶部；非 null 只允许与请求完全一致的 finite position。完成对象摘要与恢复比较必须区分“未提供排序”与显式值，不能因系统默认重排误拒所有普通完成，也不能在重放时重新计算并再次写排序。
