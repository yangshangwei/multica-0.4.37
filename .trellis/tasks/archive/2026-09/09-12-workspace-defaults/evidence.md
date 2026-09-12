# 工作区内置能力验收记录

状态：代码与验证完成，已提交并在 Trellis 归档。
分支：feat/workspace-defaults；基线：fe38a164b。

## 已交付
- 工作区创建完成后进入项目页，保留 Mika 会话入口。
- 项目创建直接选择资料和执行小队；缺运行环境仍保存选择，准备失败可恢复。
- 服务端幂等准备并复用小队、智能体和 skill，保护权限、用户修改、并发和删除边界。
- 项目详情展示全体成员的实际可用状态；普通新建任务和明确交给小队入口都带入默认分配，显式覆盖优先。
- 小队、智能体、skill 及自动化内置目录直接展示，保留自建与复制编辑。
- 自动化从项目入口预填项目与小队，成员读取失败可重试，只有明确启用才创建定时规则。
- 四种界面语言对齐；项目文档四语更新，内置文档包与相关 skill source map 同步。

## 验证
| 检查 | 结果 |
|---|---|
| 全仓类型检查 | 9 项通过；最后 API UUID 兼容修正后 core typecheck 再次通过 |
| 全仓 lint | 6 项通过，无错误；既有 Hook/disable 警告保留 |
| 完整 TS 测试（限制并发） | 680 文件、8013 测试通过 |
| 最后 API UUID 兼容修正后的 core 全量 | 151 文件、1887 测试通过；比全仓记录新增 2 个用例 |
| Go 完整守卫测试 | 65 包通过，9 包无测试；继承 Git config 仅在测试子进程规范化 |
| Go 新接口回归 | 24 个顶层测试通过，包含并发、权限、回滚、旧数据、完整响应 |
| Go race / vet | 相关接口 race 和所有修改包 vet 通过 |
| Go 内置文档 | go test ./internal/docs 通过 |
| Playwright | workspace-defaults 两个场景及原自动化模板场景共 3 项通过 |
| 规格复核 | PASS；共享调用、全成员状态、畸形目录、删除恢复、默认值覆盖均已核对 |
| 代码审查 | PASS；成员失败恢复 P2 已补回归并修复 |
| 视觉验证 | 94/100，桌面与 430px 窄屏通过；截图在 evidence/visual |
| 依赖与 diff | 无新增依赖；git diff --check 通过 |

最终 TS 覆盖合计 8015 个测试：core 1887、views 5220、web 260、desktop 588、docs 60。

## 验证环境
独立数据库 multica_multica_workspace_defaults_916；API 18996 / Web 13916。仅启动 API/Web；邮件仅写控制台。浏览器中的 runtime 是数据库测试记录，没有连接真实进程或模型；验证了实际配置、任务分配和入队、自动化创建与调度启用。

机器上系统 pnpm 是 11，项目锁定 10.28.2。临时 /tmp/multica-workspace-defaults-bin/pnpm 转发到 corepack，确保 Turbo 子进程也使用锁定版本。完整测试用 pnpm test --concurrency=1 -- --maxWorkers=4，避免并发重负载导致 5 秒超时。此前超时的 53 个测试独立复跑全绿。

Next 开发服务器曾达到内存阈值并重启；仅本验证环境重启并设置 NODE_OPTIONS=--max-old-space-size=6144 后，最终 3 个浏览器场景全部通过。页面等待基于真实区域/状态，不以 URL 改变代替页面就绪。

## 主要改动位置
- server/internal/handler/project_execution_squad.go、project.go、squad_template.go：准备事务与 API。
- server/migrations/456_project_execution_squad.*.sql、pkg/db/queries、generated：存储与 sqlc。
- packages/core/projects、types/project.ts、api/client.ts、api/schemas.ts：状态、请求、边界解析与缓存。
- packages/views/modals/create-project.tsx、projects/components：配置主流程、状态和自动化入口。
- packages/views/issues/surface：项目默认值的最低优先级合并。
- packages/views/squads、agents、skills、autopilots：内置内容与创建入口。
- apps/web 和 apps/desktop：上手完成落点及模板参数接线。
- e2e/workspace-defaults.spec.ts、autopilot-template.spec.ts、fixtures.ts：端到端契约和隔离数据。

## 简化与限制
复用既有角色/skill materializer、权限规则、运行环境选择、任务创建、自动化配置和导航适配器；没有新依赖或后台作业框架。真实模型执行、账号授权及原生文件选择器的人工操作未作为本次自动测试运行；没有部署或推送远端。

实现提交：af2977950（后端与 core）、ef494ddd3（界面、集成测试与文档）。

收尾：6 个子任务与父任务已归档，Trellis 会话已记录；本次验证服务已停止，工作目录与数据保留。
