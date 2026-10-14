# Research: 部署与安全

- 日期：2026-10-14
- 方式：原子代理中断后由主会话接手，只读核查当前实现。
- 完整结论和精确证据见 `../audit-report.md` 以及 `findings.json`。

## 确认问题

- D04（P1）：快速上手和桌面安装仍把邮箱登录当成默认路径
- D12（P2）：Redis 限流说明未区分邮箱认证和密码认证
- D13（P2）：独立工作目录并不覆盖 local_directory 的原地模式
- D14（P2）：Desktop 并没有禁止远程 HTTP 连接

## 覆盖与源码入口

- `self-host-quickstart`：Compose 初始化、默认登录、健康探针、反向代理、模板挂载、升级命令。源码：`docker-compose.selfhost.yml`、`Makefile`、`server/internal/handler/auth_device.go`、`server/cmd/server/router.go`。
- `vcs-integration`：提供方、Webhook 签名、PR/MR 状态和关闭意图、代码凭据范围。源码：`server/internal/handler/vcs.go`、`server/internal/integrations/vcs/forgejo.go`、`server/internal/integrations/vcs/gitlab.go`。
- `desktop-app`：登录方式、独立 profile、正式连接配置、更新 URL、HTTP/HTTPS 连接前提。源码：`apps/desktop/src/shared/runtime-config.ts`、`apps/desktop/src/main/server-switch.ts`、`apps/desktop/src/main/daemon-profile.ts`。
- `security-model`：OS 权限与沙箱说明、task token、工作目录与原地执行的例外。源码：`server/internal/daemon/execenv/execenv.go`、`server/internal/handler/project_resource.go`、`server/internal/handler/daemon.go`。
- `environment-variables`：认证/Redis 分支、连接池、重连宽限、Compose 传递、daemon 配置和模板目录。源码：`server/internal/auth/password_limiter.go`、`server/cmd/server/dbstats.go`、`docker-compose.selfhost.yml`、`server/internal/daemon/config.go`。
- `auth-setup`：三种认证入口、密码配置和限制、设备身份、注册规则、TTL/Compose 生效步骤。源码：`server/internal/auth/password_config.go`、`server/internal/handler/auth_device.go`、`server/internal/handler/auth_password.go`、`server/internal/auth/cookie.go`。
- `platform-administration`：密码会话门禁、首次管理员、凭据文件、保留天数与显式恢复入口。源码：`server/internal/handler/admin_auth.go`、`server/cmd/server/platform_admin_initialization.go`、`scripts/initialize-platform-admin.sh`、`server/cmd/server/password_recovery.go`。
- `troubleshooting`：排队/目录等待、并发、健康/WS、日志命令、数据库回填入口。源码：`server/cmd/server/router.go`、`server/internal/daemon/config.go`、`server/cmd/multica/cmd_daemon.go`、`server/cmd/backfill_task_usage_hourly/main.go`。
- `auth-tokens`：cookie/PAT/task token、30/90 天时限、7 天续期阈值、退出与吊销区别。源码：`server/internal/auth/cookie.go`、`server/internal/handler/personal_access_token.go`、`server/internal/handler/daemon.go`、`server/internal/handler/auth.go`。

## 局限

未执行真实部署、真实账号操作、数据库集成测试或图片视觉对照。没有修改本任务之外的文件。
