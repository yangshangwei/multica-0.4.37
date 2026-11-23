# Git 工程物料工作流：外部一手证据

查阅日期：2026-11-10。研究范围是 Git 与 Multica 连接的正确性机制，不是集成功能或竞品清单。下文严格区分官方事实和本任务建议；GitHub 的具体交付语义不自动代表 GitLab 或其他托管系统。

## 1. Git 仓库和托管平台资料是两类数据

**已核实事实：** GitHub 官方把 Git CLI 的 mirror clone 与“Git 仓库加部分元数据”的迁移归档分成不同备份方式。前者保存文件、目录与修订历史；后者另经 REST API 生成。官方还明确迁移归档不是全部仓库相关数据的完整备份。

**对本任务的含义：** 必须先区分“把需求、设计、任务、缺陷保存为提交到 Git 的文件”和“使用 GitHub/GitLab 的 Issue、PR、评论等平台对象”。不能把 `git clone` 当作全部协作状态的获取或备份机制。两者的同步、导出和恢复契约不同；已有托管平台对象需要明确 API 映射，不能仅以仓库 URL 表示已打通。

来源：[Backing up a repository](https://docs.github.com/en/repositories/archiving-a-github-repository/backing-up-a-repository)。核验时也读取了 [GitHub 官方 docs 源码](https://github.com/github/docs/blob/main/content/repositories/archiving-a-github-repository/backing-up-a-repository.md)。

## 2. Webhook 可能乱序，也不是即时通知

**已核实事实：** GitHub 明确可能按不同于事件发生顺序的顺序交付 Webhook，建议使用 payload 时间戳理解事件的相对发生时间；通知可能延迟数分钟。

**对本任务的建议：** Webhook 应作为启动核对的信号，不能仅以接收顺序覆盖业务状态。需要对象版本检查、读取权威来源现状以及定期对账。时间戳有助于判断先后，但不能自行解决跨系统并发修改；方案仍须定义字段归属和冲突策略。

来源：[Troubleshooting webhooks](https://docs.github.com/en/webhooks/testing-and-troubleshooting-webhooks/troubleshooting-webhooks)。

## 3. GitHub 不自动重投失败交付

**已核实事实：** GitHub 不会自动重投失败 Webhook。官方给出人工重投以及定期查询失败交付、通过 API 请求重投的处理方式。

**对本任务的建议：** 连接器必须拥有可观测的补偿机制。验收应模拟接收服务中断，证明恢复后能找回变更，并明确失联期间禁止哪些推进动作。只测正常通知到达不够。API 当前对象状态核对与失败交付重投是互补机制，不能假定重放通知就能重建一切历史。

来源：[Handling failed webhook deliveries](https://docs.github.com/en/webhooks/using-webhooks/handling-failed-webhook-deliveries)。同时读取了 [GitHub 官方 docs 源码](https://github.com/github/docs/blob/main/content/webhooks/using-webhooks/handling-failed-webhook-deliveries.md)。

## 4. 重投仍使用原交付标识

**已核实事实：** GitHub 建议使用 `X-GitHub-Delivery` 判断交付身份；请求重投时该值与原交付相同。官方还建议校验 Webhook secret，并将处理异步化以及时响应。

**对本任务的建议：** 保留持久化接收记录，区分“已收到”和“业务动作已完成”。按交付身份去重，但允许未完成事件继续恢复。创建任务、触发 Agent 等副作用另需业务幂等键；重复通知、重投或进程重启都不能多创建一次任务。这里只借鉴机制，不要求新增消息中间件。

来源：[Best practices for using webhooks](https://docs.github.com/en/webhooks/using-webhooks/best-practices-for-using-webhooks)。

## 5. 审批必须对应被批准的变更

**已核实事实：** GitHub 分支保护支持在 PR diff 发生变化后使旧批准失效，也可要求最近一次可评审 push 获得其他人的批准。这些是需要配置的选项，不是所有仓库的默认保证。

**对本任务的建议：** 需求、设计、任务拆分、代码与验收材料的批准不能只是一个永久布尔值。至少记录批准对象、版本、审批人、依据和批准范围。上游需求修改后应计算影响，停止继续沿用受影响的旧批准，并重新核验；不必使无关审批全部失效。分支保护本身不会替 Multica 识别跨文档的语义影响。

来源：[About protected branches](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-protected-branches/about-protected-branches)。

## 6. 来源证明不是功能正确性证明

**已核实事实：** SLSA v1.2 将 provenance 定义为可验证的来源与生产过程信息；build provenance 用于把构建输出追溯到所用源码，source provenance 用于追溯源码修订及其变更管理过程。

**对本任务的推论与建议：** provenance 解决“这个结果来自哪里、如何产生”；不能仅凭它推出“满足用户意图”。建议把需求版本、代码 commit、运行身份、测试环境与产物摘要绑定，以免用旧结果给新版本背书。绑定摘要是本方案建议，不在此声称已经达到某个 SLSA 等级。功能正确仍须由独立验收样例、真实业务结果和可复现测试证明；另一位 AI 的同意不能代替这些证据。

来源：[SLSA v1.2 — Provenance](https://slsa.dev/spec/v1.2/provenance)，直接读取官方页面核验。

## 可直接写入需求的可证伪条件

- 对同一通知重复交付或重启接收进程，只产生一次业务动作。
- 将旧通知放在新通知之后交付，不使任务状态倒退或覆盖更新的批准对象。
- 接收服务中断后，补偿与核对能恢复正确状态；界面能显示尚未确认的同步情况。
- 需求或 PR 发生受影响的变更后，旧批准与旧测试结果不能继续被当作当前交付依据。
- 从一个交付结果能回查对应需求版本、设计决策、代码版本和独立验收证据；缺少证据时不能显示“已验证完成”。

以上是待实现、待测试的候选验收条件，不是已经验证的 Multica 能力。
