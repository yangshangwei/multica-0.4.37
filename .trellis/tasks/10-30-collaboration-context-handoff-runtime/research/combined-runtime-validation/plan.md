# Runtime 收口与独立验收推进计划

状态：执行中。用户要求分析下一步、列计划并执行。当前已有父协调流程在集成 runtime，本线程承担未重复的独立验收；不新建重复任务、不替换其他 owner 的工作。

## 当前判断

启动注入补丁已经进入 `/Volumes/artisan/code/2026/mc-runtime-integration-20261031` 并通过定向复核；runtime 仍为未提交集成树。初轮 Desktop 通过，Web 来源跳转、跨页记录一致性、recording 恢复/封口由既有 owner 修复。先闭合这些确定性契约，再推进知识候选；真实模型验收继续单列。

## 推进与当前结果

1. [x] 核对父流程、源代码归属、集成补丁和已有验证。父协调者 `01a1920d-a340-7dd3-9d25-9191aa128cb0` 统一集成、提交和任务台账。
2. [x] 独立复核双能力联合测试的断言和夹具。确认 `HandoffVersion=1` 与 `RecordingVersion=1` 真正同时开启；封口响应夹具正确；当前失败可由缺少 recovery/seal 调用解释。报告：`independent-review.md`。
3. [x] 将联合覆盖缺口交给已有测试 owner：持续 503/阻塞时 proof 仍可确认、重新打开 ledger/新 Runner 的原请求恢复、终止身份/nonce 与 seal attempt/水位断言。父流程已分配独占的新 lifecycle 测试文件。
4. [x] 完成只读 source purge/snapshot/bind 权限与来源审核，记录 29 个文件的 hash，复核初始 17 个目标无漂移。三项 P2 已转交：runtime access 与嵌套请求事件字段不完整；cursor 缺少 run_id 仍接受；handoff 来源读取回显不适用的 run/version。报告：主工作区 `research/security-source-audit.md`。未确认额外正文权限或清除绕过，不把静态审核当动态安全验收。
5. [ ] 原 owner 修复后，针对这些具体断言与来源字段做复核；不把修改前的检查当作最终集成验证。
6. [ ] 父流程在统一源码上完成服务端/前端全套检查、Web/Desktop 复跑、规范、提交和验收映射。未完成项保留明确状态。
7. [ ] runtime 来源/版本/授权契约稳定后，使用已有 knowledge_preflight 草案推进知识候选。原真实实验下一槽位为 E2a2，剩余上限 23 次，不因本轮代码审核自动重试。

## 明确归属

- 已冻结的 `host_handoff_checkpoint_test.go` 与新 `host_handoff_checkpoint_lifecycle_test.go`：父流程 resume_check/指定检查代理。
- checkpoint 恢复、封口与总重试期限：runtime/checkpoint owner。
- source access、snapshot/bind、cursor：runtime owner。
- 本线程：`combined-runtime-validation/` 下计划及独立报告，和 `research/security-source-audit.md`/源码 hash；只读审核生产代码和测试。
- 知识设计、父任务状态、发布与全部应用数据库：本线程不接管。

## 验收原则

联合测试必须证明实际进程/原始 proof 行为；成功的 fixture 回应不能替代持续故障和重启场景。权限审核区分确认缺陷、缺少验证和推测风险；游标绑定不严尚未证明越权，不夸大严重性。全部观察带源码版本/hash；代码变化后由对应 owner 和独立复核确认修复。

## 2026-11-01 独立小修授权

父协调者已释放 SSA2，允许本线程在独立 `/Volumes/artisan/code/2026/mc-handoff-cursor-20261101` 修复 `parseHandoffCursor` 和新增纯回归测试；保持 integration/main 只读，交付函数级补丁和 base hashes。新增范围仅为从零值解码、显式 run_id 绑定及已约定的未知/重复字段拒绝，不修改 schema 或其他来源/恢复逻辑。

- [ ] RED：字面省略/null run_id、重复字段、未知字段等错误必须能复现旧实现接受。
- [ ] GREEN：保留空 raw 首页及四种合法 emitted cursor，错误均返回既有 invalid_handoff_cursor。
- [ ] 纯测试、静态检查、独立复核和最小补丁交接。所有 borrowed source 单独记账，不纳入本补丁。
