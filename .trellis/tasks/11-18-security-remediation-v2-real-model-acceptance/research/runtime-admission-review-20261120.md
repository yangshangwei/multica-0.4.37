# 独立真实运行准入复核（2026-11-20续）

复核者runtime_review，未编写产品/驱动/通道，产品与control只读。唯一写入本报告。工作树 `mc-secrem-v2-real-20261120`，基线d1b25077b。

## 当前结论

**执行准入暂未通过；不是预算授权不足。** 用户明确要求真实调用不限次数与总时长，持续推进到成功。当前固定模型`api666-zzzcoding/gpt-6-astra`、provider magpie、effort max、账号来源别名current-magpie-config，1200秒单次watchdog是卡死检测，不是累计预算。新授权覆盖旧PRD/spec固定预算约束；保留失败、记账、合成数据、隔离、平台证据和新模板版本纪律仍有效。

## 已核对的生产路径

- server/internal/handler/daemon.go的claim构建拒绝受限角色的外部MCP、runtime overlay、plugin、remote MCP、connected apps；不得借用mdt_或人类凭据绕过。
- server/internal/daemon/daemon.go实际runTask准备环境、完整角色/协议/花名册/小队正文，生产pkg/agent/codex.go执行原生app-server。RK1KnowledgeDriver只作为tagged驱动桥，不改这些产品行为。
- Codex适配器自动批准command/file和network/fileSystem权限申请，单靠普通config的never或workspace-write不构成边界，必须用只读managed requirements和容器边界。
- CLI透明文件请求通道可以保持不配置任何外部MCP，原始mat_身份与真实CLI/API不变。宿主broker固定token/workspace/agent/task/server，不接受模型覆盖；模型侧只写普通文件、shell仍禁网。

## 本轮亲自执行的无模型验证

- `python3 -B -m unittest discover ... -p test_runtime_controls.py -v`：11/11通过，1.454秒。包含显式uncapped授权、失败保留/诊断后恢复、原有限模式、并发预留、真实准入拒绝和网关边界。
- `python3 -B -m unittest discover ... -p test_multica_spool.py -v`：6/6通过，0.933秒。原始合成mat_与stdin不经shell、越界/全局覆盖拒绝、reviewer只读/评论、路径拒绝、variant约束、请求幂等。
- 静态读取私有relay.py及relay-preflight.json：仅GET/model/models与POST两个responses路径；固定host目标；实际internal模型网络前置探针models200、API403、直连公网拒绝，host仅看到models。此证据属于无凭据路由探针，不是模型结果。

## 已向实现者反馈的准入问题

1. **通道复用**：每轮固定spool目录会与复用workdir冲突，已改唯一run目录。
2. **角色与网络默认**：缺read_only_code或network不能默认可写/bridge，已改必须显式；leader/reviewer/QA代码只读，real网络inspect要求Internal=true。
3. **结果证据**：CLI audit只有hash不足以核对真实回执，已增加仓库外完整脱敏stdout/stderr；失败落盘需阻止wrapper继续。
4. **强制边界探针**：需真实live peer正例、普通与network widening拒绝、managed/代码/response不可写、peer不可读、mountpoint不可替换；结果必须解析exit/字段，不能只看BYPASS字符串缺失。最终native探针尚在运行。
5. **P1范围**：仅查询当前issue无法发现额外创建的issue/task，需workspace-wide完整快照与diff；不得用三个关键词自动判已证明known/unknown、禁止动作和“不宣称已修复”。应留独立语义评分。
6. **冻结与准入绑定**：source commit与fixture manifest须实际校验，输入使用冻结P1正文或显式重冻；准入须绑定场景、源码/二进制/配置与探针，不接受任意文件或None==None的scope。

后续只在实际driver fake通过、最终native wrapper探针通过、私有registration/admission与源hash一致后，给出P1-only准入。P1不等于其他案例、受限reviewer实际运行或全部AC9通过。真实模型尚未调用。


## 追加：会话回写宿主边界（阻断，已通知实现者）

实际wrapper的最后步骤原样使用`shutil.copytree(sandbox_home / "sessions", destination, dirs_exist_ok=True)`。模型可写sandbox home，复制时默认跟随符号链接；它可创建指向宿主绝对路径的链接，即使该路径在容器中不可见，宿主Python仍会在回写时读取。结果可能把宿主文件复制到下一轮可见session树。这不是假设中的任意恶意依赖，而是已存在的跨信任边界读路径。

要求：复制源只接受当前session树的非链接目录/普通文件，拒绝所有链接与特殊文件；目标当前task/issue session根及所有后代不得经符号链接重定向，写入不能跟随现有链接。初始session种子也须限制为自己的session存储，不可凭“位于整个evidence根内”导入其他角色/私有配置。用宿主合成sentinel分别测试源链接和目标链接拒绝，保留无泄露证据。P1真实准入在修复/复测前仍关闭。


## 当前通道修复与最终native证据

会话copyback已改为只接受普通目录/文件，源和目标后代链接及FIFO等特殊节点全部拒绝，文件open带O_NOFOLLOW；容器结束后复制，拒绝发生在ledger终态之前，记evidence_failed/125并停止后续。复核亲自跑5条copy控制全部通过（含宿主sentinel源链接、目标alias、嵌套链接/FIFO、普通rollout保留、拒绝后持久账本停止）。

实际wrapper native spool最终r11探针为PASS，复核逐项读响应并核对当前wrapper/broker/probe三个源码hash一致：nativeCodex0.164 initialize；managed批准never、只允许read-only/workspace-write、network=false；同容器live peer先200，然后普通和显式networkAccess=true命令均拒绝连接；dangerFullAccess被策略拒绝；代码/config/requirements/response写、peer读、requests/responses挂载点重命名七项均拒绝；唯一允许CLI请求保留合成mat_及workspace，越界issue与reviewer status操作126；wrapper退出0、ledger completed、源码未变。无turn/start，无供应商请求。

复核另外亲自跑最新runtime控制12/12通过，新增真实admission解析正例只使用合成文件、不启动provider；scope/model/account/source/verified缺失对照拒绝。以上允许继续实际fake P1驱动，尚未单独授权真实P1；仍需完整Claim→Run→Report通过及精确私有注册审查。


## 实际平台fake P1复核

读取private runs/fake-p1-04：真实NewRouter、模板创建/人类负责人添加、actual daemon Claim→Run→Report，task mat_经最终编译Multica CLI调用GET issue200、POST comment201、POST squad-evaluated201、PUT issue200；mdt_仅用于daemon领取、skill bundles、start/progress/messages/complete。评论author是实际leader、source_task_id是所领取task、revision1，非人工代写身份。workspace-wide diff只有单一issue/task状态变化、一条结果评论、对应活动/回执/订阅，agent唯一变化updated_at来自生命周期；没有新增任务/issue/成员或小队配置变化。

fake04原测试9项自动平台标准全true；其结论已改`platform-checks-passed-semantic-review-pending`，人工语义核对known/unknown、禁止动作、incident移交和不宣称已修复仍待真实记录。fake模型脚本只是plumbing证据，不算真实样本。

复核实际生成AGENTS为80,171字节，包含完整角色/协议/花名册/小队与最终`END MULTICA-RUNTIME`；claim合成指令62,284字节，无projection。harness源码hash47b7b50839b332455acbabdf5bac39ef7c6f63798d80efa5fdc3291affba887d，二进制ae23075f9968a7a571725e8855dda12a6cb7eba37b5fc06d37e2282358b451c7，当前文件与fake注册一致。该fake用实际CLI、不经Docker wrapper；与最终r11 nativewrapper+合成CLI的边界证据组合，仅支持P1只读、无仓库回合，不提前证明受限reviewer真实轮次或Contributor可写角色。

待最后私有政策/配置/source集合校验后，可给出P1-only首次真实执行准入，仍不等于P1行为通过。


## P1-only真实执行准入结论

**已批准首次P1真实执行，不是P1行为通过。** 私有receipt：`config/p1-admission.json`，由独立reviewer在23个当前source/binary/fixture/policy/relay/gateway绑定全部匹配后写入。root单独完成循环引用的最终registration/transport/harness配置，必须保持已审transport projection与source集合不变，再校验全文件hash。

核准证据为真实平台fake04及真实nativewrapper r11的组合；没有把fake自述算模型证明。实际model/network/role第一轮仍须保留RPC、原始CLI、平台和供应商输入输出证据。P1限定只读leader、无仓库、无成员执行。reviewer/Contributor等后续案例须另核scope。1200秒单次watchdog、无限累计授权、失败保留和诊断恢复依旧有效。

最后复核：runtime controls12/12（2.070秒）、spool6/6、context5/5、全部Python AST、当前native源码hash、relay只读mount与Internal网络、fixture16冻结及whole workspace evidence均满足P1范围。未运行全仓lint/typecheck，因为本次只改任务内Python/Go测试装置，Go编译由实现线完成，未改变TS产品；不宣称全仓检查已重跑。


最终循环引用配置复核通过（没有启动provider）：使用真实gate仅调用`load_registration`解析，registration065af14a8d8e4218aebd61d2c5c847fe5c97aabe371aa1d9ad9b22b4c0a1b795、harness config74d2da103b51f8bc2b8ff5473741bb85bf7b3f75413edc2f48c110d0777db9ac、admission ae6dce27ddae3358becc8be8dfb130fb3b221e7267c74e9885e9058dbcedea78；23source hash、政策projection、模型/effort/账号、fixture/CLI/harness均一致；ledger与输出尚不存在，各私有文件0600。私有回执`config/p1-final-config-review.json`。root可开始首个P1真实调用，仍须实际终态和语义复核。


## P1首轮启动不完整（实际模型0）

读取p1-real-01平台记录：一条leader task终态failed，只有失败system评论，issue仍todo；无成员任务。daemon.Run返回blocked，initialize报wrapper Refused，wrapper尚未预留ledger，也无供应商请求。因此这轮是**运行装置未完成启动**，原自动verdict的disproved字面不得解释为模型没有遵守指令。原输出保留，另记解释，不覆写。

根因已由代码确认：pkg/agent/claude.go `mergeEnv`对所有后端删除继承的MULTICA_*命名空间，只追加daemon显式任务env；Go测试入口的MULTICA_RUN_REAL_AGENT_SMOKE未向wrapper传播，guard在正确拒绝。修复方向是trusted parent的独立SECREM授权标记传给wrapper，wrapper局部恢复gate供guard校验；原Go入口仍先检查MULTICA门，真实admission/source绑定不变，标记不得进入模型容器或shell。不能为验收改产品filter，也不能泛化允许缺gate。修复源码须重绑独立准入并保留首轮零模型请求事实。


门标记修复已独立核对：Go入口仍先验证真实MULTICA开关、私有注册、source/binary/fixture/admission与owned DB，然后设置`SECREM_REAL_EXECUTION_AUTHORIZED=1`；fake设置空。wrapper只在自身进程收到该精确标记时恢复本地MULTICA gate供原guard解析；Docker显式env及shell policy不带任一gate。缺标记/非精确true/变更hash/fake模式全部拒绝。复核亲自跑context控制6/6通过。新的native r12再次全部通过，wrapperhash55e5aceea82f7d36ab9d4e36f79a284b7672c9b141ce73c0d95f90a7c682b138，broker与probe不变。

harness后继把runErr或noncompleted状态记为incomplete（不判模型行为），system-only不再满足普通结果来源检查。旧real01证据未改。以上装置变更不改变v2模板，下一次须新source/binary绑定与后继准入文件。


P1后继准入02已写独立文件，不覆盖旧记录：private config/p1-02-admission.json，sha2569b49cf3ceb384b519141c544bf487a21e164fd4aadde7cfecfa091fa1fc2b229。新wrapper55e5ace、新harness二进制41aed9791158c689de91deffbbf203a6e3459db06c74c2779a970fc2c1d2531e，全部23source匹配；真实平台fake05九项通过、native r12完整通过。前驱p1-real-01记录为启动未完成、native reserve/model turn/provider request均0；仅装置门传递与失败分类改变，模板/冻结输入不变。仍只批准P1范围，待root完成最终配置hash链接校验后启动。


## P1第二次：供应商拒绝，模型行为仍未分类

实际请求已到供应商，安全字段观察为model api666-zzzcoding/gpt-6-astra、reasoning.effort=max。第一份完整返回403，供应商提示账号仅允许官方Codex客户端。模型没有完成语义turn；平台harness正确写incomplete/model_behavior_classified=false，不能判模板失败。usage没有返回即unknown，不能写零成本。应以网关ledger与全部请求目录统计尝试，取消时有不完整请求也保留。

已发现装置relay/gateway使用header白名单丢弃原生User-Agent/originator等协议身份字段；修复只允许透传实际native字段，不伪造客户端身份。固定upstream、精确路径、真实token/actor由网关覆盖，不能通过任意Host/Forwarded/URL/凭据头覆盖。此为装置传输修订，无模板版本变化；需fake上游透传/拒绝对照与后继source准入。


第二轮最终账本为1个native process completed、2个POST请求：403权限拒绝，以及HTTP200 SSE中的response.failed并发限制。平台结果blocked/incomplete，没有完成模型turn，usage未提供为unknown。native进程正常退出不能替代平台任务/模型成功。网关完整响应在本地停后仍补齐，证实关闭本地连接不代表远端立即停止。

头字段修复复核：仅透传native实际User-Agent、originator、session_id、conversation_id、OpenAI-Beta、content-type/accept及x-codex-*；没有伪造任何官方身份。Authorization与x-openai-actor-authorization由private网关配置覆盖，Host/转发目标/hop头不传；relay固定路径和固定host仍然有效。复核亲自执行12控制测试全部通过（1.761秒），fake上游收到native字段原值，攻击者actor被覆盖，X-Forwarded-Host未收到。403根因仍需下一真实返回确认，不能仅从字段缺失推断已经修好。


P1准入03已按新gateway/relay两文件hash签发，其他21个绑定不变，sha2562902062653c5b526ee735604df2e12b089c068d3b71399c3f39408458d0b39f8。实际relay容器header前置证明只透传native UA/originator/x-codex-session、host固定、ForwardedHost丢弃，供应商请求0。记录前轮1native/2POST/0完成模型turn/usage unknown。当前无并行真实运行，可在最终配置hash闭合后串行重试P1。


## P1第三轮实际账户边界

读取新gateway证据可见官方二进制实际生成的UA为multica-agent-sdk/0.164.0（clientInfo来自产品SDK），协议头已经保留；首403及后续SSE response.failed仍含“账号仅允许Codex官方客户端”。这说明保留头本身未解除当前账号对SDK形态的限制，不可用伪造clientInfo/UA或改为exec桥逃避；不能把此失败归到v2指令。并发限制错误同样是供应商层失败，完整HTTP流不等于完成模型turn，usage缺失仍unknown。已及时报告root，建议停止对相同403的无变化重试，选择合法支持产品SDK的已授权路线或明确账户约束。

## O1预备增量审查（不构成真实准入）

另读actual fake-o1-03八项平台标准全true、四条预期执行：leader完整上下文、受限security reviewer无MCP且真实api_capability、无效回复普通leader上下文、owner新评论后完整leader。跟踪sourcehash与当前文件一致。指令、角色模板没有改。独立复核要求并已落实：人类决定把“other”作为type、route=human，避免非法route；实际评论时间等待到审查结果可观察秒之后；真实放行需人工读取审查结果后写私有permit并绑定review内容/decision内容hash。普通回复不改变状态/派发，受限结果不唤醒，最终in_review。O1后续仍须新scope注册、传输源hash及实际语义复核，P1准入不外推。


## 最终外部访问阻断与纠正

root明确停止真实调用及替代路由探测，本复核已收到并执行。当前账号GET模型目录返回47项，agent-router/gpt-6-astra及max存在（目录sha256400d20e76ad1109178eff47d6f545c797007f08a882bc0fe1540a8000a934c59），但**目录存在不证明这个账号允许产品SDK调用**。此前讨论“同账号另一目录路由或可用”的推测不能充当准入依据，现撤回任何由目录推断可调用的建议；必须有明确支持SDK的账号/配置或供应商许可，不能通过换路由、伪造官方client身份、exec桥或修改认证限制推进。

没有签发P1-04准入，没有执行替代模型调用。准确用量：P1real01本地装置未进入native预约；real02一个native进程、2个POST；real03一个native进程、5个POST；合计2个native真实尝试、7个供应商POST请求，另1次人工账号目录读取。完成模型语义turn为0，token/费用usage未返回，记录unknown而非0。不同HTTP状态/流完整性不提升为成功；平台两个实际供应商轮次均blocked/incomplete。原始官方客户端限制与并发错误证据保留，真实AC9仍未覆盖。

最终准入状态：**真实继续被外部账户SDK访问限制阻断**。已有P1范围的隔离/驱动机械准入证明保留为历史，不授权对未获支持的alternate route试探。无模型O1准备可留存，不能写成真实通过。需要合法SDK调用权限的配置后，重新核对账号/model/source/隔离注册再开展新的可追溯系列。
