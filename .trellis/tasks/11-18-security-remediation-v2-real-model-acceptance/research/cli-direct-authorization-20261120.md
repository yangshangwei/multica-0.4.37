# 官方Codex CLI直连授权及证据边界

用户明确选择“不用SDK，直接调用codex”。采用真实官方CLI `codex exec` 标准入口，它自己生成客户端身份，不修改User-Agent、originator、clientInfo，不把SDK请求伪装为CLI请求。沿用当前账号与原模型，未选择替代模型路由。若官方CLI仍被拒绝，保留拒绝并停止该入口，不绕过。

现有app-server真实结果（2native、7POST、无成功模型回合）独立保留。新的official-cli-direct队列从0计数，继续适用用户不限累计次数/时间的授权；单次watchdog只防卡死。

产品不新增CLI后端。test-only agentintegration overlay在唯一backend解析处选择试验直连Backend，生产普通Claim、完整prompt/AGENTS/技能和task环境、平台工具以及Report链条保留。backend stdout由CLI原生JSONL解释，不使用假app-server桥；结论只从真实平台状态/评论/回执/活动及人工语义复核得出。这样能验证指令+平台闭环，不能宣称现有app-server生产适配器已通过。

角色容器、managed不可写禁网策略、internal网络与模型代理白名单、原始mat_ CLI文件通道、私有会话/响应保存规则保持。官方CLI自己的读指令/原生工具行为须用假Responses端点先验证，再独立绑定准入。
