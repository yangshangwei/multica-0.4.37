# 新建任务：“帮我完善”服务端配置指南

适用范围：当前仓库版本的 Web / Desktop 新建任务界面。依据 2026-09-30 的代码核对。

## 1. 配置在哪里

手动创建和智能体创建中的“帮我完善”共用 Go 后端的 LLM 辅助服务，通过 `POST /api/issues/optimize-description` 调用模型。

**在运行 Multica API 的服务器上配置 `MULTICA_LLM_*` 环境变量，并重启后端即可。** 无需修改数据库或重新构建前端。

这套配置与执行任务的智能体独立：在客户端登录 Codex / Claude、配置守护进程或选择智能体模型，都不会自动启用这个功能。完善结果通过校验后会自动写入草稿，并提供撤销；它不会执行指令或直接创建任务。生成期间若用户编辑正文或开始提交，旧结果不会覆盖新内容。

## 2. 必要配置

| 环境变量 | 用途 | 默认行为 |
| --- | --- | --- |
| `MULTICA_LLM_API_KEY` | 模型服务或网关的 API Key | 空；有鉴权的服务必须填写 |
| `MULTICA_LLM_BASE_URL` | OpenAI 兼容接口的 API 根地址 | 未设置时使用 `https://api.openai.com/v1` |
| `MULTICA_LLM_DEFAULT_MODEL` | 网关实际支持、当前 Key 有权调用的模型 ID | 当前代码回退为 `gpt-5.6-luna`；建议显式填写可用模型 |
| `MULTICA_LLM_MAX_RETRIES` | 传输错误重试次数 | 空为 2；0 禁止重试；允许 1～5 |

API Key 和 Base URL **同时为空**时，辅助服务关闭。只填模型名不能启用它。免鉴权的内网服务可以只设置 Base URL 和模型名。

Base URL 应填写 API 根路径，例如 `https://llm.example.com/v1`，不要填写完整的 `/chat/completions` 地址。若供应商的兼容接口有其他前缀，使用供应商提供的完整 API 根路径。

上游需要支持：

- OpenAI 兼容的 `POST /chat/completions`。
- `response_format: {"type":"json_object"}`，并按提示词返回 `text` 和 `questions` 字段。
- `stream: true` 的 SSE 响应，用于界面中的流式预览。
- 当前请求的输出预算 `max_completion_tokens: 16000`。对明确报告不支持该参数的旧网关，代码可以改用 `max_tokens`；这不代表所有兼容性问题都会自动修复。

仅提供原生 Anthropic Messages 或其他非 Chat Completions 协议的地址不能直接使用，需要通过兼容网关接入。默认模型名来自本仓库，不代表你的供应商一定提供它。

## 3. Docker Compose 部署

在部署目录现有的 `.env` 中添加或修改下列配置，保留原有数据库、登录等配置。示例值必须替换：

```dotenv
MULTICA_LLM_API_KEY=replace-with-your-provider-key
MULTICA_LLM_BASE_URL=https://llm.example.com/v1
MULTICA_LLM_DEFAULT_MODEL=replace-with-your-model-id
MULTICA_LLM_MAX_RETRIES=2
```

仓库的 `docker-compose.selfhost.yml` 已将这四项传入 `backend` 服务，不需要再修改 Compose 文件。若使用自己维护的 Compose 文件，也要在 `backend.environment` 或 `env_file` 中显式注入变量；仅在目录中放置 `.env` 并不会自动将所有变量传入容器。

在同一部署目录执行：

```bash
docker compose -f docker-compose.selfhost.yml up -d --no-deps --force-recreate backend
docker compose -f docker-compose.selfhost.yml logs --tail=100 backend
```

沿用原部署的 `--env-file`、`-p` 和额外 `-f` 参数，确保操作的是同一个实例。不要仅执行 `docker compose restart`，它不会把修改后的环境变量更新到已有容器中。

注意：宿主机 shell 中已导出的同名变量优先于 Compose 的 `.env`。如果修改文件没有效果，应检查启动命令的环境来源。

### 使用内网模型服务

```dotenv
MULTICA_LLM_API_KEY=
MULTICA_LLM_BASE_URL=http://llm-gateway:8000/v1
MULTICA_LLM_DEFAULT_MODEL=replace-with-your-deployed-model-id
MULTICA_LLM_MAX_RETRIES=0
```

以上假设 `llm-gateway` 是后端容器能够访问的服务名；实际部署时替换成可达地址。有鉴权的网关仍须填写 Key。

容器里的 `127.0.0.1` 指向容器自身。如果模型运行在宿主机上，Docker Desktop 通常可使用 `host.docker.internal`；Linux Docker 需要配置宿主机映射或使用可达的内网地址。

## 4. 源码或进程部署

使用仓库 Makefile 启动时，在其实际加载的环境文件中设置同样的四项变量。默认优先读取 `.env`，没有时回退到 `.env.worktree`；也可通过 `ENV_FILE` 指定。重启对应的 API 进程后生效。

直接启动二进制、systemd 或 Kubernetes 部署时，将这四项注入 **API 进程环境**，再重启进程或滚动更新 Pod。不要假设二进制会自动读取工作目录里的 `.env`。密钥只放在服务端环境文件或密钥管理系统中，不要使用 `NEXT_PUBLIC_*` 变量，也不要提交到 Git。

这些设置在后端初始化时读取，不支持热更新；多副本部署需要更新所有 API 副本。

## 5. 验证是否配置成功

### 通过界面验证

1. 打开连接到该后端的 Web 或 Desktop，进入有权限的工作空间。
2. 新建任务，在智能体指令里输入：“检查登录页面，列出问题和修改建议，只给方案，不修改代码。”
3. 点击“帮我完善”，等待结果写入草稿。
4. 确认草稿保留“只给方案、不修改代码”的限制；可撤销本次完善，也可回答澄清问题后合并到草稿。

普通创建模式的“帮我完善”也使用相同配置。输入为空时按钮不可用。

### 通过接口验证

下面请求只获取优化建议，不创建任务。`MULTICA_TOKEN` 使用有目标工作空间访问权限的 Multica PAT 或 JWT，**不是模型服务的 API Key**。先在当前终端设置这些变量，再执行请求：

```bash
export MULTICA_API_URL='http://127.0.0.1:8080'
export MULTICA_WORKSPACE_ID='replace-with-workspace-uuid'
# Set MULTICA_TOKEN securely in the current shell.

curl --fail-with-body --max-time 60 \
  "$MULTICA_API_URL/api/issues/optimize-description" \
  -H "Authorization: Bearer $MULTICA_TOKEN" \
  -H "X-Workspace-ID: $MULTICA_WORKSPACE_ID" \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  --data '{"text":"检查登录页面，只给方案，不修改代码。","title":"登录页面检查","mode":"agent"}'
```

成功时返回包含 `text` 和 `questions` 的 JSON。`mode` 只允许 `agent` 或 `manual`。请求正文最多 128 KiB，指令或描述非空且不超过 20000 个 Unicode 字符，标题不超过 500 个字符。

测试流式链路时，将 `Accept` 改为 `text/event-stream`，并给 curl 加上 `-N`。预期依次看到 `text_delta` 和最终的 `done` 事件；只有 `done` 表示结果完整且通过校验。已开始的 SSE 响应即使 HTTP 状态为 200，也可能以 `error` 事件结束。

## 6. 常见问题排查

| 现象或错误码 | 含义与处理 |
| --- | --- |
| `503 / ai_unavailable` | API Key 和 Base URL 都为空，或配置没有注入实际 API 进程。检查环境文件、容器注入和重建是否生效 |
| `401`、工作空间访问错误 | 检查 Multica 登录状态、访问令牌和工作空间成员权限；这与上游模型 Key 是两层鉴权 |
| `400 / invalid_request` | 检查输入是否为空、长度是否超限、`mode` 是否正确，以及是否包含未知字段 |
| `429 / ai_busy` | 当前 API 进程已有 8 个优化请求并发执行；响应提供 `Retry-After: 5`，稍后重试 |
| `504 / ai_timeout` | 优化请求超过 45 秒。检查网络、模型排队与生成速度；优先选择延迟低的模型，缩短输入，必要时减少重试 |
| `502 / ai_generation_failed` | 上游调用失败。核对 Base URL、Key 权限、模型 ID、额度、TLS 和 JSON / SSE 协议支持；通过网关日志定位具体上游错误 |
| `502 / ai_invalid_output` | 返回值不是可用的 JSON，或模型改动了受保护的链接、附件引用、代码等。先用短纯文本测试，再检查模型的指令遵循能力 |
| JSON 请求成功，界面仍失败 | 单独测试 SSE；检查网关和反向代理是否支持流式响应、是否缓冲或提前断开 |
| 后端启动失败并提示 `invalid MULTICA_LLM_MAX_RETRIES` | 重试次数只能留空或填 0～5 的整数，负数、非数字、超过 5 都会导致启动失败 |

45 秒超时、单进程并发数 8、输入长度和输出预算目前是代码中的固定值，没有对应的环境变量。增加重试次数不会延长 45 秒的总时间。

反向代理应允许 SSE 及时转发，并把读取超时设置得长于 45 秒（例如 60 秒）。后端已发送 `X-Accel-Buffering: no`，仍需确认中间代理没有强制缓冲。上游详细错误不会从优化接口直接返回或记录，避免暴露提示词或凭据；排查上游拒绝原因时应查看模型网关侧日志。

## 7. 影响范围与关闭方式

这四项是部署级共享配置，同时用于聊天自动命名、聊天追问建议、新建任务的 AI 完善和创建助手推荐。当前没有仅为“帮我完善”单独配置 Key、模型或开关的环境变量。

点击优化会把当前描述或指令、可选标题和创建模式发送给配置的模型服务。Markdown 中的引用会发送；后端不会读取并发送附件文件内容。内网部署应将 Base URL 指向允许使用的内网网关。

要关闭整个服务端 LLM 辅助层，将 `MULTICA_LLM_API_KEY` 和 `MULTICA_LLM_BASE_URL` 同时清空，再重启后端。普通任务创建仍然可用，智能体执行任务所使用的独立凭据不受这两项控制。

## 8. 配置依据

- [环境变量样例](../.env.example)
- [自托管 Compose 配置](../docker-compose.selfhost.yml)
- [服务端环境变量读取](../server/cmd/server/router.go)
- [重试次数校验](../server/cmd/server/main.go)
- [LLM 客户端与模型默认值](../server/pkg/llm/client.go)
- [流式 JSON 生成](../server/pkg/llm/json_stream.go)
- [优化接口、限制和错误码](../server/internal/handler/issue_description_assist.go)
- [前端优化交互](../packages/views/modals/issue-description-assist.tsx)

本文依据代码和部署模板编写；实际模型连通性、权限及效果需按第 5 节在目标部署验证。
