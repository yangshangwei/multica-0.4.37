# 遗漏补齐与运行恢复技术设计

状态：in_progress，2026-10-21 终审通过且用户批准实施；本设计使用父任务 D03/D05/D06/D07/D08/D10 和 E01–E07，不替代它们。需求与验收以 `prd.md` 为准，事实与行号见 `research/`。

## 1. 交付边界与依赖

现有父任务保留完整 R/AC 总账，本子任务承接恢复和遗漏的补齐切片，并追踪运行端、代管及最终验收。每个切片有独立测试和可逆提交；不能用树结构推断依赖。

| 切片 | 前置条件 | 服务/平台边界 | 完成证据 |
| --- | --- | --- | --- |
| S1 调度和证明恢复 | 当前改动基线、失败回归 | service/SQL 与 daemon/ledger | AC-I01/02 |
| S2 私人资源提供 | 明确最小提供/采纳 API；控制基础稳定 | resource service → core → shared views | AC-I03 |
| S3 人员提及和取消历史 | 提及 helper、受保护命令投影定稿 | content/queue/history → core → views | AC-I04/05 |
| S4 文件/Git/provider/OS | 资源授权、恢复及能力契约稳定 | protocol/service delivery → daemon/controller → provider | AC-I06 |
| S5 有限代管 | lifecycle/队列收尾/历史合同稳定 | admin service → core/views → Web/Desktop platform | AC-I07 |
| S6 全量验收 | 每个切片已独立检查 | 各唯一主验证层与真实平台接线 | AC-I08/父 97 项 |

S2 和 S3 可独立推进；S4/S5 在依赖齐备后可并行。SQL generation、路由/API barrel、locale 和共享 schema 由单一协调者整合。每个切片先读对应 `.trellis/spec/`。

## 2. S1：有界扫描与失败隔离

服务端为 runtime/binding 的 waiting、wake 和候选扫描分别维护稳定的尝试位置。采用 keyset 分页与有限回绕，不仅扩大 LIMIT，也不一次无界加载全部目标。跨次轮询的位置须可恢复，使进程重启不反复返回同一失败前缀；扫描位置是调度元数据，不是业务授权、成功回执或队列顺序。

waiting 的稳定 key 使用现有 issue/target 唯一元组；候选 key 保留 reservation/派生优先级及创建/ID tie-break。对仍持 namespace reservation 的 run 保留优先机会；真正可领取的派生 run 延后新根的判定改用不受页大小影响的存在性查询。分页不能改变 lane 输入 FIFO、当前 active group 或真实容量占用。

优先级选择先于同等级内的公平轮转。每次给较低等级候选机会前，按当前授权、状态及完整作用域检查 reservation 优先候选，并完成该等级的有界分页与回绕；当前页为空或游标已越过 reservation key 不代表优先候选不存在。多页扫描可跨轮次完成，未完成时不以低等级候选填补名额；新出现或优先级变化的 reservation 在下一次选择时重新纳入检查。实现可用分等级持久游标或等价机制，daemon 消费服务端按当前优先等级选出的单个候选。派生存在性查询复用完整准入过滤，不能因页外、撤权或 held 记录误判；任何轮转都不能越过真实 namespace 占用。

wake/promotion 仍各有独立事务。分类明确可归于某个对象的错误后，保留可观察原因并继续扫描其他对象；全局 daemon 身份、请求取消和数据库整体不可用仍返回错误。事务提交不确定时仅通过原 group/run/operation 身份查询结果，扫描游标推进不能成为重新提交 mutation 的理由。运行中普通 DB 错误不被伪装为已持久拒绝或永久 target hold。

服务端候选页固定最多一项，与 daemon 每次 dispatch 实际只领取一项对应；候选优先级与持久尝试位置统一由服务端负责。非 `ErrUnconfirmed` 领取失败后，下一次轮询得到后继的机会。若服务端一次推进 16 项而 daemon 只领取一项，交替页面 `[1..16]`、`[17,18]` 仍可能反复只尝试 1、17；实施中的组合回归已证实这个反例，故不引入重复的 daemon 候选游标或额外 wire 字段。保留原 `run_ids` 响应，兼容严格解码的已安装 daemon。领取失败不生成替代 run/nonce；unknown run 保留真实 slot、blocked 和更新屏障，未支持协议不能降级普通任务执行。

proof 恢复保持最多 8 项/轮，使用持久尝试游标绕过固定目录排序；失败和 `ErrBusy` 均推进位置，原 proof/receipt/attempt 字节完全不变。游标放在独立私有 sidecar，沿用跨进程锁和 fsync，名称不得被当作 run JSON。单轮共享 10 秒总截止时间，避免 8 个独立 10 秒请求叠加阻塞主轮询；先记录尝试位置再发请求，崩溃后有限回绕仍会重试。取消尊重调用 context。

账本无法验证时仍 fail closed；不因跳过某文件释放容量、忽略 burned run 或生成新的 proof。proof ACK 只收尾原 run，停止/hold 仍需有效参与者明确恢复。

## 3. S2：明确资源提供，不开放私人目录

保留现有组织者工作空间 inventory 入口。为已合法参与的实际资源 owner 增加本单范围的明确资源提供记录：owner 在自己有权访问的库存中选择一个具体 agent/squad 及真实 roster，主动披露办理所需的最小资源摘要。组织者看到的只有本单被明确提供的组合，不能分页查询 owner 的全部空间/智能体/运行目录。

提供记录不创建 target、不授权、不邀请、不恢复。creator/handler 明确采纳时，在同一事务重验当前参与/账号、提供者的实际资源权、每个实际 owner 已参与、配置/绑定/fingerprint 和 target-set version，复用 `RegisterTarget` 的配置/角色逻辑。随后每个 owner 通过既有 consent 确认自己的部分，全部有效才进入 @。普通参与 owner 的提供能力不等于组织权，组织者采纳不等于 owner consent。

提供记录最小持久字段包括 issue、提供者、typed resource、固定 roster/fingerprint、版本及提供/撤回/采纳事实；幂等复用已有本单 operation/request identity。只保存办理摘要，禁止供应商凭据、路径、私人会话或一般目录。失效或撤回的未采纳记录不能采纳；采纳后的资源撤回仍走已有 grant revoke primitive，不通过删除提供记录偷偷撤执行。

多 owner 小队仅披露明确组合及办理所需最小摘要，任何无法合法确认的资源不返回详情；各 owner 的 consent 不合并或代签。原目录 membership/visibility 测试保留，新能力有自己的 issue-scoped owner/organizer 谓词。

## 4. S3：提及与不可变命令历史

`@all` 使用服务端已接受的 `[@all](mention://all/all)`，core 增加非 UUID 的显式生成分支；不伪造用户 UUID、扩 typed AI 目标或新增通知服务。共享选择器清楚区分人员（含 all）和 AI。预览与发送沿用当前草稿身份、suppression、权限版本和同事件去重。正文/评论编辑仅通知新增有效人，不执行 AI。

新增受本单当前 ACL 保护的分页命令历史投影，复用不可变 command 正文、作者/目标/附件快照与事件序号。execution 中的控制 DTO 不承担全部历史正文；独立历史 query 以 Server/viewer/issue/cursor 为 key，使用 schema、`redact:true` 和访问 epoch fence。普通来源成员按既有 read basis 查看历史，取消按钮仍要求当前参与者且为作者/creator/handler。

取消 provenance 使用精确 command/event 关联。未来 manual/自动取消写入与取消 state、reason、真实动作来源和 event 同事务。已有手动事件可用精确 payload 关联；只有可证实的历史事实才回填。旧自动取消缺乏精确 actor/time 时字段为空并说明记录缺失，不按临近时间猜测、不把后台服务冒充为人类。

展示原文、作者、typed 目标、sequence、等待/消费/取消状态、取消原因和真实可证实 actor/time，允许从取消记录定位原评论/附件但不以当前可编辑评论替代输入快照。`suppressed` 没有 command，不混进已取消历史；未读取 M11 输入仍可 waiting cancel，claimed/in-flight 只能走停止入口，界面不得自动扩大动作。

## 5. S4：文件准备、provider 与 OS 支持

先定义 versioned preparation/provider capability：固定 commit 或明确 attachment 版本、run/资源/configuration 身份、允许的业务数据和运行环境账号来源。附件以该 run 的当前窄业务能力取受控字节，事务外准备、开始前重验；终结 token 不能读取/上传文件。准备失败区分从未许可、已经许可和不确定，沿用 E03/E04，不制造不存在的 daemon proof。

Git 使用明确已提交 commit 在私有 run 工作目录物化，不复制原目录/未提交改动、全量用户 home 或私人 provider session。记录确切 commit/file 版本和 provider 来源；新环境清原恢复指针，只有资源匹配且上次可信结束时才能复用供应商 session。

复用 `server/pkg/agent` 的 canonical provider registry 和可复用结果/消息解析，执行仍由 collaboration supervisor 控制全部 launch/fallback/后代。为每个 provider/OS 显式声明可执行格式、依赖、环境账户输入、过程/结果/用量输出和完整 containment 能力；供应商账号不成为平台 token。没有证明完整隔离的组合保持 unsupported，不从静态 Linux 测试外推普通 provider、Windows 或 macOS。

Linux 保留可监督 PID namespace；Windows 需可验证 Job 绑定与封闭后代，绑定失败不启动；macOS 必须提供同等级可验证范围，否则继续明确拒绝。具体适配只使用现有依赖；遇到不能满足合同的组合保留交付缺口，不降低停止证明要求。

## 6. S5：独立有限管理入口

新增 collaboration-specific administration handler/service，复用 `authorizePlatformAdmin`、管理组织和写锁。只能办理 disabled creator 的 complete/cancel/reopen 与办理所需最小 metadata/candidate；管理员不凭该接口读全文/AI 日志、普通转交、组织资源或改 creator。

动作捕获 reason、request key、revision/status confirmation 和必要 successor。仅 double-disabled reopen 要求已有有效参与者接手，其他动作/非双禁用夹带 successor 拒绝。锁内重验实际管理员 session/role/scope、creator disabled、候选账号与参与、责任/版本；幂等 payload 包含候选。

复用同一 lifecycle close/reopen effects；重开/handler/持久待办/失效补充链/真实管理员与原 creator 的审计/history/outbox 同事务。重开保持 paused、旧 proof barrier 和指令终态，不恢复账号/grant/AI。客户端沿现有原键恢复，候选失效后刷新不自动改选。

Web 管理域及 Desktop 账号壳通过平台 adapter 接共享表单；平台路由只在 app platform，业务与 Query 在 core/views。password/admin-enabled 合成环境的证据单独保存。

## 7. 数据、兼容与回退

若需 scan/offer/cancellation provenance schema，采用加法 migration，无 FK/cascade；每个 `CREATE [UNIQUE] INDEX CONCURRENTLY` 单独单语句 migration，sqlc 由协调者生成。历史身份、nonce、sealed proof 和指令正文不能改写；已消费快照、已取消/已停止输入的通道与序号及历史 membership 保持不变。仅原 R26/M06 Switch primitive 可将尚未入组的 waiting 输入迁到新队长通道末尾并分配新序号，保留既有历史关系；本轮公平扫描或手动取消不获得额外重排权。旧缺字段 API 通过独立 schema 防御解析，畸形或关联不一致不产生额外权限。

切片回退关闭新增入口/执行准入，保留读取、停止和终结收敛。存在新协作活数据时不能直接回退旧 writer 或删表；默认创建/执行 gate 始终关闭直到父 P8 结论和独立发布授权。

## 8. 验收依据

`acceptance-matrix.json` 保存父 97 条的唯一 owner 和本轮状态，`implement.md` 保存命令和顺序。主层验证底层权限/竞争，Web/Desktop 验证真实接线；真实 provider、Linux/native containment、安装包、S3 和 Redis 故障均注明自己的证据范围。当前基础测试通过不等于前缀复现或上述实现通过。
