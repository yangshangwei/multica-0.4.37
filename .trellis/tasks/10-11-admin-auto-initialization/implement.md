# 自动初始化专用管理员：实施计划

状态：等待最终规划审阅。完成本文件不等于获准实施；本轮只产出规划资料。

## 1. 准备与执行约束

- [ ] 用户在看到最终规划摘要后确认实施，再执行 `task.py start`。
- [ ] 读取 `prd.md`、`design.md`、两个 research 文件及 `implement.jsonl`；记录新的 git 状态和本任务文件基线。
- [ ] 为并行修改保留独立归属，尤其不能覆盖迭代/分拣改动；实施当天分配尚未占用的迁移号。
- [ ] 创建独立测试数据库/私有 fixture schema 和临时部署目录；不修改已有本机 `platform_admin`、真实部署数据库或服务端运行配置。
- [ ] 不新增依赖；使用项目已有 Go 密码与数据库设施、标准库文件/随机数处理、已加载镜像中的 Node。

## 2. 实施顺序

### A. 初始化存储与事务

- [ ] 在 `server/migrations/` 增加初始化记录表与单独的并发唯一索引迁移；按已有管理迁移的方式拒绝删除已使用的记录。
- [ ] 在 `server/pkg/db/queries/` 增加终态读取/创建及既有管理员历史检查，运行 `make sqlc`，不手改 generated 文件。
- [ ] 新增 `server/internal/service/platform_admin_initialization.go`，复用平台锁顺序、账号查询和审计设施；采用借用事务接口，事务/文件协调及锁外密码 KDF 由部署 CLI 负责。
- [ ] 实现首次创建：新账号、受限密码凭据、角色、永久标记、操作和审计在同一个事务中提交；文件持久化失败不能提交账号。
- [ ] 对人工 Bootstrap/超级管理员授予补写同事务的保留标记，不改变原有授予、最后管理员、版本提升或密码复核规则。
- [ ] 更新相关数据清理/工作空间删除分类清单，初始化标记永久保留。

### B. 私有文件与部署命令

- [ ] 在 `server/cmd/server/` 增加 `platform-admin initialize --credentials-file ... --reason ...` 分支及边界测试，保留现有 `bootstrap`。
- [ ] 实现有版本的凭据文件、加密安全随机初始密码、0600/归属/常规文件校验、不可覆盖的原子发布和文件/父目录同步。
- [ ] KDF 在锁外，事务内重验快照；已有合法待提交文件复用原意图，未知文件不覆盖。
- [ ] 实现按永久标记核对的并发和不确定提交结果；stdout 回执不带密码，日志和错误也不带原始文件内容。
- [ ] 补齐待改密账号文件丢失/损坏时的明确恢复结果，不提供自动覆盖/重置参数。

### C. 首次改密闭环

- [ ] 在现有 `server/internal/handler/auth_password.go` 中仅对强制改密状态增加同密码拒绝，并在事务内复核相关状态。
- [ ] 在共享密码错误映射与中英文 locale 中提供新错误提示；复用已有表单和 API 解析。
- [ ] 验证改密前业务与凭据入口全部受限，改密成功后账号仍有角色、新会话可用、旧密码和旧会话失效、管理台跳转保留。

### D. 离线升级集成

- [ ] 增加 `scripts/initialize-platform-admin.sh` 及必要的纯 JSON Compose 覆盖生成 helper，保持明确的脚本职责，不重构其他发布流程。
- [ ] 使用最终 Compose 配置和有序覆盖文件执行模式判断及一次性容器；无宿主 Node/Go 依赖、无拉取、无额外服务端口。
- [ ] 把 helper 接到 `scripts/offline-upgrade.sh` 的迁移健康成功之后、整次成功输出之前；初始化失败不能输出整次成功。
- [ ] 让 `scripts/offline-bundle.sh` / `scripts/build-offline-upgrade.sh` 交付 helper，并保证独立重试不要求再次升级。
- [ ] `.gitignore` 与 `.dockerignore` 排除初始密码及暂存文件；测试镜像上下文和最终包都不带实例秘密。

### E. 文档与规范

- [ ] 更新 `docs/offline-upgrade.zh-CN.md` 的 7.5 节为自动初始化、读取文件、首次改密、重试及恢复边界，保留人工授权说明。
- [ ] 更新 `docs/desktop-intranet-update-runbook.zh-CN.md` 的交付验收项。
- [ ] 更新 `apps/docs/content/docs/platform-administration.mdx` 及中文页面，运行 `pnpm generate:docs-bundle` 并检查相关生成差异。
- [ ] 更新 `.env.example` 说明以及 `.trellis/spec/server/offline-delivery.md`、认证相关规范，保存永久标记、首次同密码拒绝和秘密文件交付契约。

## 3. 关键回归矩阵

| 场景 | 必须证明 | 主要层级 |
| --- | --- | --- |
| 合规空部署 + 注册关闭 | 生成一个新专用账号、密码凭据、角色、标记和审计；无需公开注册 | service / CLI |
| 设备模式 / 管理关闭 | 无账号、标记或密码文件写入 | CLI / scripts |
| 缺失/错误部署 UUID | 明确配置错误，无部分账号写入 | CLI |
| 已有有效/停用/待改密/禁用名单管理员 | 保留账号与哈希，自动初始化不以有效数量为零重建 | service |
| 历史管理员已撤权/删除 | 成功授予历史或永久标记阻止静默重建 | service / migration |
| 同名普通账号 | 不授权、不改密、不覆盖身份 | service |
| 两次升级、API 重启、两个维护进程 | 只有一次账号/审计创建，初始哈希和文件不变 | service / scripts |
| 人工 bootstrap 尚未配置部署 UUID 或后台开关 | 保持原有密码模式下的显式授权行为，写保留标记不引入新配置前提 | service / CLI |
| 两个维护主机共享数据库 | 仅一个创建者；另一方缺少匹配初始文件时返回 `credentials_unavailable`，不虚报交付成功 | integration |
| 初始登录 | 只能进入改密流程，admin/业务/PAT/CLI/task/daemon 路径不可用 | real router |
| 用相同初始密码提交改密 | 受控错误、UI 提示、版本/标记不变 | handler / views |
| 改成不同密码 | 角色保留、新会话可访问后台；初始密码/旧会话失效 | real router / browser |
| 文件创建/写入/fsync/发布失败 | 账号事务不提交；不覆盖预存文件 | CLI file tests |
| 文件已发布但父目录同步失败，然后重试 | 重试必须重新同步已有文件及父目录；同步成功前仍无账号提交 | CLI / integration |
| 文件是链接、错误权限/归属/部署/意图 | 拒绝并保留原文件 | CLI file tests |
| 文件已持久化、DB 事务回滚 | 文件可核对，重试复用同一密码/意图 | integration |
| 提交成功但响应丢失 | 锁内查询已提交标记，不二次创建或换密 | service / CLI |
| 文件在待改密账号提交后被外部删除 | 明确要求维护恢复；不能自动生成新密码 | CLI |
| 已改密后删除初始文件再升级 | 不重建文件，不回滚哈希或角色 | integration |
| 审计写入失败 | user/credential/role/marker 全回滚 | service |
| 全部有序 Compose 覆盖 + 特殊路径 | 目标镜像/DB/网络一致；参数不被 Shell 或 Compose 插值误解 | scripts |
| 容器调用/初始化失败 | 升级返回非零，保留备份和状态，日志不含秘密 | scripts |
| 归档与 Docker 上下文 | 不包含生成密码；两份操作手册与源文档一致 | packaging |

测试按行为所在的单一层建立矩阵，浏览器只覆盖真实首次改密到后台的主流程和导航，不重复所有纯函数案例。使用 `internal/testutil`、私有 schema 和假外部镜像边界，不运行用户已安装的智能体 CLI。

## 4. 验证命令与证据

实施时按变更范围执行，先窄后宽；以下为计划，尚未宣称运行通过：

```bash
make sqlc
go test ./cmd/server ./internal/service ./internal/handler ./internal/middleware ./internal/migrations -run 'PlatformAdmin|Password|AdminRollback' -count=1
go vet ./cmd/server ./internal/service ./internal/handler
node --test scripts/offline-changelog.test.mjs scripts/offline-build-changelog.test.mjs
bash scripts/selfhost-config.test.sh
pnpm --filter @multica/views test -- auth/password-error.test.ts auth/password-form.test.tsx
pnpm generate:docs-bundle
pnpm lint
pnpm typecheck
pnpm test
make test
```

Go 命令在 `server/` 中执行；其余在仓库根目录执行。新增脚本/CLI/文件测试的精确路径在文件落地后加入执行记录；数据库测试必须连接本任务独立测试数据库。

- [ ] Trellis check 与独立安全审阅覆盖新部署身份权限路径、永久标记和日志秘密边界。
- [ ] 使用一次性旧版本部署、已加载且架构正确的镜像完成真实升级演练：保留普通账号和业务数据，自动初始化 → 首次改密 → 后台 → 再次升级。
- [ ] 浏览器验收使用 production Web 构建，记录前后版本、迁移健康、账号 ID/版本和 API 结果，不保存密码或 cookie 到普通验证报告。
- [ ] 记录实际命令、退出码、未覆盖项；只提交本任务路径，使用 Lore 意图说明与 Tested/Not-tested trailers。

## 5. 失败与回退

优先向前修复。自动初始化只能增加首次账号和永久标记，不能修改已存在的管理员凭据。服务更新与初始化不是同一个事务；初始化失败不会让脚本回滚已完成迁移。

有使用记录时不能破坏性 down。需要停用自动流程时暂停维护脚本调用，不删除永久标记或账号来模拟“未初始化”。初始文件遗失且账号尚未完成改密时，使用现有显式部署恢复流程；恢复最后一位管理员仍需其现有 break-glass 约束。

普通升级和最终发布操作待实现验收后另行执行，本规划阶段不进行。
