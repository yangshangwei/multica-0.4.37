# 调试 / RCA 能力审计

## 范围与修复计划（2026-09-23）

保留 D1-D3：新增诊断角色和独立方法 skill，contributor，一角色一技能；不修改 bug-fix / incident 路由。审计提交前实现及其真实消费链路。

1. 对照 R1-R5、AC1-AC8 检查注册、顺序、四语言、分类、版本和模板复制语义。
2. 修复已证实的内容契约缺口：间歇失败的触发范围、无修复时失败的回归测试交接、可复核证据、隔离和清理、未查明时的完成条件；将固定两假设上限改为无进展或实际受阻时升级。
3. 补齐前端内置 skill 识别与四语言名称 / 简介，先用现有展示测试证明缺项。
4. 补充诊断角色真实数据库创建、技能默认展示元数据及自定义副本保留测试；扩展已要求的内容契约检查。用独立数据库执行，核对实际运行数量。
5. 修正文档旧分类、已解除的依赖、前端无改动假设、回滚无数据残留的错误表述，并更新用户文档 / 操作 skill / spec。
6. 运行 Go 构建、service / handler 测试、Go 静态检查、前端测试 / 类型 / lint；复审最终 diff，记录证据与剩余限制。

## 已修复的发现

- 前端未识别 `multica-debugging` 为内置 skill，导致错误分组、名称 / 简介不本地化。
- 角色完成标准要求所有结论定位到文件，和允许疑似 / 未查明结果相冲突。
- 技能没有明确要求独立挂载时仍在隔离分支调查，也没有落实 D2 的失败回归测试交接。
- 两个假设被排除就必须停止，误将正常缩小范围当作调查失败。
- 分类依赖已由 `28c91e13d` 提交；旧规划仍写 engineering 和暂停实施。
- 代码回滚不会删除已经 materialize 的工作区 agent / skill，设计的“无数据残留”不成立。
- 安全异常原先写“请求人工介入，再继续”，与停止要求矛盾；已改为报告后停止。报错行或一次实验成功也不再自动视为根因。
- 日 / 韩语虽然能显示译名，原搜索索引没有当前语言名称和简介；已补入现有搜索字段。

## 最终处置

上述问题均已修复。沿用既有模板注册、创建、物化和前端展示 helper，没有新依赖、迁移、API 或额外权限机制。删除固定两假设上限和文件行号必须存在的过度约束；测试能自动化时交接实际失败的用例，否则如实交付复现步骤或缺失条件。保留 D1-D3，不修改小队路由。

变更文件集中在：

- `server/internal/service/builtin_agent_templates/diagnostician/INSTRUCTIONS.md`、`builtin_role_skills/multica-debugging/SKILL.md`、roster / version map 和内容契约测试。
- `server/internal/handler/agent_template_test.go`、`skill_template_test.go`：模板列表及真实数据库创建 / 默认值 / 工作区自定义副本保留。
- `packages/views/skills/lib/skill-presentation{,.test}.ts` 与四语言 `locales/*/skills.json`：内置分组、本地化、搜索和源文本同步。
- 四语言 `apps/docs/content/docs/agents-create*.mdx`、对应服务端内置文档页、`multica-creating-agents` 正文及 source-map。
- 任务 PRD / design / implement / 验收记录和 server / views 的模板、本地化 spec。

## 本次审计自测证据

本次审计另在独立数据库 `multica_rca_audit_1790133548585` 上执行检查；测试后已删除该数据库、临时 Redis 容器及连接文件。日志保留在 `/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-rca-audit-z0RQoY/`。

| 检查 | 实际结果 |
| --- | --- |
| 新增内容契约保护，修复前运行 `TestDiagnostician_EvidenceContract` | 按预期失败，指出隔离分支、失败测试、退出码、已有改动保护缺项；修复后通过 |
| 模板 / 创建 / 技能目录定向 Go 测试 | 43 个测试及子用例通过，0 跳过 |
| `go test ./internal/service/... ./internal/handler/... -count=1 -json`，使用 agent CLI guard | 4,294 个测试及子用例通过，66 跳过，0 失败；数据库测试实际执行 |
| 独立 Redis，按初次跳过项及新增诊断测试补跑 `go test -race ... -run "$RCA_TEST_FILTER" -count=1 -json` | 68 个测试及子用例通过，0 跳过、0 失败，无 data race；其中 64 项消除初次环境跳过 |
| 上述完整运行与补测按 package / Test 去重 | 4,358 通过，2 个仓库原有显式跳过，0 失败 |
| `go build ./...`、`go vet ./...` | 均通过（Go 1.27.0，darwin/arm64） |
| 前端本地化 / 搜索 / 列表 / 详情 / 模板 / slash / tab / locale parity | 12 套件 388 用例通过；缺内置登记、日 / 韩搜索已先红后绿 |
| 文档测试 | 8 套件 60 用例通过；四语言角色列表与本次生成页一致 |
| `pnpm typecheck` | 9 个任务成功 |
| `pnpm lint` | 6 个任务成功，0 错误，33 条既有警告；修改的 helper / 测试定向 lint 无警告 |
| Go 格式、`git diff --check`、Trellis context validate | 均通过 |

两个原有显式跳过为 `TestWebhookHandler_DBErrorOnTokenLookupReturns500`（尚未注入错误 stub）和 `TestFetchFromSkillsSh_AnthropicPptxIntegration`（需主动启用真实 GitHub 集成）。并发验收者还对最终代码执行了完整 race 套件，单独证据见 [verification.md](./verification.md)。本表只记录本次审计实际发起的检查，不混用另一轮的资源或命令。

## 独立 skill 前向试用

独立 Codex 子代理只读取最终角色 / skill 和临时 Python cache 工件，接收一个已观察到的 TTL 失败，没有读取审计结论或预设修复答案。

- 实际 5/5 复现相等边界仍返回旧值；到期前、到期后对照及 `sys.settrace` 共同定位 `now > expires_at` 遗漏等号。代理排除两个假设后继续第三个有证据的方向，未因固定次数停止。
- 交付标准库 `unittest`：3 项中 2 通过、到期边界 1 失败，退出码 1。该失败是诊断交付物的预期状态，不是产品测试回归；没有提交正式修复。
- 主审读取并复核了测试与结构化证据：`cache.py` 无改动，README 原有未提交内容保留，没有遗留代码插桩。
- 对只有调用方口述超时、无日志 / 环境权限的第二场景，报告明确为运行 0 次、未查明，没有冒充实测。
- 报告与证据位于日志目录的 `forward-test/diagnosis.md`、`test_cache_ttl.py`、`repro-evidence.json`、`regression-output.txt`。

这只是一次隔离夹具试用，未覆盖多种真实项目和生产运行。演练曾有错误路径 / 工作目录的工具参数，被工具拒绝且未写入；不能将这次试用宣称为模型操作边界的全面保证。实际部署后的代理行为仍需按现有权限政策约束。

## 剩余限制

- 本任务范围内未发现尚未修复的阻塞问题。AC1–AC8 已完成，代码随后以 Lore 格式提交为 `e348b009a`。
- 未运行浏览器 E2E、移动端或已部署 Multica runtime 的真实代理流程；本次没有页面布局或路由变更。
- 已有工作区角色 / skill 副本不会自动升级，代码回滚也不会删除这些实例，这是既有产品契约。
- 文档生成器发现 `self-host-quickstart.json`、`skills.json` 的既有无关漂移，本次只同步 `agents-create.json`。全仓既有 lint 警告和两项显式跳过均已披露，未为通过检查而放宽断言。
