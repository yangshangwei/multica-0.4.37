# 第二期技术设计

## 选择与边界

采用本地默认偏好＋项目配置投影＋服务端受控推荐。候选选择使用现有配置的LLM，结果不执行任何动作。

| 方案 | 取舍 |
| --- | --- |
| 仅前端文字匹配 | 可作为检索，但难比较任务意图，不满足本期AI辅助体验 |
| 全量检索后有界LLM比较（采用） | 无新依赖，出站可控，理由能核验；大目录跨语言召回有限 |
| 向量检索／多轮层级模型 | 需要新索引、成本与生命周期，不纳入本期 |

项目关联是独立非AI入口。推荐接口只接收任务文字，不传项目资料；项目切换仍使当前建议失效，避免用户误认为旧建议针对新上下文。

## 客户端数据流

1. `quick-create-store.ts`增加 nullable `defaultActor`、带作用域保护的setDefaultActor，partialize/normalize/reset同时更新；旧数据默认为null，保留第一期所有字段。
2. 新纯helper集中处理5级预选及候选的pending/known-invalid区别。父表单保留用户手动选择，不因后续查询返回或项目变化重新选人。
3. 从已有projectListOptions数据找到projectId，用getProjectExecutionSquads投影configured squad IDs，与visibleSquads取交集。plural空数组不回退singular；不增加逐项目／逐成员请求。
4. 选择器新增项目分组和project浏览视图；只读defaultActor标记、底部设为默认／清除动作。8项快捷预算按PRD分配并全组去重；搜索仍全目录。所有会移除控件的动作先聚焦持久搜索输入。
5. 选择器外、编辑器上方增加紧凑「帮我选」入口与最多3条建议。调用时flushPendingUpdate，再读取getMarkdown；UI原文不改写。建议展示当前目录名称、类型、`reason`原文及采用按钮。
6. 通过core mutation发显式workspace UUID与AbortSignal，retry:false、gcTime:0，不写React Query实体数据或Zustand草稿。父层采用时复用onPick路径，随后清理结果并恢复到创建助手触发器焦点。
7. 每次请求捕获文字、projectId、当前actor、wsId/user/resetGeneration和request generation；任何相关值改变时abort并使generation失效。采用前也验证当前作用域与目录，避免缓存建议越过权限变化。

空文字禁用请求；同一请求进行中显示取消并防重复。没有AI配置或旧服务端404显示推荐不可用，手动流程完整工作；不新增provider配置界面。推荐读取文字，不读取上传文件或系统指令。整体提交、上传和描述完善行为保持原合同。

## 推荐接口合同

`POST /api/issues/recommend-creators`

请求：`{ "text": "用户当前需求" }`。工作区从鉴权请求头解析，拒绝未知字段／尾部JSON，body≤128KiB，trim后非空、text≤20000 runes。

响应：`{ "recommendations": [{ "actor_type": "agent|squad", "actor_id": "uuid", "reason": "职责原文片段" }] }`，0–3项。客户端用Zod验证业务字段，并按既有兼容规则忽略无害的新增字段；缺失/null、非法类型、非UUID、重复或超量、空reason为格式错误而不是空成功。reason≤240 runes，按纯文本显示，名称由当前授权目录解析。

服务端流程：

- 这是人类成员主动使用的Web／桌面接口：在读取目录／调用provider前拒绝agent凭证（403），然后验证工作区成员；候选来源Queries.ListAgents(active user-kind)和ListSquads。跨工作区、归档、无runtime绑定、不可调用对象及队长不合格小队过滤掉。
- 可见不等于可调用。复用agent_access.go的调用规则；为避免550次查询，抽出已加载行／授权目标的纯判定，复用loadInvocationTargetsByAgent批量加载。原invokeAgentDecision保留查询包装与同一纯判定；原权限矩阵测试必须先保护，不能引入admin私有权限旁路。现有其他接口的agent/system调用语义保持不变；新推荐接口仅传入已经验证的人类成员身份。
- 扫描所有合格对象的完整description。≤40个有描述对象时全部参与，存在文字命中则取命中附近片段，无命中则确定性取前600runes供模型比较；>40时按描述的英文词／CJK双字片段与任务文字的重叠排序，采用稳定type/id打平。仅在>40分支保留正文字证据，不能先截断数据库返回顺序；这一分支没有证据时返回[]。选取命中附近的最多600-rune连续片段，避免永远只看长描述开头。检索规则不是能力标签系统。
- 发送`{text,candidates:[{ref,actor_type,description_excerpt}]}`；ref为本请求c1..c40，模型看不到名称、数据库ID、instructions、模板、runtime或凭据。序列化payload≤128KiB，超过预算逐步减少候选，输入文字本身不静默截断。
- 使用h.LLM.GenerateJSON，默认已配置模型，无工具，输出预算2000tokens，总生命周期30秒。复用现有descriptionAssistSlots的有界准入；共享饱和行为明确返回429，客户端不自动重试。无描述／无候选可直接返回[]，不调用模型。
- 系统提示将文字和描述视为不可信数据，只输出`{recommendations:[{ref,evidence}]}`，最多3项。严格解析输出≤32KiB，拒绝未知／重复ref、>3项、空／超长evidence或不在发出片段中的精确文本。公开reason是验证过的evidence，不能自由生成能力说明。
- 生成后重新检查最多3个对象的当前权限、归档、运行时绑定、队长及description。失效对象丢弃；当前描述已不再包含依据则丢弃。数据库故障返回错误，不能当作无匹配。请求取消立刻返回，不写任何实体或启动task。

错误码沿用既有AI规范：invalid_request400、成员／主体鉴权401/403/404、ai_unavailable503、ai_busy429+Retry-After、ai_timeout504、ai_generation_failed/ai_invalid_output502。不记录或返回provider原始错误、提示词和凭据。

## 文件与责任

- Core状态：quick-create-store.ts/test，预选helper及测试；默认数据仍属client state。
- Core API：types/issue.ts、api/schemas.ts、api/client.ts、issues/mutations.ts和对应测试；请求workspace固定、信号转发、响应解析。
- Views：quick-create-issue.tsx/test；actor picker/model/tests；新recommendation hook/panel及真实组件测试；两种modals locale。
- Backend：新issue_creator_recommendation.go/test；agent_access.go及权限回归；server/cmd/server/router.go注册静态路由；server/pkg/llm客户端consumer说明与outbound_contract_test；现有.env.example与两种环境变量文档同步出站字段并重新生成相关文档bundle。
- E2E：新phase2测试（使用TestApiClient、隔离DB、httptest／本地确定性provider）；扩展第一期标签变化与回归。
- 规范：更新quick-create-actor-picker.md，新增server推荐接口合同。无需migration/sqlc新查询或依赖。

## 风险与回退

默认等待不能锁死手动选择；只有未明确选择时按预选链等待高优先级数据。未知旧server project字段只影响项目组，默认和手动入口继续有效。

推荐的存在不等于运行就绪，实际提交仍走原权限／CLI版本门槛。检索对大目录的同义／跨语言召回有限，提示无依据并保留全量搜索；不宣传“全局最优”。

新前端遇到旧接口404可继续手动；新服务端不要求旧客户端采用推荐。回退UI可忽略defaultActor字段，旧客户端重写可能丢失新默认；任务数据不受影响。当前目录含第一期及其他未提交修改，使用隔离快照，回传仅本期增量。
