# 所选运行时执行Git物料：实现设计

状态：已由用户确认，实施合同冻结于2026-11-27。用户要求GitHub/GitLab仅依赖所选底层运行时；本设计替换物料服务端provider路径，保留现有物料与审核领域模型。

## 1. 改动边界

最小行为差距是：当前服务器持App/VCS凭据直接publish/merge/read，而应由项目选定运行时执行Git工具。涉及共享设置、持久调度、普通任务生命周期、daemon读回执和CLI工具。复用git_material修订/决定/请求/计划表，新增物料操作尝试，不建设通用工作流引擎。

移除物料专用material_provider.go及ResolveProvider接线，不将现有服务端密钥转交Agent。其他GitHub/VCS连接功能不受影响。内部materialgit的只读解析/门禁/路径规则可复用，写入实现不再作为物料产品执行路径；删除不再使用的生产写入分支，不保留双写/自动回退。

## 2. 配置与运行时身份

Settings删除connection_id和plan_agent_id，新增execution_runtime_id；enabled时要求非空且操作者具有使用权限。provider表示GitHub/GitLab，实例/仓库从授权project resource解析。保留resource_id、repository、base_branch、root_path、reviewer_ids、auto_start_plan和版本。首次auto_start_plan默认true；明确false保留。

采用现有显式RuntimePicker，显示provider和机器。没有有效选择就提示选择；离线保持配置并显示等待，权限失效/删除显示不可用，不自动选第一台。新客户端只在execution_protocol=1时提交运行时配置，老响应缺字段仅作未配置展示。

普通队列需要Agent，复用system carrier模式：每个(project,runtime)一个物料专用内部载体，model不设覆盖，使用所选runtime默认，实际调用记录有效model/config。所有阶段使用同一runtime；不用用户Agent，不改其runtime绑定。计划保留可见issue，Git操作任务不另建可见dummy issue。配置权限按当前人类和runtime访问权判断，载体owner不能代替实际originator。

## 3. 逻辑意图与执行尝试

保留git_material_job作为逻辑操作/调度去重，增加git_material_operation记录具体尝试：job/revision/material/project/workspace、action、attempt_no、operation_key、expected descriptor及digest、config_version、decision/baseline、authorizer/credential事实、carrier、runtime/daemon/profile/config摘要、task_id、generation、状态及receipt摘要。必要唯一索引独立CONCURRENTLY迁移，无新外键。

动作publish、review_sync、merge、observe、verify_baseline；生成方案继续已有plan领域。一个逻辑job只允许一个活动尝试；普通queue状态决定是否仍运行。2分钟scheduler lease只解决扫描归属，不能因过期重开写入。prepare在事务外，权限/配置/当前修订/queue+attempt插入在同一短事务，commit后唤醒。

运行时切换/删除不迁移旧尝试，未启动的旧任务拒绝并保留history；有结果不确定的旧任务先对账。runtime teardown先将物料system carrier解绑再执行既有系统Agent清理，不能级联删除物料run/usage/token历史；operation保存原runtime快照。

## 4. 任务执行与只读事实核验

用普通agent_task_queue、claim/start/backend.Execute及终态消息。新增material-operations-v1能力和MaterialExecution描述；旧daemon明确阻塞，不静默当普通任务执行。物料carrier只能领取绑定project/task/operation的阶段，不能配置/审批或调用普通创建任务等API。

模型运行时通过自身git/gh/glab或相应工具执行所有远端写入。Multica指令给出精确仓库、分支、文件、原始操作键及获批HEAD，不给服务端token。runtime结果完成文本只作诊断，不改变正式基线。

同一运行任务的daemon新增只读观察器，独立拉取冻结期望，使用本机授权工具读取真实PR/MR、完整变更/门禁与固定提交文件。Agent可通过CLI请求观察/提供locator提示，不能提交权威receipt JSON。只读观察器允许GET/树/blob等读取，禁止commit/push/创建PR/评论/合并和App令牌创建。凭据只在主机内消费，不写入结果。观察协议与现有HTTP provider解析复用，但不得暴露可写接口。

receipt走daemon认证专用回调，绑定operation/task/runtime/generation/expected_digest和host-only challenge。challenge在实际已注册daemon的领取/凭据完成交换中下发，只留在host传输封装，不进入模型prompt/mat_凭据；普通GET不凭人类PAT生成或泄露challenge。现有daemon可用owner PAT认证，故校验以具体注册/领取绑定为准，不能声称仅凭凭据类型拒绝所有人类PAT。服务器比较真实读取的完整有界字节与冻结修订，核对目标/全路径/模式/HEAD/merge事实和当前权限；不使用服务端Git网络或App读取作为隐藏依赖。信任边界为已授权daemon，非恶意主机的密码学证明。

## 5. 阶段转换

1. ready候选冻结并入队publish；运行时执行写入，观察证实精确内容及review后才进入in_review。
2. 人类决定绑定当前revision/HEAD；review_sync由所选运行时发布真实署名记录，观察核对标记/正文。退回/撤回不创建merge授权。
3. merge先在运行端读当前HEAD、目标、完整变更和原生门禁；服务器复核当前批准并授予短时、精确条件的merge permit。所选运行时执行条件合并，不能管理员绕过门禁。
4. 合并后观察到实际merge commit/actor/正确字节，服务器复核并一次消费回执；排队、超时、命令退出0均不等于合并。
5. requirement基线生效后建立原有plan todo。自动关闭时不启动模型；自动或人工启动获准后由运行端只读preparation验证基线，随后才启动同一runtime的方案生成。移除processMaterialPlan服务端provider读取。
6. 方案按既有受限API提交并进入同一发布/审核/合并流程。终态无产出可人工重试，保留issue和旧run；不自动编码。

门禁等待及未知结果不每3秒新建模型。一个有界只读观察请求可由该runtime所在daemon按截止时间退避重查，产生明确waiting/blocked；下一次模型写阶段仍需当前条件成立。已授权但结果不确定的写入先观察原操作键/branch，确证无结果或可恢复部分结果才允许新尝试。外部未授权合并、无法归属的GitLab残留分支不得伪装获批成功。

## 6. 历史与兼容

旧connection字段不再参与物料授权。没有runtime的旧设置展示需选择，不从plan_agent偷偷推导；旧未合并候选因配置已变需明确重发/重审，保留原快照。已有合并基线保持不变，通过新运行端读核验后可继续新授权计划。旧pending配置请求不可删字段沿用request_id，显示需重新核对；新请求保持精确重放。

服务端旧任务停止直接provider扫描副作用；history可读。API响应schema容忍旧/未知字段，新的execution_protocol声明用于写入准入。没有真实已发布客户端证据，不引入旧执行路径双写/降级。

## 7. 验证与回退

确定性fixture使用测试可执行程序模拟所选runtime，实际消费claim/start描述并通过测试Git HTTP/本地仓产生副作用，daemon观察真实fixture结果并发送receipt。必须证明绕过fake执行器、手工最终状态或伪造model JSON无法得到通过。

覆盖无App/VCS、runtime变更/撤权、旧daemon、重复/丢响应、门禁/路径/字节、手动等待/重试、历史删除和普通任务公平性。Web/Desktop使用生产构建、同一流程。真实Git/模型另有证据等级，保持原LIVE计划。

回退停止新阶段和取消尚未执行任务，保留远端已发生事实、审核和operation历史；不自动重启旧服务端provider。数据库down遇到已执行历史应拒绝破坏性回滚，保留可读证据。
