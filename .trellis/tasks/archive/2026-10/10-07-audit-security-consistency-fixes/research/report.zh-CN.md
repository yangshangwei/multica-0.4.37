**代码审计报告 · 2026-10-07**

结论：发现 **2 项 P1（高优先级）和 4 项 P2（中优先级）**。建议优先修复桌面 HTML 预览的本地文件读取风险，以及插件桥接接口对任务凭据的身份提升问题。本轮仅审计，未修改业务代码、提交代码或创建 Trellis 任务。

使用的主要 Skills：`code-review`、`trellis-start`、`trellis-check`、`pnpm`、`verification-before-completion`。通过原生子代理分别检查后端权限、项目/迭代事务和共享前端；主会话复核关键调用链与运行证据。

审计开始于 `codex/projects-p1@70db509c2`。期间工作区合入 main，当前核对提交为 `4a9be02e5`；新增差异已检查，并补跑相关模块测试。以下发现均仍存在于该提交。本次重点覆盖认证及插件边界、桌面不可信内容、项目与迭代数据流，并非全仓逐行审查。

| 编号 | 优先级 | 问题 | 证据 |
|---|---|---|---|
| A1 | P1 | 桌面 HTML 预览可读取已知路径的本地文件 | 隔离 Electron 运行复现，真实调用链复核 |
| A2 | P1 | 任务令牌经插件桥接被当成人类成员使用 | Auth → bridge → handler → service 完整权限链核对 |
| A3 | P2 | 迭代表单以新版 revision 提交旧字段 | 真实组件挂载复现 + 服务端版本检查核对 |
| A4 | P2 | 桌面切换项目后把上一项目草稿提交到新项目 | 真实组件挂载复现 + 路由生命周期核对 |
| A5 | P2 | 并发关联本地目录绕过 daemon 唯一性检查 | SQL 及事务边界证明 |
| A6 | P2 | 资源局部更新覆盖另一请求的路径或执行模式 | Handler 读改写及 SQL 无版本检查证明 |

**A1 · P1：桌面 HTML 预览可读取本地文件**

定位：[renderer-web-preferences.ts](/Volumes/artisan/code/2026/multica-0.4.37/apps/desktop/src/main/renderer-web-preferences.ts:37)、[code-block-iframe.tsx](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/editor/code-block-iframe.tsx:48)。

当前主窗口与独立任务窗口均设置 `webSecurity: false`。评论中的闭合 HTML 代码块默认进入预览，`HtmlPreviewBody` 只追加片段导航脚本，原始内容进入 `srcDoc`，iframe 允许脚本执行。Markdown 的清理器不会移除代码块文本里的脚本。

隔离 Electron 39.8.7 使用与产品一致的 `sandbox: true`、`contextIsolation: true`、`nodeIntegration: false`，结果为：开启 webSecurity 时 `fetch(file://...)` 失败；关闭时成功读出临时假数据文件。父文档访问仍被阻止，说明父 DOM 隔离无法替代此处的文件读取隔离。

攻击者需能投递受害者会查看的评论代码块或 HTML 附件，并知道或猜中目标文件路径。风险限于应用进程本就可读取的文件；没有证明绕过操作系统权限、IPC 访问或命令执行。本轮没有读取真实敏感文件，也没有外传数据。实际预览链没有限制网络出口的 CSP；导航守卫只处理顶层导航，不处理 iframe 内的 fetch。

建议：让不可信 HTML 在启用 webSecurity 的独立浏览上下文运行，或完成自定义协议/CORS 配套后恢复该安全设置；在此之前，桌面端可暂时禁止不可信 HTML 执行脚本。补充真实 Electron 文件读取拒绝回归，单纯断言 sandbox 属性不足。

证据：[Electron 复现结果](/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-code-audit-jzu0hkn0/electron-file-proof.log)、[隔离复现脚本](/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-code-audit-jzu0hkn0/electron-file.cjs)。调用入口还包括 [comment-card.tsx](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/issues/components/comment-card.tsx:816)、[html-block-preview.tsx](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/editor/html-block-preview.tsx:58)、[html-preview-body.tsx](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/editor/html-preview-body.tsx:63)。

**A2 · P1：插件桥接把任务凭据提升为成员身份**

定位：[plugin_action.go](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/handler/plugin_action.go:127)、[router.go](/Volumes/artisan/code/2026/multica-0.4.37/server/cmd/server/router.go:1569)。

`/api/plugin-bridge/v1` 通过通用 Auth 认证，因而接收任务令牌。Auth 正确写入任务绑定的用户、Agent、任务、工作区及 `X-Actor-Source: task_token`，但 `pluginSessionCaller` 只取 `X-User-ID`，然后根据客户端指定的插件 installation 重新选工作区，检查该用户的成员身份，最终返回 `Type: member`。

这条路径不拒绝机器身份，也不比较任务绑定的工作区与插件工作区。若任务所属用户同时是 A、B 两个工作区的成员，持有 A 任务令牌的调用者知道 B 的有效 installation ID，就能经桥接使用 B 的成员身份，访问该 installation scopes 允许的资源。插件功能开关 `plugins_v1` 必须启用，installation 必须启用并具有对应 scope；这些条件限制可用端点，但没有恢复任务令牌的边界。即使在同一工作区，机器调用也被错误标记为成员。

建议：按注释约定使浏览器桥接只接受明确的人类会话身份；若产品确需机器调用，应单独保留任务身份、工作区绑定及 Agent 能力检查，不得仅凭令牌所属用户的成员身份授权。增加真实 Auth 中间件下的任务令牌负向测试，覆盖同工作区、跨工作区与 human-only hook。

证据边界：本项完成源代码调用链核对，未完成真实 HTTP/数据库攻击复现。审查子代理的模型服务两次中断；为复现创建的隔离数据库已由主会话清理，不将其算作通过的权限测试。

**A3 · P2：迭代编辑会静默覆盖其他用户的修改**

定位：[iteration-form.tsx](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/iterations/iteration-form.tsx:74)。

计划中迭代的名称、描述、日期、负责人只在组件初始化时进入本地 state。远端更新经实时事件刷新后，props 的 revision 已变化，表单字段仍是旧值；提交却使用新的 revision，并全量提交旧字段。组件按工作区/迭代 ID 保持挂载，服务端比较新 revision 后会接受该请求。

复现：挂载 revision 2，编辑本地描述，再以同 ID 的 revision 3、新名称和日期重渲染。捕获的请求包含 revision 3、revision 2 的名称/日期和本地描述。服务端 [iteration_lifecycle_metadata.go](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/service/iteration_lifecycle_metadata.go:236) 会通过版本检查并写入所有传入字段。

建议：表单字段和编辑基线 revision 一起保存；远端刷新不能替脏表单升级基线。保存成功或明确处理冲突后再推进基线，并优先只提交实际修改字段。

**A4 · P2：切换项目后，进展编辑器沿用上一项目正文**

定位：[project-detail.tsx](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/projects/components/project-detail.tsx:601)、[project-progress.tsx](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/projects/components/project-progress.tsx:204)。

桌面端同一未固定标签从 A 导航到已有缓存的 B 时，标签 ID 与 mountGeneration 不变。项目概览、ProjectProgress 没有按项目 ID 重挂载，composer 状态保留，编辑器 key 又固定为 create。ContentEditor 的 defaultValue 不随 props 更新正文，但预览/发布使用的项目 ID 已切到 B。

挂载复现确认：在 A 输入正文，切换 props 到 B，再预览和发布，两次请求都携带 B.id 与 A 的正文。桌面导航不重挂载由真实路由和标签代码链确认；未通过完整桌面 UI 重演这一流程。

建议：在共享详情、概览或编辑器适当边界按 workspace/project ID 重挂载，确保 A 草稿只写入 A，B 恢复自身草稿；回归应覆盖目标项目已有缓存、同标签导航的场景。

A3/A4 证据：[主会话重复验证日志](/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-code-audit-jzu0hkn0/client-reproductions.log)。两个外置 AUDIT 测试均确认错误行为，不是修复后通过；22 个非 AUDIT 用例被过滤，另有完整仓库测试结果见下文。

**A5 · P2：并发添加目录留下重复 daemon 绑定，导致任务拒绝执行**

定位：[project_resource.go](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/handler/project_resource.go:515)；更新路径的第 622 行同样存在检查与写入分离。

两个请求为同一项目、同一 daemon 添加不同路径时，可以同时通过无锁的冲突查询。项目锁直到后续 INSERT 的 CTE 内才获得，写入前不重新检查。数据库唯一约束比较整个 resource_ref，因此不同路径不会冲突。两次操作均可成功，随后 daemon 的 [local_directory.go](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/daemon/local_directory.go:193) 因多条匹配拒绝执行，需手动删除重复配置。

建议：在同一事务取得项目锁，再检查 daemon 唯一性并写入；同时补充两个并发 POST/PUT 的回归。此项通过实际 SQL 和事务边界证明，未新增数据库并发复现测试。

**A6 · P2：资源局部更新会回写旧的执行配置**

定位：[project_resource.go](/Volumes/artisan/code/2026/multica-0.4.37/server/internal/handler/project_resource.go:730)、[project_resource.sql](/Volumes/artisan/code/2026/multica-0.4.37/server/pkg/db/queries/project_resource.sql:53)。

Handler 第 590 行先无锁读取资源，随后把 resource_ref、label、position 全量写回。若 A、B 同时读到旧数据，A 保存新路径/执行模式，B 随后只改名称或排序，B 会把旧路径/模式一并覆盖回来。最终 UPDATE 内部的项目锁只能串行写入，无法保护之前的读取，也没有 CAS 检查。

现有 stale-ref 回归覆盖顺序请求与旧客户端字段合并，不覆盖两个服务端请求先后读到同一旧行的交错。

建议：在事务持锁后读取并合并局部字段，或使用明确的版本校验。增加“执行配置变更”与“仅改名/排序”并发时均被保留的回归。此项为源代码及 SQL 路径证明。

**验证与范围**

原始基线：Web/Desktop/Core/Views/Docs 共 874 个测试文件、10,219 个测试通过；15 个 lint/typecheck 任务通过，lint 0 errors、30 warnings；`go vet -p 2 ./...` 通过；UI exports 检查通过。

数据库从空库迁移至 566 成功。handler、service、middleware、iteration、projecthealth、cmd/server、cmd/migrate 七个包执行 `go test -race`，全部通过，JSON 记录 6,775 条 test/subtest pass 和 82 条 skip。跳过项包含缺少 Redis 环境的测试、显式性能开关测试及部分测试自身标注的空缺。临时数据库已清理。

合入 `4a9be02e5` 后：Core/Views 相关 6 个静态任务通过；新增管理员 schema 5 个测试、actor picker 22 个测试通过；lifecycle handoff / squad briefing / triage 定向 race 检查记录 114 条 pass、2 条 skip；再次创建的隔离数据库已清理。两项前端问题也由主会话重新复现。

本轮未执行移动端检查、完整产品浏览器 E2E 或依赖漏洞数据库扫描。现有测试通过不能覆盖上列尚缺失的回归场景。没有进行清理或重构，源码改动为零，六项问题均未修复；原有未提交改动保持原状。

[机器可读验证记录](/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-code-audit-jzu0hkn0/verification.json) · [完整前端测试日志](/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-code-audit-jzu0hkn0/frontend-tests.log) · [Go race 日志](/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-code-audit-jzu0hkn0/go-tests.jsonl) · [静态检查日志](/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-code-audit-jzu0hkn0/static.log)
