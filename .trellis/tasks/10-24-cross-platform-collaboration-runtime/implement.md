# 实施计划：普通本机执行能力接入协作任务

> 2026-10-26，in_progress。用户已授权并激活现有任务；S1–S5 已进入全范围复核，S6 仍有原生/账号/交付验收。逐项证据及未完成门禁见 [verification.md](verification.md)。

## 实施原则

- 业务权限保持，执行边界改为普通任务级别。只在协作适配处接入，不把协作任务伪装成普通 Task。
- 原 Linux 记录按原义收尾；新模式使用独立能力与终态，不伪造强证明。
- 先固定状态契约，再接 Provider；先有真实启动/等待观测，再开放平台组合。
- 不为此整理或重写整个 daemon.go，不新增依赖，除非实现阶段有明确理由并获得授权。

## 顺序与验收归属

### S1. 固定契约与迁移

- [x] 按代码契约建立 v3 runtime / v6 read 与旧 v2/v5 的正反例，覆盖模式、身份、版本和回执交叉拒绝。
- [x] 增加 host_exit_observed CHECK 前向迁移；更新 sqlc 查询及 Go 统一终态谓词。新字段优先复用现有 text/JSONB，必须新增的列在实现评审中说明。
- [x] 新增 v3 逐 launch 终态接口及 collaboration_attempt 的 launch_descriptor/terminal_observation；该回执只结束 attempt、不结算 run，N+1 许可事务重验前次已结束、授权及未停止。最终封口覆盖全部已发许可。
- [x] 协商新客户端能力；旧端保留合法 unavailable 读取与人类操作，host AI 写入要求新语义版本。
- [x] 定义受管理执行与后代清理两轴，业务结果继续独立。阻止 Result、ProcessExited、EOF、计时器被误用为通用强证明。
- 主验收：HA06、HA10。

### S2. 启动账本与生命周期适配

- [x] 将 collaborationrun 对具体 Process/NamespaceID 的依赖改为执行句柄和模式专属终态观察；保留旧模式适配器。
- [x] ledger v2 登记全部真实 launch、attempt、nonce、进程实例、结果和封口。旧 v1 保留原读取与回执。
- [x] 接入集中 start/wait 或经验证的 Provider 回调；覆盖 Provider 内部重试和外层 fresh-session retry，验证逐启动终态回执重放与 Stop 竞争下的下一次许可。
- [x] Windows 实现文件保护、持久化和跨进程锁；Linux/macOS 复用经验证的辅助函数。
- [x] 停止封口、spawn 与日志持久化故障形成明确状态；burn 后不明状态不倒退为未启动。
- 主验收：HA05、HA06、HA07。

### S3. 协作环境、身份与 Provider 接入

- [x] 沿 Backend/ExecOptions/Session 接入现有 Provider，局部提取普通消息收集/watchdog；普通任务 token、StartTask/ReportProgress 路由不复用。
- [x] 创建协作专属会话/工作目录键、临时目录、同身份互斥使用权与会话连续性处理；unknown 和重启不提前释放持久使用权。不可核实的 resume 只建立新会话，不借用私人指针。
- [x] 生成本单业务 prompt、CLI/MCP 上下文与窄用途凭据；账号、工具权限和普通 Provider 配置明确映射。
- [x] 适配冻结的模型/指令/配置、附件/Git；不支持的参数报拒绝，不静默忽略，不用 preparation=0 绕过。
- [x] 真实作者、Agent owner、运行资源 owner、父子关系、每次 launch 与用量完整上报；仅投影允许共享的来源和内容。
- 主验收：HA02、HA03、HA04、HA09。

### S4. 控制、队列与小队

- [x] 目标/整单停止先封闭 Server 扩展，再取消全部已登记启动；内部 fallback 重新检查许可、授权和封口。
- [x] 实现断网继续已有执行、无新许可不新增启动、确定撤权后停止的规则。
- [x] 新模式可靠终态后结算并释放容量；unknown 不释放逻辑屏障、不自动恢复。
- [x] 小队每个 member/continuation 独立登记和终态；验证同 lane 别名、合批与容量一接续。
- [x] 转交、退出、移除、整单完成/取消和重新授权仍使用现有业务规则。
- 主验收：HA07、HA08，回归父任务 AC08–AC12、AC23/24、AC34、AC37–AC40、AC57/58、AC92–AC96。

### S5. 重新授权、客户端与文案

- [x] 执行模式和有效权限配置进入授权/配置指纹。升级或扩大工具权限后旧 grants 失效，由实际资源拥有者重新确认。
- [x] 按 copy.md 更新中英文授权、模式说明、按所选范围的停止回执、未知/未确认、记录不完整及升级提示；区分 best_effort/unverified 与受管理终态，旧端不承诺展示新文案。
- [x] shared core schemas/API 支持新旧读模型；shared views 展示实际证据范围和三类事实，Web/Desktop 平台接线一致。
- [x] 不顺带实施完整“人→智能体”选择器改版；本轮授权和审计仍须能区分拥有者及稳定 ID。
- [x] 内置协作 CLI/skill 说明与交付文档同步新模式；不把当前尚未验收的平台标为已支持。
- 主验收：HA01、HA10、HA11。

### S6. 原生验证、发布与收尾

- [ ] 为每个拟开放的 Provider/版本/模式/OS/arch 形成验收矩阵，至少验证首批实际交付组合；未通过的组合明确不可用。
- [ ] 真机/fake CLI 覆盖启动前后故障、等待、Job 绑定失败、进程组逃离、关闭 stdio、内部 fallback、取消、撤权、断网与 daemon 重启。
- [x] 通过组合测试验证双端、单 Agent、小队、附件/Git、容量与新旧客户端；既有普通任务保持原行为。原生、真实账号及安装交付的具体范围分别记录在 verification.md，不把组合测试扩大为全部平台已验收。
- [ ] 新能力默认关闭，服务端和客户端先兼容，再发布 daemon、完成重新授权并按矩阵开放。
- [x] 实施完成后同步当前实现的规范状态和来源链接；当前 planned 文档不能代替实现验收证据。
- 主验收：HA12 及全部 HA 回归。

## 检查命令

从仓库根目录运行，使用项目既有 fake agent 防误调用约束。详细测试过滤器在对应改动阶段确定，不在规划阶段宣称已通过。

~~~sh
git diff --check
python3 .trellis/scripts/task.py validate .trellis/tasks/10-24-cross-platform-collaboration-runtime
pnpm typecheck
pnpm lint
pnpm test
make test
make check
~~~

Go 迭代检查在 server 目录下，覆盖 pkg/protocol、pkg/agent、internal/daemon、internal/daemon/collaborationrun、internal/service 及 cmd/server；数据库用隔离测试环境。默认测试仅使用测试创建的 CLI，不能调用用户已登录的工具。真实 Provider smoke 必须按 CLAUDE.md 的单独授权与开关执行。

原生 Windows/macOS/Linux 的停止语义和安装包验收另行记录；交叉编译及 Docker Linux 不替代 Windows/macOS 原生证据。

## 风险与回退

- 旧在途 Linux run、unknown、nonce 和授权记录不得改标或删除；必要时先停止新准入，等待原收尾。
- 关闭 host 新准入不等于可以卸掉其回执处理。存在 host 数据时只回退开放开关，不盲目回退数据库枚举或旧二进制。
- 无可验证终态能力的 Provider 模式保持不可用；不要用一条成功样例代替声明矩阵。
- 新授权说明必须随新模式上线；不能保留强隔离同意后静默扩大本机权限。
- 改动涉及协议、daemon 和 UI，最终验收以完整协作行为为单位，不以“CLI 能启动”判定完成。
