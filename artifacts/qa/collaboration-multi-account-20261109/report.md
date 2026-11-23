# 多账号、多 Agent 跨工作空间协作测试报告

**结论：本次模拟闭环通过。** 三个协作账号分别拥有工作空间、Agent 和运行环境，在同一条共享任务上完成五轮 Agent 执行、一次退回修改和最终人工验收。最终四项场景全部通过，零跳过；首次主线运行曾因测试断言使用错误的响应字段失败，修正测试后重跑通过，失败证据完整保留。没有修改产品代码，也没有发现本次覆盖范围内已确认的产品缺陷。

测试日期：2026-11-09，Asia/Shanghai。浏览器测试实际运行时间为 13:47:51–13:51:07；之后完成证据核对和服务清理。本文不是首次运行全绿的声明。

## 1. 测试范围与方法

- 主线：3 个协作账号、3 个工作空间、3 个私有 Agent、3 个独立托管运行环境；另设 1 个无关账号验证拒绝访问。
- Web 使用独立浏览器会话；任务创建、邀请、授权、指令预览发送、信息请求/返回、转交、交付、退回和验收通过页面操作。
- 账号、空间、运行环境登记及资源提供/采纳使用公共 API fixture；数据库仅用于环境初始化、迁移、测试门设置与只读核验，不伪造业务流程或运行结果。
- 后端、数据库、生产版 Web、调度器、CLI 和 Provider 子进程实际运行。Provider 是确定性模拟程序，按测试指令产生合成结果，通过真实 CLI 写入评论与共享上下文。
- 每轮运行同时核对输入、指令作者、资源拥有者、Agent、运行环境、上下文版本、服务端终态和本地已确认退出记录。

**证据边界：** 合成场景名为“订单重复扣款修复”，但并未实现或验证真实支付代码，也未调用商业模型。本报告证明协作编排、权限、数据交接和验收通路，不证明模型自主修复能力。

## 2. 环境与源码身份

| 项目 | 本次事实 |
|---|---|
| 源码提交 | `4d454bb2c2c945664fe73947ecaccf053909d55a`，并保留当前工作区内容的源码哈希 |
| 部署方式 | 任务私有源码快照、独立数据库、独立服务进程与端口 |
| API / Web / renderer | `127.0.0.1:58765` / `58766` / `58767` |
| Web 模式 | production，构建 ID `3iIFVPCnhVORPEid8eW4f` |
| 身份认证 | password，托管安装启用，使用合成账号 |
| 数据库 | `multica_qa_20261109_4f1375f76bad`，创建前确认不存在，迁移至 765 |
| 平台 | macOS arm64；Node 22.23.2、Go 1.27.0、Playwright 1.58.2 / Chromium |
| 源码终检 | 8,171 个产品相关文件未发生漂移；运行二进制哈希及最终测试源码匹配 |
| 外部账号及费用 | 未使用真实模型账号或付费额度 |

来源见 [环境分配与运行身份](environment/allocation.json)、[构建与隔离说明](environment/README.md)、[最终源码核对](final-source-audit.json)。页面实际 API/WS 地址与服务 PID 已核对。三个运行环境在同一台物理机器上，分别隔离账号配置和工作目录；这不是三台机器或操作系统沙箱测试。

## 3. 三方拓扑

| 角色 | 独立账号 ID（前 8 位） | 工作空间 ID（前 8 位） | Agent ID（前 8 位） | 主线实际执行次数 |
|---|---|---|---|---:|
| 产品 P | `660be9a7` | `0dea74be` | `848cd227` | 1 |
| 研发 D | `514b5116` | `4f36c01f` | `719b8cd9` | 2 |
| 测试 Q | `afccbd43` | `4aba8930` | `20f8df78` | 2 |

主线任务：`01a1bedb-5453-710d-9824-59623f8d595a`，始终归产品空间 P。D、Q 没有加入 P 的工作空间，仍可作为任务参与者完成处理。三方拥有不同的运行环境和安装身份，完整 UUID 见 [主线证据](closure-evidence.json)。

## 4. 主线逐步结果

| 步骤 | 实际操作与断言 | 结果 |
|---|---|---|
| 1 | P 创建 AI 辅助任务交给 D，附合成规则文件，邀请 Q；三方打开同一任务 | 通过 |
| 2 | 三方分别提供自己的私有资源，由 P 采纳；未经本人授权的目标不可用，P 替 D 授权被拒绝 | 通过 |
| 3 | 三位拥有者在各自页面授权；队列仍暂停，显式恢复后才可执行 | 通过 |
| 4 | P 发送需求指令；需求 Agent 实际运行，CLI 写回评论和上下文 v1→v2 | 通过 |
| 5 | 撤回 P 的授权后目标不可选；重新授权仍保持阻塞，需明确恢复 | 通过 |
| 6 | D 请求 P 补充回调规则；P 返回信息后，处理责任回到 D | 通过 |
| 7 | D 的修复 Agent 执行第一轮，读取 v2 并写入 v3 | 通过 |
| 8 | D 转交 Q；Q 的验证 Agent 读取 v3、写入 v4，返回预设的“缺少审计依据”结果 | 通过 |
| 9 | Q 提交第一版交付；P 退回并指定 D 修改，保存退回原因和原提交 ID | 通过 |
| 10 | D 的修复 Agent 再次执行，读取 v4、写入 v5 | 通过 |
| 11 | D 调用 Q 已授权的验证 Agent，读取 v5、写入 v6；资源仍属于 Q | 通过 |
| 12 | D 提交第二版结果；P 验收该准确提交 ID，任务变为 `done` | 通过 |
| 13 | 三个 Web 会话读取一致终态；API 核对两版交付及决定；无关私有任务快照保持不变 | 通过 |

五轮运行均为单次实际启动，实际数量为 P/D/Q = **1/2/2**；每轮都观察到 `host_exit_observed`，本地账本均已封存且退出证据已获确认。Agent 成功后任务仍未完成，只有 P 验收第二版交付后才进入 `done`。

| 轮次 | Agent | 共享上下文 | 实际 run ID |
|---|---|---|---|
| 1 | 需求 | v1 → v2 | `9bc98f0f-3cb8-42a1-adbf-bf3986cef623` |
| 2 | 修复 | v2 → v3 | `dd496464-8119-4a8d-84d0-b4c5ba3703ca` |
| 3 | 验证 | v3 → v4 | `5f6e37e5-32ba-49df-887d-6145bef723db` |
| 4 | 修复 | v4 → v5 | `201778de-fd16-4acc-a398-f26f171b93a0` |
| 5 | 验证 | v5 → v6 | `d4111f87-7003-4512-9322-b571bb32092f` |

第一版提交 `7afabb18-5a68-4b2b-add0-8d880830a906` 被 P 退回给 D；第二版 `6850fec4-e63a-48fa-8d20-7d4a5d923f10` 被 P 接受。历史中两个提交及各自决定均保留。

截图：[首次交付退回](screenshots/first-delivery-returned.png)、[产品视角完成态](screenshots/completed-product.png)、[研发视角完成态](screenshots/completed-development.png)、[测试视角完成态](screenshots/completed-quality.png)。

## 5. 权限与异常边界

| 编号 | 本次实际检查 | 结果 |
|---|---|---|
| B01 | 无关账号 X 读取任务和共享文件被拒绝，响应不含任务标题 | 通过 |
| B02 | D/Q 读取 P 的另一条普通私有任务被拒绝；源空间资源目录不可读；各账号仍只属于自己的空间 | 通过 |
| B03 | 登记和采纳资源后，未授权目标不出现在可用提及目标中，未启动进程 | 通过 |
| B04 | P 尝试替 D 授权资源，返回 403 | 通过 |
| B05 | 三方授权后队列仍暂停，未自动启动；明确恢复是独立动作 | 通过 |
| B06 | 普通评论保存成功，但该评论回执中的触发目标为空，执行指令数仍为零 | 通过 |
| B07 | 同 key、同正文重复提交第一条 AI 评论，回执与指令 ID 不变且没有额外运行；邀请原请求重放与创建/完成原结果查询也通过 | 通过 |
| B08 | 实际进程停止未确认时拒绝切换；退出确认后仍需新资源授权和显式恢复；旧停止输入不重跑 | 通过，独立 switch 场景 |
| B09 | 撤回授权使目标不可用；重新授权不解除 hold；显式恢复后解除 | 通过，主线测试针对已结束的运行 |
| B10 | 用过期上下文版本写入返回 409，不覆盖最新内容；之后五轮接续成功 | 通过 |
| B11 | 每轮 Agent 成功后任务不为 done；最终人工验收后，三方的提交指令能力均为 false | 通过；终态未额外发送一次被拒绝的 AI POST |
| B12 | Leader 派活、成员执行与评论回写、Leader 接续，以及后续指令加入/排队 | 通过，独立 squad 场景 |

B12 的小队成员属于同一资源拥有者；本次多拥有者证明来自主线三个独立 Agent，不能把补充小队场景表述成跨拥有者小队测试。B09 未覆盖“撤权发生在进程运行中”的独立竞态；B08 覆盖的是配置切换触发的实际 Stop。

## 6. 执行批次与数量

| 批次 | 场景 | 通过 / 失败 / 跳过 | 耗时 | 证据 |
|---|---|---:|---:|---|
| run-01 | 主线首跑 | 0 / 1 / 0 | 22.39 秒 | [原始结果](run-01/playwright.json) |
| run-02-host | 小队、停止与运行环境切换 | 2 / 0 / 0 | 35.67 秒 | [原始结果](run-02-host/playwright.json)、[小队证据](squad-evidence.json)、[切换证据](switch-evidence.json) |
| run-03-memory | 两运行环境上下文接续 | 1 / 0 / 0 | 15.16 秒 | [原始结果](run-03-memory/playwright.json)、[上下文证据](memory-evidence.json) |
| run-04-closure | 修正测试断言后的完整主线 | 1 / 0 / 0 | 66.85 秒 | [原始结果](run-04-closure/playwright.json)、[主线证据](closure-evidence.json) |

最终场景集合为 **4 通过、0 失败、0 跳过、0 自动重试**。包含首跑的全部执行历史为 **4 通过、1 失败**；首次失败没有删除。最终通过的场景合计观察到 **13 次实际模拟 Provider 启动**：主线 5、小队 4、切换 2、上下文 2；加上首次失败前的 1 次，共 14 次。切换测试中另有一次证明从未启动的 continuation，不计入启动次数。

新增测试通过严格聚焦 TypeScript 检查、ESLint、Provider 编译和 `go vet`；环境的 API、CLI、daemon、Web 和 renderer 均构建成功。未执行全仓 `make check`，它默认使用 legacy 认证，不等同本次 password/managed 执行环境。

## 7. 发现与处理

**HARNESS-001：首次断言错误，已修正并重跑通过。** 首次运行已完成第一个 Provider 的实际执行，但测试错误地期待公开评论响应提供 `author_id`。当前实现只在普通成员评论填充 `author`，Agent 评论为 `author: null`。修改新测试后，通过准确的评论 ID/正文与该 run 的记录关联，并进一步核对运行资源与共享上下文署名中的 Agent、owner、runtime，保留身份核验强度。

这是本次新增测试的问题，未作为产品缺陷计数。见 [发现记录](findings.json) 和 run-01 原始失败记录。三个主线页面的 `pageerror` 均为空；这不代表做了全站网络、性能或无障碍审计。

环境准备阶段一次 psql 前置检查误把连接串传入 `PGDATABASE` 而被拒绝，尚未执行迁移。随后改为明确传入各连接参数，核对期望库名后，在同一个归属本任务的新数据库迁移成功，未向其他数据库写入。Web 构建有 CSS `::highlight` 优化器警告，但构建退出 0。

## 8. 清理与复现

测试后三个模拟运行环境均正常退出，主线清理错误为空；协作门恢复为测试前状态（execution=false、creation=true、host=false）。API/Web/renderer 已关闭，三个任务端口均不再监听；独立测试数据库和证据保留。见 [清理核对](cleanup.json)、[最终门状态](final-gate.json)。

本次运行入口如下；重新运行前需用同目录 `services.py start` 启动保留环境，再用 `services.py verify` 核对。保留环境只供本任务使用。选择新的 `QA_RUN_ID`，不覆盖旧证据：

```sh
python3 artifacts/qa/collaboration-multi-account-20261109/environment/run.py --provider closure env QA_RUN_ID=rerun-01 pnpm exec playwright test e2e/collaboration-multi-account-closure.spec.ts --config playwright.qa.config.ts --workers=1 --retries=0
python3 artifacts/qa/collaboration-multi-account-20261109/environment/run.py --provider host env QA_RUN_ID=rerun-02 pnpm exec playwright test e2e/collaboration-host-squad.spec.ts e2e/collaboration-host-switch.spec.ts --config playwright.qa.config.ts --workers=1 --retries=0
python3 artifacts/qa/collaboration-multi-account-20261109/environment/run.py --provider memory env QA_RUN_ID=rerun-03 pnpm exec playwright test e2e/collaboration-memory-runtime.spec.ts --config playwright.qa.config.ts --workers=1 --retries=0
```

机器可读汇总：[results.json](results.json)。完整证据中的凭据形态扫描未发现匹配值，见 [扫描结果](credential-scan.json)。测试新增三个源文件，不新增依赖，不改变产品行为或已有测试流程。

## 9. 未覆盖项

- 商业模型真实推理、实际支付系统修复、账户额度与限流。
- 第二台物理机器、其他操作系统、跨服务器协作。
- 移动端和安装版 Desktop 的完整用户流程；renderer 构建和服务验证不等于安装版验收。
- 跨拥有者 AI 小队、运行中撤权竞态、全部离线恢复及配置切换组合。
- 全仓回归、性能压测、全站视觉和无障碍审计。

以上项目未标记为通过；它们不影响本次已执行的三方模拟协作闭环结论。
