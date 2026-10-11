# 自动初始化专用管理员：实施计划

状态：实现完成，已通过任务范围内的质量检查；真实旧部署升级演练仍需在具备目标镜像的环境执行。

## 1. 准备与执行约束

- [x] 用户已确认实施；任务已激活并执行。
- [x] 已读取规划、研究和上下文清单，并保留实现前基线。
- [x] 使用独立 worktree、任务专属数据库和私有 fixture schema；未修改真实部署或当前登录账号。
- [x] 未新增依赖；复用现有 Go 密码/数据库设施、标准库文件处理和已加载镜像中的 Node。

## 2. 实施顺序

### A. 初始化存储与事务

- [x] 增加初始化记录表及独立的并发唯一索引迁移，并拒绝删除已使用记录。
- [x] 增加 sqlc 查询和生成代码，已运行 `make sqlc`。
- [x] 新增初始化服务，复用平台锁、账号查询和审计设施；KDF 与文件协调留在部署 CLI。
- [x] 首次创建的账号、受限凭据、角色、永久标记、操作和审计在一个事务中提交，文件持久化失败不会提交账号。
- [x] 人工 Bootstrap/角色授予在同一事务补写保留标记，既有授权边界保持不变。
- [x] 更新工作空间删除清单，初始化标记永久保留。

### B. 私有文件与部署命令

- [x] 增加 `platform-admin initialize --credentials-file ... --reason ...`，保留现有 `bootstrap`。
- [x] 实现版本化凭据文件、随机初始密码、0600/归属/常规文件校验、不可覆盖发布及文件/父目录同步。
- [x] KDF 在锁外，事务内重验快照；合法意图文件可复用，未知文件不会覆盖。
- [x] 实现永久标记并发核对和不确定提交恢复；回执、日志和错误不带密码。
- [x] 待改密账号缺少或损坏凭据文件时返回 `credentials_unavailable`，不自动重置。

### C. 首次改密闭环

- [x] 强制改密仅在锁内复核状态后拒绝同密码，普通改密行为保持原契约。
- [x] 共享错误映射和中英文 locale 已加入 `password_unchanged`。
- [x] 回归覆盖临时会话限制、改密后角色/新会话、旧密码和旧会话失效。

### D. 离线升级集成

- [x] 增加初始化脚本和纯 JSON Compose helper，保持脚本职责边界。
- [x] 使用最终 Compose 配置和有序覆盖文件运行一次性容器，无宿主 Node/Go 依赖、拉取或额外端口。
- [x] 在升级健康检查后、成功输出前调用；初始化失败不会报告整次成功。
- [x] 离线 bundle/upgrade/installer 交付 helper，独立重试无需重新升级。
- [x] Git/Docker/打包路径排除初始密码及暂存文件，并拒绝复用含私密文件的输出目录。

### E. 文档与规范

- [x] 更新升级指南 7.5、重试/恢复边界及人工授权说明。
- [x] 更新桌面内网交付验收项。
- [x] 更新平台管理双语页面，运行 `pnpm generate:docs-bundle` 并同步内置文档。
- [x] 更新 `.env.example`、离线交付和认证规范，记录永久标记、同密码拒绝和秘密文件交付契约。

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

实施时按变更范围执行，先窄后宽；以下命令已按任务专属数据库和 worktree 执行，结果记录见本节及最终提交说明：

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

- [x] Trellis check、Go vet、迁移/服务/CLI/handler 回归和脚本边界测试覆盖身份权限、永久标记和秘密边界。
- [ ] 使用一次性旧版本部署、已加载且架构正确的镜像完成真实升级演练：保留普通账号和业务数据，自动初始化 → 首次改密 → 后台 → 再次升级。
- [ ] 浏览器验收使用 production Web 构建，记录前后版本、迁移健康、账号 ID/版本和 API 结果，不保存密码或 cookie 到普通验证报告。
- [x] 已记录任务范围内命令与结果；全量 `make test` 受既有 daemon/realtime/session 环境失败影响，未将其归因于本任务。
- [ ] 使用一次性旧版本部署和真实生产 Web 完成升级→首次改密→后台演练（需要目标镜像和独立验收环境）。
- [ ] 提交时只包含本任务路径，使用 Lore 意图说明与 Tested/Not-tested trailers。

## 5. 失败与回退

优先向前修复。自动初始化只能增加首次账号和永久标记，不能修改已存在的管理员凭据。服务更新与初始化不是同一个事务；初始化失败不会让脚本回滚已完成迁移。

有使用记录时不能破坏性 down。需要停用自动流程时暂停维护脚本调用，不删除永久标记或账号来模拟“未初始化”。初始文件遗失且账号尚未完成改密时，使用现有显式部署恢复流程；恢复最后一位管理员仍需其现有 break-glass 约束。

普通升级和最终发布操作待实现验收后另行执行，本规划阶段不进行。
