# 真实旧版 capability 路由验证

2026-10-05 13:37:38 UTC。仅重启本任务保留的 `0af59c5d5` 基线服务，使用原基线私库和已有认证成员fixture；没有运行性能测试或修改产品源码。

- Binary SHA256：`503316d0ff94ca471389c3888df867b12d36e98c36250cf8141814face86387d`。
- `/health`：commit=`0af59c5d5-performance-baseline`，PID=`60479`，started_at=`2026-10-05T13:37:38Z`；确认是本脚本启动的子进程。
- 端口：独占 `localhost:19072`，启动前无监听；完成后仅停止自己的进程，并确认端口无监听。
- 工作空间：`c07e5806-c089-4761-b03a-9599141353de`；项目：`a48e4bdf-950c-4e66-8c9f-1da51821686e`。每次请求使用同一已认证fixture成员及匹配的 `X-Workspace-ID`。

| 请求 | 实际状态 | 安全响应／校验 |
| --- | ---: | --- |
| `GET /api/workspaces/c07e5806-c089-4761-b03a-9599141353de` | 200 | id匹配该workspace；slug=`p1-performance-baseline-b0017dc5` |
| `GET /api/projects/a48e4bdf-950c-4e66-8c9f-1da51821686e` | 200 | id匹配项目，workspace_id匹配同一workspace |
| `GET /api/workspaces/c07e5806-c089-4761-b03a-9599141353de/project-capabilities` | 404 | `404 page not found` |
| `GET /api/projects/capabilities` | 400 | `{"error":"invalid project id"}` |

因此旧版的专用workspace子路由确实不存在，而旧 `/api/projects/{id}` 会把 capabilities 当非法项目ID；上述404不是无法访问workspace/project产生的假象。所有结果由真实旧router返回，无mock。

运行前依次source `.env.worktree` 与 `.omx/projects-p1-performance/baseline.env`，再执行 `.omx/projects-p1-performance/legacy-capability-probe.mjs`。初次只source baseline.env，缺少原签名配置，控制请求返回401；补齐原环境后重新启动，先确认两条200，再执行404/400断言。没有把401当作unsupported。

脱敏机器证据：[legacy-capability-result.json](legacy-capability-result.json)。只保存状态、身份及安全错误正文，不保存token、认证header或数据库连接信息。
