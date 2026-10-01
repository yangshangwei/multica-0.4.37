# S01 实施验收

状态：实现与切片验收通过；父任务和 S02–S07 继续推进。日期：2026-10-01。

## 基线、保全与环境

- 原始源码基线 `6dd927657`；实施分支 `feat/platform-admin-console`，工作区 `/Volumes/artisan/code/2026/multica-platform-admin`。
- 原工作区无 tracked/staged 改动。8 个平台任务目录共 63 个原文件完整复制并逐文件 SHA-256 校验；原目录和无关 integration-catalog 任务未修改。
- 规划及授权进度已单独提交 `a6316d9fa`。8 个任务的 Trellis validate 均通过。用户授权从 S01 实施，不重做需求访谈。
- Go 测试数据库 `multica_platform_admin_s01_test`；浏览器数据库 `multica_multica_platform_admin_313`，均为本任务新建。
- 生产 Web build `YDSXFawMsM_71ecWB9lRf`，源码指纹 `0315c52c7cc5608c1a63ec069c7065f1689c8833ff1592c7934290bcd23a8091`；API PID 71622，Web PID 71999。实际 password 模式、独立数据库、listener/父进程、commit、源码指纹均核验。
- `make status` 的 API 汇总因配置指纹口径显示 mismatch；实际 health PID/commit、listener ownership、相同源码指纹和直接 api_identity_matches 均通过，Web 汇总 running。未因此修改无关 dev-env 脚本。完整 provenance 见 evidence/。

## 已交付与权威证据

| S01 要求 | 实现与验证 |
| --- | --- |
| JWT/cookie、PAT/机器凭据边界 | `/api/admin` 位于 Auth 内、workspace guard 外。完整密码 JWT/cookie 才允许平台授权。真实路由 19 类凭据、CSRF、伪造头、未支持模式、503 和所有响应 no-store 测试通过。 |
| 凭据转换与兼容 | 先复现 PAT→JWT 200，再修复为403；浏览器 JWT→PAT、JWT/cookie交接、PAT续期回归通过。公共 LockPasswordSession 保持 PAT 兼容。 |
| 授予权限撤销旧会话 | 角色授予/提升、bootstrap 在事务内提升版本并撤销派生凭据，提交后显式断连目标；A 的会话保留。历史JWT不能跟随新权限升级。 |
| 并发撤权与最后管理员 | 单次快照读取角色/版本/状态；平台 advisory lock 后按 UUID 排序用户行锁；锁内再次核验。互降、并发 bootstrap、撤权排队及临时封禁名单 UUID/email 回归通过。 |
| 幂等与审计 | 当前 actor/组织/key 找回原 operation；不同非秘密请求冲突；未知提交结果用原key恢复。业务、operation、审计同事务；密码/KDF不进入操作、审计或摘要。失败注入验证全部回滚。 |
| 组织归属 | 迁移465–474，无新外键，每个索引独立CONCURRENTLY；501个空间按500/1/0分批回填；新空间与映射同事务。inactive或写失败回滚，删除空间移除映射但保留平台角色/审计。 |
| 无workspace后台 | `/admin`独立shell；共享登录保留完整next；无workspace可进入；角色撤销清后台缓存但不登出普通业务。账户/凭据/服务器/组织作用域隔离和迟到请求回归通过。 |
| 管理员初始化/恢复 | pre-listener bootstrap需现有具名密码账号；password-recover要求reason；last-admin保护与显式break-glass均留审计，临时密码仍必须改密。 |

## 实际运行

- 前端有界全套：`pnpm --workspace-concurrency=1 --filter @multica/core --filter @multica/views --filter @multica/web test --maxWorkers=4`：core 185文件/2315测试、views 472/5830、web 36/281，全部通过。之后新增config解析矩阵所在schemas.test 154项通过；hash登录修复相关16项通过。
- `pnpm typecheck`：9/9成功；`pnpm lint`：6/6成功，保留仓库已有warnings。最终frontend修复的定向lint/typecheck通过。
- `pnpm knip`非零：9个闲置文件、1个依赖、1个开发依赖。原始main工作区同命令输出完全相同发现项，无本任务新增项；不夹带清理。
- 通过agent-CLI guard执行受影响Go包完整race：auth、middleware、service、migrations、cmd/server、cmd/migrate通过。handler初次仅schema deletion manifest失败；补齐所有新表分类后完整handler race复跑：3955 pass、48 skip、0 fail，131.115s。跳过主要为未配置Redis及外部集成；没有本任务Admin/Password权限用例被跳过。
- 修订后平台service/router定向race通过（58.530s/13.495s），Go vet通过；组织/删除回归race通过。每个全局管理员测试使用独立schema，支持跨包并行。
- `pnpm exec playwright test e2e/platform-admin.spec.ts --workers=1 --retries=0`：1通过，18.3s，零pageerror。实际HttpOnly cookie、无localStorage token、无workspace、普通用户拒绝、真实角色授予撤旧cookie、重新登录、真实撤权经15秒轮询移除敏感页面而/api/me仍200。
- 最终桌面1440×900、窄屏390×844、拒绝/撤权截图与ARIA快照：visual-verdict pass94。截图位于`.omx/reports/platform-admin/`；结构化证据已复制至evidence/。

## 独立审阅与已修复失败

独立源码复核关闭四项：临时封禁用户错误计入有效管理员；inactive组织零行写仍创建空间；组织故障误404；密码复核错误误清后台。浏览器发现Next片段缓存重复hash，管理登录改为平台边界校验后的文档导航，回归和真实浏览器均通过。

## 范围限制

这是S01基础切片完成，不是全平台后台或发布完成。浏览器本轮为Chromium/英文/浅色。Redis专项、Windows/macOS原生安装、1000安装/100万执行容量、终端协议和发布演练属于S02–S07，尚未完成。仅任务自有测试账号bootstrap；未发布、推送、发送外部通知或控制真实终端。
