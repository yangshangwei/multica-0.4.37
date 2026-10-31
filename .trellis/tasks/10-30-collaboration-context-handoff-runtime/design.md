# 可靠交接与召回：技术设计

状态：可执行设计，产品实现尚未开始。基线 `4c833e737`；现有代码锚点见 `research/current-code.md`。

## 权威来源与数据边界

复用 `lockRuntimeBusiness` 的当前业务能力和 run/attempt/reservation 校验。人类读取复用本单当前访问检查，不以 daemon owner 或来源空间身份替代调用者。所有查询绑定 issue/run/workspace。

| 交接事实 | 权威来源 | 不能据此声称的事实 |
| --- | --- | --- |
| 当前处理人、转交说明 | issue assignee、`issue_handoff`、关联 collaboration event | 转交说明不是 comment，也没有独立内容版本；使用 handoff ID + event sequence |
| 人类输入和来源 | frozen prompt + `input_event_id` 关联的不可变 comment-created 快照 | 后来编辑的评论不能覆盖已受理正文 |
| reader run IDs | 现有 immutable prompt membership / conservative readers | 只证明可能提供给 run，不证明模型理解或任务完成 |
| 成员结果/最终答复 | 已接受的 activity/outcome/record event | proof 证明终止，不证明任务成功；无 final answer 必须显式为空 |
| memory | run base freeze + latest authorized memory pointer | 最新版本不是启动时基线；清除后的 reanchor 仅沿用现有规则 |
| 决定、理由、证据 | 已授权 memory/comment/record 的引用或原文 | 没有结构化 writer contract 前标记 `not_structured`，不由服务端推断“确认/验证” |

数据库 sequence/revision 在线路中使用十进制字符串。来自用户的正文与服务端生成的作者、权限、身份、run 状态字段分开；正文中的权限声明永远无效。

## H1：有界只读投影

在现有 runtime context 旁增加版本化的 `handoff` 投影，或共用相同 service helper 的专用读取；不向 `Inputs` 数组塞入全历史，保留原约 2 MiB claim input 预算。协议包括：

- `protocol_version`、issue/run/attempt 身份、投影时间；
- authority（当前 handler、handoff/event 引用和时间）；
- original intent / handoff note 的出处与可用性；
- frozen inputs 的原始 comment/event 引用、command lifecycle 与 potential readers；
- 作用域内前序/成员结果、final answer 的 present/absent/partial/unavailable 状态；
- memory base/latest 指针、来源引用和各列表 total/returned/truncated；
- `missing`（legacy、purged、authority_lost、record_not_uploaded、not_structured 等可区分原因）。

所有列表和正文使用单独数量/字节预算，预算只截断投影而不删除历史。只有可授权读取的引用进入响应；被过滤的数据不通过 total、名字或拒绝原因泄露。H1 先投影已有行，无需通用知识表。

## H2：启动快照与直接派活

沿用 memory 协议的 claim 能力协商：旧 daemon 没声明支持时保持旧行为；声明支持后读取/验证失败必须在 Provider 启动前失败，不能静默降级。协议协商不修改资源配置 epoch，不触发无关重新授权。

在启动前锁定的事务中产生不可变快照及 hash/reference。同一 run 重试复用该快照；当前刷新另带时间和 `snapshot_kind`。快照包含有界派活正文和来源、role/step/parent、前序结果事实，不保存完整私人 transcript、文件或凭据。memory 继续使用独立 base 机制。

优先复用 run event 的 producer identity 持久化快照；新增内部事件必须同时更新 `ListCollaborationRunRecordEvents` 和 `collaborationRecordEventItems`，显式过滤内部元数据，不能让旧 records 页面遇到未知事件报错。hostPrompt、daemon client validator、CLI 及内置 skill 在同一增量更新。自然语言仅解释边界，实际权限仍由服务端受理输入/派活决定。

## H3：续作来源保留

当前单个 `parent_run_id` / `wake_parent_run_id` 无法重建所有来源。增加小范围 group wake-cause relation：group、分配顺序、kind、原始 event/run/input 引用、待分配或已分配 continuation。幂等键使用来源身份，不使用描述正文。

在现有 group lock 下序列化插入、合并和 claim 分配。claim 前所有来源归本续作；claim 后新来源归下一轮；prepared continuation 仍最多一个，保持原 ordinal、proof 和队列规则。历史轮次标 `cause_coverage=unknown`，不从最后一个 parent 伪造完整列表。

## H4：checkpoint 与终止分离

复用脱敏记录和 `collaboration_run_event`。checkpoint 只接受原始 business capability、run/attempt/nonce、单调 sequence 和有界 message delta。精确重试返回不含正文的 receipt；同序号不同内容、跳号和超预算被拒。撤权竞态以同一业务锁/事务顺序决定。

host 流式收集器按数量/字节/时间阈值提交 delta，队列和重试必须有界，不阻塞 Stop、launch termination 或 proof。达到预算或上传失败记录 coverage gap；权限失效后丢弃待上传正文并在仅元数据通道声明不完整，不能借终止 token 上传新发现。

定义 run/attempt 下统一的 message sequence；final outcome 带 covered-through 信息，使 records 投影只展示一次。旧 final-only runs 明确 checkpoint unavailable。新增 checkpoint 投影复用现有公开 message/tool discriminants，避免破坏旧客户端。usage 只记录实际获得的数据，不推算尚未返回的 token/cost。

## H5：runtime 评论分页与可信度

人类 comments API 已有 cursor/limit；主要缺口在 runtime/CLI 最新 200 条窗口。复用稳定排序和 cursor 校验机制，但保留 runtime 独立授权。游标绑定 issue/run、watermark 和 `(created_at,id)`；先固定可见范围，后续 append 不改变已开始遍历的范围。

新响应提供 items、total、returned、truncated、next cursor、serialized bytes 和预算。保留旧 context comments 数组，增加明确截断摘要；CLI `issue comment list` 只开放规定的分页参数。预算以实际 UTF-8 JSON 字节衡量，不能用字符数。

单条评论放不进请求预算时返回明确的 `comment_budget_too_small` 与最小所需预算；端点最大预算须能容纳一条合法最大评论及有界元数据的最坏 JSON 转义表示。不得静默跳过该条、返回无法前进的空页或让计数包含无权限内容。具体预算常量由共享 helper 定义并用最坏转义回归锁定，无需为此另建一套单条正文读取接口。

作者当前角色/显示名/SubmitCommand 能力只是观察时元数据。所有普通 comments/memory/handoff 引文标记为 evidence/data；只有原始服务端受理 input/delegation 标记为 admitted instruction，且也不能扩大工具权限。

## H6：共享界面与兼容

在 core 添加 schema/query，在 views 复用当前 handoff/activity/records 展示。Web/Desktop 仅做 platform wiring。所有私有读取保留 no-store、account/access epoch 和拒绝后缓存清理；不把服务端状态放进 Zustand。

缺失、截断、未验证、已撤回采用可区分展示；引用能定位原始记录。未知枚举、畸形 provenance、旧响应缺字段都不能赋予操作权。安装版旧客户端继续收到认识的 records 项。

## 迁移、回退与测试

新关系不添加外键/cascade；所需索引各用单语句 `CREATE [UNIQUE] INDEX CONCURRENTLY` 迁移。清理路径显式处理新增关系。禁用新写入仍须保留新历史读取，不能以关闭开关为由使 records reader 失败；仅在现有协商不足时增加运行开关。

测试顺序和逐项验收见 `implement.md`。真实模型实验用于评估行为，不替代服务端授权、快照幂等、分页和 checkpoint 竞态测试。
