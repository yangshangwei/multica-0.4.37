# 单次 E0b 重试结果

用户报告网关已修复并明确授权一次新编号重试。为严格执行单次范围，新增独立
`TestRK1KnowledgeRealE0bRetry`，复用原 E0b 的生产者、claim、实际 daemon 执行和全部
断言。它与原完整系列入口双向隔离，不进入 K1/K2/K3。

重试登记 `mode=e0b_retry`、`attempt_limit=1`，通过不可变哈希引用原停止台账的
1 次实际消耗，总 cap 仍为15；即使重试样本完整，也不能启动第二个新进程。旧台账
未修改，新台账单独保存。实现提交 `0fdb56632`，本地集成 `c160ba334`。

## 验证和实际执行

- 专库普通/竞态模拟各42个具名结果通过，0失败/跳过/非法JSON；四个固定顶层测试，
  单个 E0b 样本、1fake/0seed/0paid。编译源码、vet及独立准入复核通过。
- 新测试程序与4503个源码/依赖/嵌入资源绑定，实际Codex0.166、gpt-6-astra、网关端点
  和账户配置与首次尝试相同。用户所述修复属于外部状态，本轮未修改客户端身份。
- UX 明确交付≤10分钟准备+≤5分钟单次真实窗口，准备在界限内完成。
- 实际启动器运行于 **2026-11-05 06:08:42.990746–06:09:11.361005 UTC**，exit1。
  唯一新物理Codex进程启动1次，随后收到6个同进程403错误事件和失败的turn终态。
- 错误仍为 **`This account only allows Codex official clients`**；最终请求ID：
  `b620e541-333a-41d5-8439-56a9aa42c483`。
- 样本 `incomplete / unclassified`；没有完整汇总，K1/K2/K3均未运行。
  底层进程exit0不代表模型执行成功。

## 台账与释放

原失败1次 + retry02实际1次 = **2/15**，剩余 **13**。新台账已停止，执行许可关闭；
原签署许可另存，原05:40失败、原台账和新失败均保留。未把真实失败重标为通过，
也未增加总调用额度。物理启动数不是底层模型请求数或计费金额。

06:10:09UTC确认所有本轮自有测试/Provider进程0、专库连接0，并向UX明确RELEASE。
UX后续5case通过后已将窗口移交runtimeH1。知识无重型或Provider后继。

独立结果复核18项完整性检查通过：原台账/登记未变、源码4503项一致、三项样本构件
哈希一致、实际模型/程序/配置一致、签署报告哈希没有漂移。这证明失败记录可追溯，
**不证明E0b或知识模型行为通过**。

## 下一步

当前尝试仍被模型网关客户端策略拒绝。需要网关管理员核对上面的请求ID，允许真实
`multica-agent-sdk` / `Multica Agent SDK` app-server集成，或由用户选择允许该集成的
账户/端点。不能用直接CLI成功代替当前生产路径，不能修改originator/User-Agent绕过
限制。不得自动再消耗剩余13次；外部条件变化后再进行明确的新编号和窗口登记。

- [单次入口准入复核](research/codex-real-20261104/e0b-retry-review.md)
- [模拟验证](research/codex-real-20261104/e0b-retry02-fake-results.json)
- [实际结果指针](research/codex-real-20261104/e0b-retry02-result-pointer.json)
- [资源释放](research/codex-real-20261104/e0b-retry02-release.json)

- [实际结果独立复核](research/codex-real-20261104/e0b-retry02-result-review.md)
