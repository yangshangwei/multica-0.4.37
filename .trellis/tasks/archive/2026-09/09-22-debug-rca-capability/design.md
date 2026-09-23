# 设计:调试/RCA 能力(诊断工程师 + multica-debugging 技能)

## 依赖与最终取值（2026-09-23 更新）

技能分类改造已在 `28c91e13d` 提交，任务已在 `2350999ce` 归档。9 月 22 日记录的并发修改风险已解除，本实现沿用新的分类契约：

- `multica-debugging` 与 test-report/code-review 同属 `quality`（测试与质量）。
- 图标选定为白名单中的 `microscope`。
- `TestRoleSkillTemplates_PresentationDefaults` 已在新分类基线上增加 debugging 条目。

## 架构与边界

两个配套交付物,沿用现有「角色模板 + 角色技能」两分,均为简体中文、创建时拷贝进工作区、工作区拥有、升级不覆盖:

- **角色提示词** `builtin_agent_templates/diagnostician/INSTRUCTIONS.md` = 这个角色是谁、读什么、交付什么、不做什么、何时升级(WHO / 契约)。
- **角色技能** `builtin_role_skills/multica-debugging/SKILL.md` = 可复用的 RCA 方法(HOW),materialize 成工作区技能后可挂到别的 agent、可被 squad leader 按能力委派。

## 角色定义(roster 条目)

- `Key: "diagnostician"`(稳定、不本地化、不改名);`Version: 1`;`Listed: true`。
- `DefaultName: "Diagnostician"`;Titles en=Diagnostician / zh=诊断工程师 / ko / ja;Descriptions 四语言一句话(定位失败根因、给出带证据结论、不发修复)。
- `Autonomy: AutonomyContributor`(D2);`MaxConcurrentTasks: 1`(与 implementer 同:复现/插桩会占用一个工作树)。
- `AvatarEmoji: "🩺"`，与现有角色不重复。
- `RoleSkills: []string{"multica-debugging"}`。
- 位置:插在 `qa-engineer` 之后、`code-reviewer` 之前(roster「按改动流经团队的顺序」——失败定位在实现/测试之后、评审之前)→ 列表 9→10。

## 技能定义(multica-debugging)

- frontmatter：`name: multica-debugging`；`description` 说明何时使用和产出；`user-invocable: false`；`metadata: { category: quality, icon: microscope }`。
- body(house style 章节):何时使用 / 提交调查前先读什么 / 复现与最小化 / 形成并验证假设(插桩·日志·二分·对照)/ 结论必须有证据(根因 vs 假设)/ 输出格式(复现步骤·最小用例·根因+证据链·建议修复方向·未确认项)/ 以下情况停止并询问人工。
- 边界(R2)与证据契约(R3)显式写进正文。

## 数据流 / 契约

- 无 DB / schema / API 契约变更:纯内置内容(embed)+ roster 切片 + version map + 两个 Markdown 文件。
- materialize 路径已存在:`from-template` 创建 diagnostician 时自动 materialize + attach multica-debugging。
- 角色 picker 读取 `AgentRoleTemplates()` / templates API，自动多出一个可选角色，无需新增页面或路由。skill 展示另有内置名称表与四语言目录，工作区已并发补充 `multica-debugging` 的识别、名称和描述；正文及存储身份保持不变。

## 兼容性 / 回滚

- 纯增量,无迁移;旧 agent 不受影响(autonomy 空值兼容规则不变)。
- 代码回滚可还原目录、version map、roster 和测试。若发布后已创建工作区角色或 skill 副本，代码回滚不会删除这些普通实例，也不会覆盖其自定义内容；数据清理由后续显式操作处理。

## 测试影响(计数/清单被 pin,漏改即红)

- `TestAgentRoleTemplates_ListedRosterIsTen`：角色数为 10，清单在 qa-engineer 与 code-reviewer 之间包含 diagnostician。
- `TestAgentRoleTemplates_DefaultRoleSkills`:want 加 `"diagnostician":{"multica-debugging"}`(该测试断言 len 相等,必须加)。
- `TestAgentRoleTemplates_AutonomyDefaults`:want 加 `"diagnostician":contributor`;operator 计数仍为 1(不受影响)。
- `TestRoleSkillTemplates_RosterIsNine`：角色 skill 数为 9，multica-debugging 排在 code-review 与 documentation-change 之间。
- `TestRoleSkillTemplates_PresentationDefaults`：want 加 `"multica-debugging":{"quality","microscope"}`。
- 自动满足即通过(无需改测试,但要满足):`_LocalizedCopyIsComplete`、`_RoleSkillsExist`(≤1 skill、multica- 前缀、有 description、version≥1)、`_InstructionsCarryTheContract`、`EveryEmbeddedSkillIsRegistered`、`FilesMatchSource`。
- 新增 `TestDiagnostician_EvidenceContract`(仿 progress-reporter):把 R2/R3 关键词(复现、最小化、假设、插桩、根因、证据、不发修复、test-report、code-review)钉进角色 + 技能文本。
- handler 的 `agent_template_test.go` 角色数已改为 10，`skill_template_test.go` 模板清单已增加 debugging；完整 handler 验收同时覆盖 `skill_presentation_test.go`。

## 文本契约复核

- 已确认根因要求定位到代码位置或依赖 / 配置 / 环境机制，给出因果证据；疑似或未查明必须如实说明证据缺口，不编造未获得的交付物。可自动化时保留无修复时失败的回归测试，否则交付可执行步骤及限制。
- 排除假设是调查进展，不能照搬 bug-fix 的失败修复次数作为 RCA 的假设数上限。连续无新证据、重复方向、达到约定预算或确实缺少访问权限时交接。
- 数据丢失、安全缺陷或凭据泄露触发报告后停止调查，由人工决定后续处理。
- `contributor` 沿用现有自主权限政策。禁止提交正式修复、读取生产数据及改变任务状态属于角色/skill 的更窄指令约束，不代表新增了服务端隔离机制。
- 新角色与 skill 尚未发布，保留初始版本 1。评审发现和最终结论见 `text-review.md`。
- 复现后编写并实际运行无修复时失败的测试，记录命令、退出码与关键输出；隔离分支保留他人已有改动。该约定已写入两份正文及证据契约断言。

## 关键权衡

- **9→10 值得**：RCA 是独立活动而非技术栈变体，与 code-review/security-review 从实现中拆出同理，符合 `ListedRosterIsTen` 的 distinct work 约束。
- **contributor 而非 observer**:RCA 结论要能自证,需自行复现 + 插桩(D2)。
- **不动小队**:改 bug-fix/incident 路由是产品契约变更(squads skill 要求单独确认),缩小本任务风险面,留作后续。
