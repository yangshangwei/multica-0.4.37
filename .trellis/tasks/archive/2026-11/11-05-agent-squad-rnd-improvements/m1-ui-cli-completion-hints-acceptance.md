# M1 UI/CLI 自动完成提示验收矩阵

本矩阵验收的是提示消费者，不是完成事务门禁本身。所有测试都必须保留原始失败记录、命令、源码身份和数据库/HTTP fixture；不使用历史 v1 计数代替本轮证据。

## API client / core

| ID | 场景 | 预期 |
|---|---|---|
| C1 | 严格三字段请求 | 发送 `expected_issue_revision`、精确 `target_status`、`evaluation_key:null`；不多字段、不偷传 key |
| C2 | 合法 `not_applicable` / `attention` / `matching` | schema 解析完整，`enforced=false`、`usable_for_completion=false` 保持 false |
| C3 | custom done | 保留服务器 normalized key/category，不把 custom key 改成 `done` |
| C4 | malformed 2xx | 抛 schema/API error，不返回空 hint 或当作成功 |
| C5 | 无结构化 code 的 404/405 旧服务器 | 调用方可明确分类为 unsupported；不自动重试，不吞掉实际 mutation 结果 |
| C5a | 带结构化资源/权限/交付 code 的 404/403/409 | 不能当旧路由 bypass；按拒绝/刷新处理 |
| C6 | abort/transport/5xx | 分类为 unknown；调用方不显示完成成功 |
| C7 | 缺 revision/category | 不伪造 revision/category，不产生错误的授权语义 |

## Shared Web/Desktop

| ID | 入口/场景 | 预期 |
|---|---|---|
| W1 | detail sidebar status picker，非 done → built-in done | 先 hint；attention 打开确认；取消无 PUT，确认后只有一次 PUT |
| W2 | detail mark-done button | `onDone` 只在 PUT 2xx 后执行；hint 取消/拒绝/unknown/PUT 失败不导航 |
| W3 | issue actions dropdown/context menu | 与 W1 共享 gate；不复制不同文案或直接调用 mutation |
| W4 | list/board/swimlane/table 单 issue status editor | 通过 `IssueSurfaceActions.updateIssue` 使用同一 gate；optimistic patch 在确认前不发生 |
| W5 | detail `SubIssueRow` status picker | 不再绕过 gate；普通 child 与父 issue 使用相同 category/attention 语义 |
| W6 | active custom done | catalog category=done 时触发；发送 custom status key；不误判为普通 todo |
| W7 | source done → done/custom done 或其他非新完成 | 不触发 hint；保留原 status writer 行为 |
| W8 | off/no project/non-done target | 无提示对话框；最多一次原 PUT；不读取/泄漏 evaluation key |
| W9 | hint stale revision/明确拒绝 | 不 PUT；展示刷新/拒绝；缓存不作 optimistic success |
| W10 | hint network/5xx/malformed | 不自动 PUT；Retry/Cancel 可用，Retry 不重复 mutation |
| W11 | old server 404/405 | bypass hint，旧 mutation 仍可工作；不显示“已检查”或“已授权” |
| W12 | duplicate click/unmount | 单飞；取消/卸载释放 pending；不产生双 hint/双 PUT |
| W13 | actual PUT 428/409/5xx/unknown | 只显示实际写入错误/未知；绝不因 hint attention/matching 显示成功 |
| W14 | private evaluation response fixture | UI 不展示、不缓存、不打印 evaluation key、observation、credential metadata |

## CLI

| ID | 命令/场景 | 预期 |
|---|---|---|
| L1 | `issue status MUL-1 done` + attention | stderr 警示，随后一次 PUT；仅 PUT 2xx 打印成功，JSON stdout 不混入提示 |
| L2 | `issue status MUL-1 <custom-done>` | 以 hint `target_category=done` 判定，发送原 custom key；不本地猜 membership |
| L3 | `issue update MUL-1 --status done` | 与 L1 完全相同；保留 `--no-start` 和 output 行为 |
| L4 | off/no project/non-done | hint not_applicable 后一次原 PUT，无虚假 warning/success |
| L5 | legacy 404/405 或缺 revision | 兼容 bypass，原命令仍可成功/失败，结果由 PUT 决定 |
| L5a | 结构化 404/403/409 | 非零退出，不发送 PUT，不把资源/权限错误伪装成旧 server |
| L6 | hint 明确拒绝/409 stale | 非零退出，不发送 PUT，错误包含可行动 code/刷新方向 |
| L7 | hint timeout/5xx/malformed/commit unknown | 非零退出，不发送 PUT，不打印 `status changed`，不自动重试 |
| L7a | CLI Ctrl-C 在 hint 阶段 | 请求取消且不发送 PUT；用户显式重新执行才重试 |
| L8 | 实际 PUT 428/403/409/5xx | 非零退出，保留原 HTTP code/body 分类；不把提示结果当成功 |
| L9 | `issue update --status done --title ...` | 本轮不接入 hint gate；保留服务端混合更新合同，文档明确 out-of-scope |
| L10 | `issue create --status done`、comment-child、batch | 本轮无 hint 请求；服务端 v1/旧行为独立验收，不把本包计为覆盖 |

## 合同与回归

| ID | 检查 | 预期 |
|---|---|---|
| R1 | `packages/core` schema/client/pure tests | 通过；无 `as T` 网络解析、无新依赖 |
| R2 | `packages/views` actions/detail/table/modal tests | 通过；无 Next/router 越界、无 shared UI 直接写私有 key |
| R3 | CLI Go tests | 通过；`gofmt`、`go vet` 通过，未知结果非零 |
| R4 | typecheck/lint | 受影响 workspace 通过；既有 warnings 单独记录，不压制 |
| R5 | skill/source-map contracts | 命令行为、hint 非授权语义、custom done 和旧 server fallback 与源码一致 |
| R6 | `git diff --check` / scope review | 无 server completion writer、migration、skip、authorization、provider、deployment 或生产改动 |

## 不得据此宣称

通过本矩阵只能说明已规划或已实现并验收了所列普通单 issue UI/CLI 提示消费者。它不能说明 batch/move/create/comment-child/VCS/collaboration/R38 已覆盖，不能说明完成事务 v1 重新验收，不能说明完整 M1、M2、M3、真实供应商或生产部署完成。
