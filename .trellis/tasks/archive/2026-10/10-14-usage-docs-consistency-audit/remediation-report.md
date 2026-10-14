# 使用说明一致性修订结果

日期：2026-10-14。**D01–D18 均已修订并通过独立复核和文档集成验证。**

共修改 18 篇中文文档及英文开发规范，重新生成对应的 18 个页面 JSON 和目录中的本轮摘要变更。未修改业务实现。原始审计发现保留在 [audit-report.md](audit-report.md)，本页记录修订后的结果。

文档工作提交：`4d7399b74`。任务已完成并归档。

## 逐项关闭

| 发现 | 修订结果 | 源文件 |
|---|---|---|
| D01 · P1 | URL 导入仅支持已实现的来源；内网改用本地文件，并说明来源更新条件。 | [skills.zh.mdx](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/skills.zh.mdx)、[cli.zh.mdx](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/cli.zh.mdx) |
| D02 · P1 | 分开说明分配/状态变更与评论触发；backlog 不能用作全局静默开关。 | [triggering-agents.zh.mdx](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/triggering-agents.zh.mdx) |
| D03 · P1 | 分开说明离线显示、默认 3 小时重连宽限、准备租约与实际重启回收。 | [tasks.zh.mdx](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/tasks.zh.mdx)、[daemon-runtimes.zh.mdx](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/daemon-runtimes.zh.mdx) |
| D04 · P1 | 快速上手和 Desktop 按设备身份、用户名密码、邮箱验证码说明登录。 | [self-host-quickstart.zh.mdx](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/self-host-quickstart.zh.mdx)、[desktop-app.zh.mdx](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/desktop-app.zh.mdx) |
| D05 · P2 | 环境变量管理包含智能体的人类所有者；智能体身份不可调用管理接口。 | [agents-create.zh.mdx](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/agents-create.zh.mdx)、[cli.zh.mdx](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/cli.zh.mdx) |
| D06 · P2 | Oh-My-Pi 支持托管 MCP；只在下发配置时检查已有文件冲突。 | [providers.zh.mdx](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/providers.zh.mdx) |
| D07 · P2 | 补齐 ZeroClaw 命令、最低版本、模型/MCP 及会话恢复条件。 | [install-agent-runtime.zh.mdx](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/install-agent-runtime.zh.mdx)、[providers.zh.mdx](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/providers.zh.mdx) |
| D08 · P2 | 区分邮箱邀请与分享链接，说明角色、有效期、轮换、撤销与账号前提。 | [members-roles.zh.mdx](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/members-roles.zh.mdx)、[tutorial.zh.mdx](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/tutorial.zh.mdx) |
| D09 · P2 | @all 仅抑制隐式路由，不覆盖同条评论显式提及的目标。 | [comments.zh.mdx](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/comments.zh.mdx) |
| D10 · P2 | 说明正文编辑取消旧输入执行、删除尝试取消，以及已产生结果不会回滚。 | [comments.zh.mdx](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/comments.zh.mdx) |
| D11 · P2 | 显式提及优先，未被提及的原队长不会额外被唤醒。 | [squads.zh.mdx](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/squads.zh.mdx)、[tutorial.zh.mdx](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/tutorial.zh.mdx) |
| D12 · P2 | 按认证模式拆分 Redis/内存限流和故障策略，邮箱邀请单独说明。 | [environment-variables.zh.mdx](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/environment-variables.zh.mdx) |
| D13 · P2 | 明确原地目录直接写入、同 daemon 非对话串行、对话免锁及工作树并发门槛。 | [security-model.zh.mdx](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/security-model.zh.mdx)、[project-resources.zh.mdx](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/project-resources.zh.mdx) |
| D14 · P2 | Desktop 支持内网 HTTP；HTTPS 建议与 API/WebSocket 可达性分别说明。 | [desktop-app.zh.mdx](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/desktop-app.zh.mdx) |
| D15 · P2 | 中英文规范保留 snake_case 数据字段，查询键示例携带 wsId。 | [conventions.zh.mdx](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/developers/conventions.zh.mdx)、[conventions.mdx](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/developers/conventions.mdx) |
| D16 · P2 | 标识符保留英文；内置状态/角色按界面语言显示，自定义状态保留名称。 | [conventions.zh.mdx](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/developers/conventions.zh.mdx)、[conventions.mdx](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/developers/conventions.mdx) |
| D17 · P3 | 补齐 approval 命令，并说明人工审批与执行结果登记的权限条件。 | [cli.zh.mdx](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/cli.zh.mdx) |
| D18 · P3 | 教程使用当前导航及五种 skill 创建方式，标明五张旧截图的差异。 | [tutorial.zh.mdx](/Volumes/artisan/code/2026/multica-0.4.37/apps/docs/content/docs/tutorial.zh.mdx) |

逐项实现依据与复核细节见 [独立审阅记录](research/remediation-review.md) 和 [机器可读关闭矩阵](research/remediation-findings.json)。D13 的项目资源页是核对关联描述时补充的同一问题修订；英文规范只同步 D15/D16 的共同契约。

## 验证结果

| 检查 | 结果 |
|---|---|
| 文档 Vitest | 9 个文件、62 个测试通过 |
| docs 类型检查 | `tsc --noEmit` 通过（独立 reviewer 执行） |
| MDX / 文档站生产构建 | 类型生成通过；`next build` 成功编译并生成 99 个静态页面 |
| Go 文档包 | 嵌入内容、白名单、ETag 与 CLI/服务端依赖范围测试通过；`go vet` 通过 |
| Docs handler | 12 个测试实际运行并通过，无数据库跳过；覆盖目录、页面、资源、缓存与锚点 |
| 文档包一致性 | 39 页、58 个资源，共 98 个文件逐字节一致 |
| 内部引用 | 252 个内部链接、43 个锚点、57 处图片引用，零断链或缺图 |
| 临时数据库 | 从本地迁移齐全的测试库克隆，仅用于 Docs handler；测试后已删除 |

日志、退出码、哈希与时间见 [最终验证证据](research/remediation-final-verification.json)、[链接与包检查](research/remediation-bundle-and-links.json)、[数据库清理记录](research/remediation-database.json)。docs 没有独立 lint 脚本；源文件空白检查、类型检查和实际 MDX 编译均已通过，没有新增逐字断言文案的测试。

## 维护规则与交付边界

- 已在 docs bundle spec 中记录：网络可达性不能替代导入来源支持；条件行为必须按源码分支说明；关联页面和旧截图须同步交代；包一致性与正文准确性分别验证。
- 本次验证基于共享工作区中的 39 页版本，包括此前已有的 auth-setup、分拣台、迭代、项目等文档更新。提交仅包含本轮明确修改的内容，既有修改留给原会话；这不是独立 checkout 或线上安装包的版本证明。
- 五张教程截图已核对并标明旧界面，没有重新制作。没有进行真实 AI 调用、登录流程、3 小时断线、Redis 故障或安装版 Desktop 的现场验收。
- pnpm 提示根 `package.json` 中已有的部分 pnpm 配置被当前工具忽略；本轮构建与测试通过，未改变依赖、锁文件或包管理配置。
- 未发布或部署。应用内文档通过 Go 嵌入包提供，正在运行的旧服务需要后续正常构建交付才会包含这些更新。
