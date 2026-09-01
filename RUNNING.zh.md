# 本地启动与停止

如何在开发环境中启动和停止三端 —— **后端**（Go API）、**外部端 / Web**（Next.js）、**桌面端**（Electron）。

English: [RUNNING.md](RUNNING.md) · Docker 自托管：[SELF_HOSTING.md](SELF_HOSTING.md)

---

## 速查

```bash
make up C=api,web,desktop   # 全部启动
make status                 # 当前跑着什么，并证明属于本 checkout
make down                   # 停进程，保留数据库
make destroy                # 停进程，并删除数据库 + profile + 端口槽位
```

`make up` 是所有 checkout（包括 git worktree）的推荐入口。它在锁保护下分配端口、数据库名和 CLI
profile，记录到 `~/.multica/dev/`，并通过 `DATABASE_URL` 验证数据库连通性 —— 而不是用
`docker exec`：当本机原生 PostgreSQL 占用 5432 时，`docker exec` 会连到错误的实例。

---

## 前置条件

- Node 22 + pnpm、Go 1.26、Docker（用于共享的 PostgreSQL 容器）
- 每个 checkout 执行一次 `pnpm install`
- 环境变量文件：主 checkout 用 `.env`，worktree 用 `.env.worktree`
  （`make worktree-env` 会生成带独立端口的那份）

---

## 受管环境（推荐）

### 启动

```bash
make up                       # 默认 api + web
make up C=api,web,daemon      # 加上本地 agent daemon
make up C=desktop             # Electron 连本环境的后端
make up C=api,web,daemon,desktop
make up ARGS=--ephemeral      # agent 持有、带 TTL，由 `make gc` 回收
```

组件名：`api`（Go 后端）、`web`（Next.js）、`daemon`（agent daemon）、`desktop`（Electron）。
**只要选了任意组件，都会隐含启动 `api`。**

`make up` 会先跑迁移再拉起进程。只有当 `/health` 能证明监听进程的 pid、进程组和 commit 都属于本
checkout 时，它才会复用已在运行的 API。

### 查看状态

```bash
make status          # 本环境的端口、pid、commit 证明
make list            # 本机所有已注册的开发环境
```

日志位于 `~/.multica/dev/envs/<name>/logs/`（`api.log`、`web.log`、`desktop.log`、
`migrate.log`）。daemon 的日志在 profile 目录下：`~/.multica/profiles/<profile>/daemon.log`。

### 停止

```bash
make down                                    # 停掉本环境的全部组件
make down ARGS="--components web,desktop"    # 只停其中几个
```

`make down` 保留数据库、CLI profile 和已分配的槽位，因此下一次 `make up` 会回到同一套端口和数据。

### 彻底销毁

```bash
make destroy        # 停进程，并删除数据库、profile、daemon 工作区、
                    # 桌面端 userData 和注册表条目
make gc             # 回收目录已删除或 TTL 已过期的环境
```

`destroy` 不可撤销：本地数据库会被删掉。

---

## 单独启动某一端

已经有环境在跑、只想在前台重启其中一块（看输出或挂调试器）时用这些命令。它们读取当前环境文件，并
跳过数据库预检 —— `make up` 已经验证过数据库可达。

| 端 | 启动 | 地址 |
| --- | --- | --- |
| 后端（Go API） | `make api-dev` | `http://localhost:$BACKEND_PORT`（默认 8080） |
| 外部端（Next.js） | `make web-dev` | `http://localhost:$FRONTEND_PORT`（默认 3000） |
| 桌面端（Electron） | `make up C=desktop` | Electron 窗口 |

以上都在各自终端里 `Ctrl-C` 停止。

### 只跑后端

```bash
make server     # 先确保 PostgreSQL，再启动 Go server
make api-dev    # 直接启动 Go server，并把 commit 打进 /health
```

### 只跑外部端

```bash
make web-dev    # 等价于 pnpm dev:web
pnpm dev:web    # 同上，但不走环境文件的变量注入
```

### 只跑桌面端

优先用 `make up C=desktop`：它会把注册表分配的渲染进程端口、应用名和 userData 目录交给 Electron，
让桌面端和 API、Web 共用同一份环境账本。

```bash
make up C=desktop
```

`pnpm dev:desktop` 也能跑，但它会根据 checkout 路径自行推导身份、自我隔离，不使用已分配的槽位。
只有在你确实想要一个独立的桌面实例时才用它。

用本环境的变量执行任意命令：

```bash
make env-exec ARGS="-- pnpm dev:desktop"
```

### Agent daemon

```bash
make up C=daemon      # 作为环境的一部分启动
make daemon           # 用 CLI 已存的认证信息重启本地 daemon
```

---

## 旧的单 checkout 命令

这些命令早于环境注册表，直接作用于当前环境文件，不注册任何东西，因此 `make status` / `make list`
看不到它们。优先用 `make up`。

```bash
make setup      # 装依赖、确保数据库、跑迁移
make start      # 先迁移，然后在前台跑后端 + 前端
make stop       # 杀掉占用 $PORT 和 $FRONTEND_PORT 的进程
make dev        # 端到端自举后在前台启动
```

`make start` 把两个进程放在同一个 `trap 'kill 0' EXIT` 下，`Ctrl-C` 会一起停。`make stop` 按端口
杀进程 —— 如果该端口恰好被无关进程占用，也会被一起杀掉。

worktree 变体：`make setup-worktree`、`make start-worktree`、`make stop-worktree`（固定使用
`.env.worktree`）；主 checkout 变体：`make setup-main`、`make start-main`、`make stop-main`。

---

## 数据库

共享的 PostgreSQL 容器独立于应用进程，`make down` 不会影响它。

```bash
make db-up       # 启动共享 PostgreSQL 容器
make db-down     # 停止容器（Docker volume 保留）
make db-drop     # 永久删除本 checkout 的数据库（会先确认）
make db-reset    # 删库重建并重跑全部迁移
make migrate-up  # 执行迁移
```

多个 worktree 共用一个容器，通过 `.env.worktree` 拿到彼此隔离的数据库名和端口。连同数据库一起移除
一个 worktree：

```bash
make remove-worktree WORKTREE=../path
```

---

## 端口

端口来自环境文件（`.env` 或 `.env.worktree`）；通过 `make up` 启动时来自注册表分配。

| 变量 | 默认值 | 使用方 |
| --- | --- | --- |
| `BACKEND_PORT` / `PORT` | 8080 | Go API，WebSocket 在 `/ws` |
| `FRONTEND_PORT` | 3000 | Next.js 外部端 |
| `POSTGRES_PORT` | 5432 | 共享 PostgreSQL |
| 桌面端渲染进程 | 动态分配 | Electron renderer dev server |

---

## 排查

**提示 “No environment registered for …”** —— 先执行 `make up`；`status` / `down` / `destroy`
都需要注册表条目。

**端口被占用** —— `make list` 列出本机所有环境及其占用的端口。用 `make down` 停掉占用方；如果它的
目录已经不存在，用 `make gc`。

**后端起来了但前端连不上** —— 检查环境文件里的 `NEXT_PUBLIC_API_URL` 和 `NEXT_PUBLIC_WS_URL`，
它们必须指向 `BACKEND_PORT`。

**启动时迁移失败** —— 查看 `~/.multica/dev/envs/<name>/logs/migrate.log`。

**checkout 已删除但环境残留** —— `make gc`。
