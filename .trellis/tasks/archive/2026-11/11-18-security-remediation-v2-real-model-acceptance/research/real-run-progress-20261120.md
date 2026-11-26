# 实际真实验收进度（持续追加）

用户在本轮明确授权不限调用次数与累计时长，持续推进至成功；型号沿用当前api666-zzzcoding/gpt-6-astra，provider magpie，max，账号来源current-magpie-config。每回合watchdog1200秒用于卡死检测，非累计预算。

实际平台P1入口：agentintegration tagged Go测试→生产router→普通daemon Claim/Run/Report→官方Codex app-server容器→原始mat_真实CLI文件通道。模型工具网络受managedrequirements与internal网络限制；供应商认证仅由宿主受控gateway持有。Gated源/binary/config均由独立reviewer逐次绑定。

- fakeP1-04通过9平台检查；因后续真实opt-in传递修正，fakeP1-05在新binary再次通过。fake不算模型证据。
- realP1-01：原daemon mergeEnv过滤所有继承MULTICA_，wrapper在启动原生进程前丢失验收开关并拒绝。实际模型进程0、请求0，runtime-incomplete；用独立SECREM父进程标记承接已核验Go入口，不改产品安全规则。
- realP1-02：原生app-server启动1次、POST模型请求2次。第一条403（供应商仅允许官方客户端），第二条HTTP200 SSE response.failed账号并发限制；无成功modelturn，usage未返回。实际请求model及effort=max符合注册。原生进程退出0不等于模型成功。操作者停止后远端仍继续处理过第二请求，证据保留。
- 为后续回合修正代理漏传native协议头：只转发客户端实际提供的User-Agent/originator/session_id/conversation_id/x-codex-*，不伪造；可信Authorization与actor头仍由私有配置覆盖。不能仅凭修改推断403根因已证明。

私有证据 `/Volumes/artisan/code/2026/secrem-v2-real-evidence-20261120/`，原始provider请求/响应/RPC/session不进仓库，后续只留脱敏摘要与hash。P1尚未通过；四例AC9/父AC10均仍开放。


## 当前停止原因与账户限制

P1-real-03 在保留真实原生 `User-Agent: multica-agent-sdk/...` 和原生originator后仍被明确拒绝。三个尝试累计：第一次原生进程0/请求0；第二次原生进程1/POST2；第三次原生进程1/POST5。共7个POST：6个official-client-only权限拒绝、1个并发限制；无完成的模型行为回合，没有tokenusage，记unknown。

用同一账号读取模型目录一次（47个模型），不构成模型推理请求。曾提出改用目录中agent-router通道，随后纠正并撤回：目录可见性不能证明该通道允许MulticaSDK，不能用换路由试探或绕过显式供应商限制。**替代通道调用0，P1-04准入未创建。** 没有伪造clientInfo/UA、没有换账号、没有改全局Codex配置。

用户不限预算的授权仍有效；当前需要供应商明确允许SDK的接入方式或已获SDK使用许可的配置。阻点不是调用额度，也不是v2模板行为失败。实际模型未完成任务，AC9/AC10不得勾选。P1/O1装置与无模型证据已保存；C1/D1角色挂载及Git交接机制预检完成，但完整真实平台驱动未交付。

供应商原始session/RPC/response仅在仓库外私有证据目录，仓库只保存分类和hash。所有原失败不覆盖。服务进程/网络关闭；3个独占数据库先pg_dump并记录hash，再删除原库和独占容器；供应商token副本已删除。后续恢复需新环境、新注册与合法SDK权限证据。
