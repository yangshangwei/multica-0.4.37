# Multica 源码学习计划

面向想真正改动这个仓库的人，而不是想"通读一遍"的人。

本计划中出现的每个路径、行数、命令都来自当前 checkout（`multica-0.4.37`）的实际扫描结果，不是模板。

---

## 0. 先认清规模，再决定策略

| 范围 | 源码行数 | 测试行数 |
| --- | ---: | ---: |
| `server/`（Go） | 303,346 | 351,433 |
| `packages/views/` | 159,817 | 105,630 |
| `packages/core/` | 42,831 | 29,859 |
| `apps/mobile/` | 31,425 | — |
| `apps/web/` | 27,397 | — |
| `apps/desktop/` | 22,284 | — |
| `packages/ui/` | 10,503 | — |

其他关键量级：

- `server/internal/handler/` 118 个非测试文件、71,109 行 —— 单这一个包就比多数中型项目大。
- 数据库迁移编号已到 **443**，`server/pkg/db/queries/` 有 56 个手写 SQL 文件。
- `server/cmd/server/router.go` 2,510 行，是整个后端的路由总目录。
- 最大的几个 handler：`daemon.go` 5,236 行、`issue.go` 4,462 行、`comment.go` 3,827 行。

**结论：通读是不可行的。** 70 万行代码按每天读 2000 行算要一年。

这份计划采用的策略是**纵切（vertical slice）**：不按目录横扫，而是挑 2 条贯穿全栈的真实业务路径走通，用它们把分层、约定、状态模型一次性串起来。剩下的模块在需要时按同一套坐标系自行展开。

测试量大于源码量（Go 测试比源码还多 16%）是个重要信号：**这个仓库的测试是可读的规格说明**。看不懂某个模块时，先读它旁边的 `_test.go` / `.test.ts`，通常比读实现快。

---

## 1. 前置：必须先读的三份"地图"（约 1.5 小时）

这三份文档是仓库的硬约束来源。跳过它们去读代码，会把很多"刻意的设计"误读成"可以随手改的写法"。

| 文档 | 读什么 | 为什么必读 |
| --- | --- | --- |
| `CLAUDE.md`（约 20K） | State Rules、Package Boundaries、API Compatibility、Backend UUID Rules、Database and Migration Rules | 这是仓库的宪法。里面每条规则背后都有一次事故 |
| `apps/docs/content/docs/developers/architecture.zh.mdx`（129 行） | 全文 | 官方分层图 + 一次 agent 执行的 7 步代码路径 |
| `apps/docs/content/docs/developers/conventions.zh.mdx`（324 行） | 全文 | 命名、i18n 术语表、中文产品口吻的唯一权威 |

读完后，你应该能不看文档回答这 5 个问题（答不上就回去重读）：

1. `packages/core/` 为什么不能出现 `localStorage` 和 `process.env`？
2. 什么情况下允许做乐观更新，什么情况下必须等服务端返回？
3. 为什么禁止数据库外键？
4. 为什么网络返回的 JSON 不能直接 `as T`？
5. `packages/views/` 里想跳转页面该用什么？

补充速查（不必精读，需要时再翻）：

- `RUNNING.zh.md` —— 本地启停（你自己写的，最贴合当前机器）
- `CONTRIBUTING.md` —— worktree、环境文件、测试位置
- `AGENTS.md` —— 指向 CLAUDE.md 的精简索引

---

## 阶段 0：把它跑起来（0.5 天）

**没跑起来之前不要读代码。** 读一个你无法触发、无法打断点的系统，效率会低一个数量级。

```bash
make up          # 启动本 checkout 的环境（api/web/daemon/desktop）
make status      # 确认 pid 和 commit 确实属于当前 checkout
make list        # 看这台机器上所有开发环境
```

动手做完这些：

1. 注册账号 → 建 workspace → 建一个 issue → 改状态 → 评论。
2. 打开浏览器 DevTools 的 Network，把上面每一步对应的 HTTP 请求记下来（路径 + 方法）。
3. 切到 WS 标签，看改状态时推了什么事件。
4. `make down`（保数据）和 `make destroy`（连库一起删）各试一次，理解区别。

**验收**：你能画出"点一次状态按钮"触发的请求清单，且知道其中哪些是 WebSocket 推的。

> 踩坑提醒：`make up` 用 `~/.multica/dev/` 注册表分配端口和库名。如果你本机有原生 PostgreSQL 占着 5432，不要用 `docker exec` 去建库 —— 会建到错误的 server 里。

---

## 阶段 1：纵切一 —— "改一个 issue 状态"走完全栈（2–3 天）

这是整个仓库的**主干**。它同时覆盖：路由、鉴权、workspace 边界、handler/service 分层、sqlc、zod 边界解析、TanStack Query 缓存、乐观更新、WebSocket 回灌。走通这一条，后面 80% 的代码你都能自己读懂。

### 1.1 从前端出发（自顶向下）

按顺序读，每个文件先读它的 `.test.ts`：

```
packages/views/issues/components/     状态选择 UI 在哪触发 mutation
packages/core/issues/mutations.ts     乐观更新怎么写的、怎么回滚
packages/core/issues/queries.ts       query key 结构（注意 wsId 一定在 key 里）
packages/core/issues/cache-helpers.ts 确定性缓存补丁 vs 不确定投影失效
packages/core/api/client.ts           请求怎么带 X-Workspace-ID
packages/core/api/schema.ts:38        parseWithFallback —— 边界解析的唯一入口
packages/core/api/schemas.ts          zod schema 长什么样
```

重点搞明白：`cache-helpers.ts` 为什么区分"能确定地打补丁的缓存"和"必须整体失效的投影"。这是 CLAUDE.md 那条乐观更新规则的具体实现。

### 1.2 到后端（自外向内）

```
server/cmd/server/router.go              找到 /api/issues 挂载点，看它套了哪几层中间件
server/internal/middleware/auth.go       284 行，认证与 workspace 归属
server/internal/handler/issue_status.go  状态变更的 handler
server/internal/handler/handler.go       1,232 行，Handler 结构体与共享依赖
server/internal/service/issue.go         跨查询的业务流程与事务
server/pkg/db/queries/issue.sql          手写 SQL
server/pkg/db/generated/                 sqlc 产物，只读不改
```

读 handler 时刻意验证 CLAUDE.md 的 **Backend UUID Rules**：找出 `loadIssueForUser`、`parseUUIDOrBadRequest`、`parseUUID` 三者各自的调用点，说清为什么不能互换。

### 1.3 实时回灌

```
server/internal/realtime/hub.go            连接与订阅
server/internal/realtime/broadcaster.go    事件怎么发出去
packages/core/realtime/use-realtime-sync.ts 客户端订阅入口
packages/core/issues/ws-updaters.ts         WS 事件如何改写 Query 缓存
```

关键认知：WS 事件只能**失效或补丁 Query 缓存**，绝不能把服务端 payload 镜像进 Zustand。读 `ws-updaters.ts` 时对照这条规则看它是怎么守住的。

### 动手（必做）

给 issue 状态变更加一条**你自己的日志/断言**，从前端一路加到 SQL，跑一次操作，观察日志顺序。做完再全部删掉。

**验收**：闭卷画出这张链路图，标出每一跳的文件名：

```
UI → mutation(乐观) → api client → zod 边界 → router → middleware
  → handler → service → sqlc → PG → realtime hub → WS → ws-updaters → 缓存
```

---

## 阶段 2：前端状态模型（2–3 天）

阶段 1 你已经见过一次完整链路，现在把状态规则学扎实 —— 这是这个仓库最容易写错、review 最常打回的地方。

读：

```
packages/core/provider.tsx            Provider 组合
packages/core/query-client.ts         全局 Query 配置
packages/core/platform/               StorageAdapter、持久化命名空间、auth 初始化
packages/core/workspace/queries.ts    workspace 身份怎么由路由驱动
packages/views/layout/dashboard-guard.tsx  共享守卫
packages/core/issues/store.ts + stores/     Zustand 只放客户端状态的实例
```

刻意对照检查的四条（每条找出仓库里的正例）：

1. Query 管服务端状态，Zustand 管客户端状态 —— 找一个"看起来像服务端数据但其实是客户端状态"的例子。
2. 所有 workspace 作用域的 query key 必须含 `wsId`。
3. Zustand selector 必须返回稳定引用（不能现场 new 对象/数组）。
4. 会导航的流程（create / delete / leave）必须等服务端确认后再清理。

> 已知技术债，不要照抄：workspace **leave** 目前是先清理再发 mutation，只为规避 `member:removed` 实时竞态。CLAUDE.md 明确说这是债不是范式。

**验收**：给定一个新需求（例如"issue 列表增加一个筛选条件 + 一个新的服务端字段"），你能立刻说出哪部分进 Query、哪部分进 Zustand、放在哪个包、query key 怎么写。

---

## 阶段 3：后端骨架（2–3 天）

现在横向补齐后端，不再逐行读，而是建立"我要找什么该去哪"的索引。

```
server/cmd/server/main.go        838 行，启动了什么（HTTP / WS / 调度器 / 集成 worker）
server/cmd/server/router.go      2,510 行，当路由总目录用，按需检索
server/internal/middleware/      逐个扫一遍文件名，知道每个中间件管什么
server/internal/auth/            jwt / cookie / pat_cache / membership_cache
server/internal/events/          事件总线
server/internal/scheduler/       定时任务
server/internal/storage/         本地 / S3 附件
```

数据库部分**不要读 443 个迁移**。只做三件事：

1. 读 `001_init.up.sql` 建立核心表的心智模型。
2. 读最近 5–10 个迁移，看当前的写法惯例。
3. 精读 CLAUDE.md 的迁移三条硬规则，并在迁移目录里各找一个实例验证：
   - 无外键、无级联
   - 每个索引 `CREATE INDEX CONCURRENTLY` 且**独占一个迁移文件**
   - 条件跳过的迁移仍会写进 `schema_migrations`，所以后续迁移必须幂等（`IF EXISTS` / `IF NOT EXISTS`）

顺手读一下 `server/internal/testutil/`（541 行）—— `dbfx.Issue`、`testutil.Call(h, req).Want(status).JSON(&out)`。之后你写的每个 Go 测试都要用它，不用它写会在 review 被打回。

**验收**：给一个陌生的 API 路径（如 `/api/autopilots`），你能在 2 分钟内定位到 router 行、中间件链、handler 文件、service 文件和 SQL 文件。

---

## 阶段 4：纵切二 —— agent 执行链路（3–4 天）

这是 Multica 区别于普通任务管理系统的核心，也是最复杂的部分。放在阶段 4 是因为它依赖前面所有基础。

架构文档里那 7 步，对应到代码：

| 步骤 | 代码位置 |
| --- | --- |
| 1. 分配 issue / 提及 agent / 自动化触发 | `server/internal/handler/issue_trigger.go`、`internal/service/issue_trigger.go` |
| 2. 创建 queued task 并通知运行时 | `server/internal/service/task.go`、`internal/handler/task_lifecycle.go` |
| 3. 守护进程领取 task | `server/internal/handler/daemon.go`（5,236 行，**按需检索，不要通读**） |
| 4. 签发绑定 task 的临时凭据 | `server/internal/middleware/daemon_auth.go`、`internal/auth/daemon_token_cache.go` |
| 5. 准备目录、调用 provider | `server/internal/daemon/`、`server/pkg/agent/` |
| 6. 上传进度与最终状态 | `server/internal/daemonws/`（hub / notifier） |
| 7. 更新 task 与 issue，推实时事件 | 回到阶段 1 的 realtime 链路 |

`server/pkg/agent/` 是 provider 适配层，claude / codex / cursor / copilot / grok / kimi 等各一个文件。**读 2 个就够**（建议 `claude.go` + `codex.go`），先读 `agent.go` 和 `launch.go` 理解统一抽象，其余按需。

配套文档：`CLI_AND_DAEMON.md`（58K，当手册查，不要通读）。

**关键认知**：WebSocket 只降低延迟，数据库才是最终状态。守护进程同时保留**轮询兜底**，就是为了防止一次断线让 queued task 永久卡住。理解这个"双通道"设计是读懂这块的钥匙。

**验收**：本地触发一次真实 agent 执行，用日志追出 task 从 `queued` 到终态的完整状态迁移，并说清哪一步失败会导致哪种用户可见现象。

---

## 阶段 5：跨端结构（1–2 天）

现在才看多端，因为共享层你已经熟了。

```
apps/web/app/                              Next.js App Router 分组：(auth) / (landing) / [workspaceSlug]
apps/web/platform/                         Next.js API 的唯一容身处
apps/desktop/src/main/                     Electron 主进程：daemon-manager、updater、window-state
apps/desktop/src/renderer/src/routes.tsx   session 路由
apps/desktop/src/renderer/src/stores/window-overlay-store.ts   过渡流程（不是路由！）
apps/desktop/src/renderer/src/platform/    react-router 唯一容身处
```

桌面端最容易搞错的一条：**pre-workspace 的一次性流程（建 workspace、接受邀请）是 `WindowOverlay` 状态，不是路由**，不要往 `routes.tsx` 里加。

移动端**先跳过**。它是独立的（自有 UI、状态、i18n、React 版本、发版节奏），只共享 `@multica/core` 的类型和纯函数。真要动的时候，先读 `apps/mobile/CLAUDE.md`，它有强制的 pre-flight 流程。

**验收**：说清同一个 issue 详情页在 web 和 desktop 上分别由哪些文件拼起来，共享的部分在哪、平台差异在哪。

---

## 阶段 6：外围系统（按需，不排期）

到这里你已经能独立干活了。以下模块**用到再读**，每个都用阶段 1 的纵切法自己走一遍：

- `server/internal/integrations/`（40,048 行）—— GitHub / Slack / 飞书 / 钉钉 / 企微
- `server/internal/service/plugin*.go` + `packages/plugin-sdk/` —— 插件系统
- `server/internal/service/builtin_skills/` —— 内置技能（**改 CLI 命令/API 字段时必须同步更新对应的 `SKILL.md` 和 `references/*-source-map.md`**）
- `packages/views/editor/`（30,194 行）—— 富文本编辑器
- `server/internal/entitlement/`、`seatcapacity/`、`analytics/` —— 计费与席位
- `server/internal/cloudruntime/`、`runtimeapps/` —— 云端运行时

---

## 阶段 7：用一次真实改动收尾（1–2 天）

学习的终点不是"读完了"，是**改动被验证通过**。

选一个小而完整的需求（例如给 issue 加一个新属性字段），端到端做完：

1. 迁移（遵守无外键 + 独立 CONCURRENTLY 索引文件）
2. `server/pkg/db/queries/*.sql` → `make sqlc`
3. handler + service（注意 UUID 来源规则）
4. zod schema + **一个畸形响应的测试**（这是 API 兼容性规则的硬要求）
5. core 的 query / mutation
6. views 的 UI
7. 测试放在正确的层（见下表）

| 测试对象 | 位置 |
| --- | --- |
| 共享业务逻辑、store、query、hook | `packages/core/*.test.ts` |
| 共享 UI 组件、页面、表单、弹窗 | `packages/views/*.test.tsx` |
| 平台接线（cookie、重定向、search params） | `apps/web/*.test.tsx` |
| 端到端 | `e2e/*.spec.ts` |
| 后端 | `server/` Go 测试 |

验证命令（由窄到宽）：

```bash
pnpm -C packages/core exec vitest run issues/mutations.test.tsx   # 单文件
pnpm -C packages/core test                                        # 单包
(cd server && go test ./internal/handler -run TestUpdateIssueStatus -count=1)
pnpm typecheck
pnpm test
make test
pnpm exec playwright test
make check                                                       # 全量
```

**验收**：改动通过 `make check`，且你能说出它触碰了 CLAUDE.md 里哪几条规则、分别是怎么满足的。

---

## 需要刻意跳过的东西

明确不读，避免时间黑洞：

| 跳过 | 原因 |
| --- | --- |
| `server/pkg/db/generated/` | sqlc 产物，读它等于读机器输出 |
| 443 个迁移的全文 | 只读 `001_init` + 最近几个 |
| `handler/daemon.go` 5,236 行通读 | 当作检索目标，跟着调用链进去 |
| `pkg/agent/` 全部 provider | 读 2 个即可，其余是同一抽象的重复实现 |
| `pnpm-lock.yaml`、`node_modules/` | 无信息量 |
| `apps/mobile/`（初期） | 独立体系，晚点再说 |
| `CLI_AND_DAEMON.md` 通读（58K） | 当手册查 |
| 各语言 `.ja/.ko` 文档 | 中文 + 英文足够 |

---

## 时间预算

| 阶段 | 内容 | 天数 |
| --- | --- | ---: |
| 前置 | 三份地图文档 | 0.2 |
| 0 | 跑起来 + 观察请求 | 0.5 |
| 1 | 纵切一：issue 状态全栈 | 2–3 |
| 2 | 前端状态模型 | 2–3 |
| 3 | 后端骨架 + 数据库规则 | 2–3 |
| 4 | 纵切二：agent 执行链路 | 3–4 |
| 5 | 跨端结构 | 1–2 |
| 7 | 真实改动收尾 | 1–2 |
| | **合计** | **12–18 天** |

按每天 4–6 小时有效投入估算。阶段 6 不计入。

如果只有 3 天：做前置 + 阶段 0 + 阶段 1，然后直接进阶段 7 挑一个最小改动。这个组合的性价比最高。

---

## 记录方法

边读边产出，否则一周后会忘掉 70%。

- **链路图**：每条纵切画一张，标文件名和行号。这是你后续最常回看的东西。
- **问题清单**：读到看不懂的先记下继续走，不要卡住。绝大多数会在后面阶段自动解答。
- **规则实例表**：CLAUDE.md 每条规则配一个仓库里的真实正例（和你找到的反例/技术债）。这比背规则有效得多。
- 仓库自带 `.trellis/spec/` 规格体系和 `.trellis/workspace/artisan/journal-1.md` 日志，笔记可以直接落在这里；`.trellis/spec/guides/` 下的 `code-reuse-thinking-guide.md` 和 `cross-layer-thinking-guide.md` 值得在阶段 2 之后读一遍。

---

## 进度检查表

- [ ] 前置：闭卷答对 5 个规则问题
- [ ] 阶段 0：环境跑通，画出一次状态变更的请求清单
- [ ] 阶段 1：闭卷画出 issue 状态变更全栈链路图
- [ ] 阶段 2：能判断任意新字段该进 Query 还是 Zustand
- [ ] 阶段 3：2 分钟内定位任意 API 的完整实现路径
- [ ] 阶段 4：本地触发 agent 执行并追完 task 状态迁移
- [ ] 阶段 5：说清同一页面在 web / desktop 的组成差异
- [ ] 阶段 7：一个真实改动通过 `make check`
