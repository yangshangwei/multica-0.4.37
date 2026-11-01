# 续作执行账本

统筹：Codex `01a1979f-c71b-7ea2-8734-185ab2bdc8f8`。恢复现有任务，不新建任务。现场快照及预算分账见同名 JSON；该快照只表示接管时状态，后续提交以实际分支和核销证据为准。

接管时集成分支 `codex/cross-workspace-collaboration` 为 `6e0393867`，工作区干净，相对本地 origin 跟踪记录领先 104 个提交；未 fetch、推送或触发外部 CI。Trellis 父任务已重新设为当前任务，状态保持 `in_progress`。

## 执行归属

| 工作 | 接管时 HEAD | 本轮负责人 | 状态 |
| --- | --- | --- | --- |
| runtime audit 接续分支 | `1eb0cb33c` | `/root/runtime_fixes` | 保留3个 dirty 文件；补完9项审计和5项召回复核跟进，待整批独立复核 |
| 知识 claim | `3110295d0` | `/root/knowledge_claim` | H1/M1已有提交；补 H3/M3 及托管参数复核问题 |
| 知识 launch | `3351c4c13` | 统筹排期 | 保留2个 dirty 测试；H2/M2/L6已有提交，接受检查和L7待核销 |
| 知识 write | `fba1fca3f` | 统筹排期 | 保留L5实现与测试等5个 dirty 文件及红测证据 |
| 知识 web | `a9801fb07` | 统筹排期 | M4/M5/L8/L9/L12已有提交；L10/L11/L13与复核跟进待完成 |
| 知识集成 | `8c0f37672` | 统筹独占合流 | 四分道尚未合流；不得继承旧workflow的完成状态 |
| RK1 framework | `b8fd547dc` | `/root/rk1_framework` | 保留未提交框架；修复14项复核意见并重跑fake/sim；runnable=false |
| 主集成工作树 | `6e0393867` | `/root` 唯一写入者 | 更新台账；按独立复核结果串行合入 |

旧 Claude 执行会话日志最后更新不晚于22:48，进程核对没有仍在运行的 Claude CLI；现有 Codex 旧统筹会话 idle。本轮不跨会话恢复旧 workflow，也不重置工作树。知识21项中14项已有分道提交，不等同14项通过；各分道完成门、最终复核和联合验证仍待执行。

## 串行门

1. 各分道修复、具名验证、独立复核无阻塞后，runtime → 知识 → RK1 → 交付准备代码。知识内部 claim → launch → write → web；随后吸收最新主集成并复核。
2. 最终合流版本完成sqlc、迁移、Go build/vet/测试/race、前端lint/typecheck/test、make check、门控Web/原生Electron和RK1 fake。每个DB-backed包用独立任务库，保留失败现场。知识开关开/关各验，生产默认false。
3. 重新固定源码、框架、CLI、配置与构件哈希，不能拼接分支历史通过结果。
4. 独占真实窗口才运行正式E0及预登记顺序；首个不完整结果即停，保留原始失败；不得健康探测、替换样本、换模型或延长超时。

## 预算与外部门

- RK1 runtime：25次已批准，消耗0；含H2b，H6每轮两次启动。
- 原系列：剩余14次，下一E3 after trial 2，再E4→E5→E6；冻结源码与旧编号保持不变。
- K1–K3、普通任务E0b、安装版真实账号旅程、AC43：需单独核对账号/环境和预算，不从25次扣取。
- RK1-H5允许两个自控Multica用户共用Provider；AC43仍需两个执行环境及不同Provider账号。
- Windows原生、安装升级、各平台候选包、离线升级、多账号额度均未验收。公开推送、外部CI、部署发布未获本轮新增授权，仅做本地准备。

本记录没有运行测试或真实Provider，没有关闭任何父验收项。后续结果必须附被测SHA、命令、环境、退出码、pass/fail/skip与证据哈希。
