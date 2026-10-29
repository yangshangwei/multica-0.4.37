# 能力市场最终验收

已完成已批准的能力市场提取、Web/Desktop 接线、独立审查和最终验收。基线为 `5049e0f6a4ba5801876c7e821418a23e59262c4c`，分支为 `feat/capability-market`。本任务保留原技能库、MCP 管理及智能体上下文入口；无新依赖、API 或数据库迁移。

## 最终结果

| 检查 | 结果 | 证据 |
| --- | --- | --- |
| 全部非 Mobile 单测 | 945 文件 / **11,406 项通过**，最终失败 0 | [check-results.json](./evidence/check-results.json) |
| lint | 强制运行 6 项任务、0 缓存，通过；31 项已有 warning | `.omx/artifacts/capability-market-final/review/lint-final.log` |
| TypeScript | 强制运行 9 项任务、0 缓存，通过；测试夹具修改后另跑 Views 检查 | 同目录 `typecheck-final.log`、`provider-recovery-types.log` |
| 任务 E2E 静态检查 | 严格 TypeScript 与适用现有 base ESLint 配置的直接检查通过，未忽略测试文件 | `acceptance/e2e-typecheck.log`、`e2e-eslint-task.log` |
| 生产 Web / Electron | **25 个唯一 E2E、10 个文件通过，最终无跳过**；重复与失败尝试单独保留 | [acceptance-final.md](./evidence/acceptance-final.md) |
| 三端构建 | 隔离 production Web、原生 Desktop、Docs 均通过 | 验收构建来源及 `docs-build.log` |
| 内嵌文档 | 39 页、58 资源重新生成后逐字节一致；Go docs 包 `-count=1` 通过 | [docs-bundle.json](./evidence/docs-bundle.json) |
| UI 导出 | 62 文件检查通过 | `node scripts/check-ui-wildcard-exports.mjs` |
| Knip | 原配置下基线与最终各 13 项已有报告，新增 0 | [knip-comparison.json](./evidence/knip-comparison.json) |
| 差异/任务清单 | `git diff --check` 与上下文清单校验通过 | 直接执行；7 项 implement / 8 项 check context |

单测按包计数：Core 3,057，Docs 62，Views 6,907，Web 283，Desktop 1,097。初次全量运行在 Views 的两条旧路径 mock 失败后由 Turbo 停止；补齐夹具后重跑完整 Views，并单独执行 Web/Desktop。各包最终结果均来自实际新执行；不把定向重复执行加进 11,406。

浏览器唯一用例计数包含 21 条市场/语言/默认值/上下文 Web 用例、3 条真实管理员发布用例及 1 条原生 Electron 完整流程。管理员用例也实际启动原生消费者。最终构建还重跑 5 条 MCP Web 用例；共享部署夹具最后修改后另跑其 Web 用例。它们属于重复复验，不重复计算。

## 本轮修复与失败分类

- 独立审查复现：智能体上下文打开旧版模板的自定义编辑器后，原模板撤下，关闭无法回到目录搜索。显式转发 `onCustom(preset, finalFocus)`；红测 1 失败 / 5 通过转为 3 文件 56 项通过。普通关闭原本可由 Base UI 返回卡片，不列为失败，也不宣称未证实的控制台警告。
- 全量单测发现 `provider-settings-recovery.test.tsx` 未提供 MCP 管理页新增消费的 `useWorkspacePaths`。以实际纯路径 builder 补齐 mock，保留恢复/失权断言，23 项通过。
- 首轮 Web 两项测试问题：工作空间可访问名称含头像文字；草稿未丢弃时仍断言旧 `/skills` URL。修正选择器/市场 URL 后，定向 4 项通过，包含新增隔离与故障/轮询验证。
- 原生新增用例采用实际支持的鼠标侧键历史路径。弹窗拦截方向快捷键是既有策略；不修改全局导航。跨路由断言等待持久化当前路由和目标卡片，消除旧详情 DOM 过渡期歧义。
- 管理员发布与原生目录用例最初并行，临时模板污染固定数量断言。保留原断言，清理后串行执行；已记录 API 级目录共享规则。
- 原生弹窗初始 autofocus 早于测试填值完成，模拟令牌落入名称字段。等待预期初始焦点，填写后断言 password 值；原生完整流程及共享 Web 夹具复验通过。

所有失败尝试与最终结果分别保留，未使用强制点击、睡眠或放宽产品规则来遮盖失败。

## 验收标准映射

| 标准 | 实际证据 |
| --- | --- |
| AC1 导航及类型恢复 | core paths/标签/图标、sidebar、命令搜索、编辑器链接、诊断路由测试；Web URL/Back/Forward/刷新与原生新标签恢复 |
| AC2 目录发现及关联 | Skill/MCP 目录单测；真实部署目录、全部 16 个内置 MCP 配方、改名/多副本关联浏览器验收 |
| AC3 管理页分离 | Skills/MCP 管理页单测与 Web/Electron 显式市场入口、空态、列表、详情、设置管理 |
| AC4 Skill 创建和恢复 | 实际创建留页、打开副本、草稿历史恢复；既有 dirty/unknown-result 会话单测和独立副本 E2E |
| AC5 MCP 保存及分配 | 实际仅保存、继续分配、失败重试不重建、成员复用；disabled 保持由核心/视图回归验证 |
| AC6 权限及上下文 | 普通成员/owner、Agent target/runtime 视图测试及真实成员复用、Agent 上下文和后台 403 |
| AC7 隔离及可访问性 | 真实两个工作空间资源/偏好/临时草稿隔离；EN/ZH、360/375px、键盘和关闭焦点截图/断言 |
| AC8 故障及生命周期 | 真实两类首载失败、缓存刷新恢复、30 秒仅活跃面板轮询；真实部署模板修改/撤下；既有 Skill 已采用草稿保护 |
| AC9 旧偏好退役 | 持久化 merge 丢弃 libraryView；来源/分类仍按工作空间保留，unit + 跨工作空间 E2E |
| AC10 后端与凭据边界 | 无新增 API/迁移/依赖；模板发布/摘要安全/源身份实测；新市场 mutation reset/gcTime 回归及独立源码审查 |

## 源码、环境及风险

[最终源码清单](./evidence/final-source-manifest.json)记录 76 个功能/规范文件。主代理独立比对隔离验收检出的 68 个应用及测试文件，全部一致。production Web 使用 `next build` + `next start`；实际启动来源、构建 ID、环境模式和原生执行记录见验收文档。未将 next dev 冷编译或 mock 结果冒充生产/原生证据。

普通根目录 Knip 会扫描已有 `.artifacts/release-v0.6.6/*-source`，产生 1,840 项重复检出依赖报告。因此使用新的 `git archive HEAD` 干净快照、同一依赖与原 Knip 配置，先跑基线再覆盖最终实现。基线/最终均为 11 个未使用文件加 2 项依赖报告，逐项一致；本轮未清理无关历史代码。

[独立审查记录](./evidence/source-review.md)记录了两处已存在于基线的旧自定义 MCP mutation 内存生命周期行为，本任务未新增，单独注明而不扩大导航改动。新市场创建会清空 settled mutation 并使用 `gcTime: 0`。本轮验证配置保存与分配，没有启动真实 MCP 提供方或调用工具；Windows/Linux 打包未重新执行，Mobile 不在批准范围。

本任务移除原管理页市场组合、废弃 libraryView 与重复入口，复用既有创建/分配状态机。保留其他任务的未提交审计、历史验收及 journal 内容。清理结果及保留的隔离 DB/profile 见 evidence/acceptance-final.md。

原始日志和所有尝试位于 `.omx/artifacts/capability-market-final/`；前一轮 Knip 日志在 `.omx/artifacts/capability-market-resume/`。实现前基线未留存，不补记为通过。
