# MCP「部署提供」设计与实施计划

> **For implementers:** Use `superpowers:executing-plans` after the design is accepted. This document is a proposal, not evidence that the feature has been implemented.

**Goal:** 部署管理员向服务器目录投放 MCP 模板，Web 和桌面端即可在「MCP 市场 → 部署提供」发现、配置、保存并明确分配。

**Architecture:** 增加部署目录来源，与平台内置模板共用目录查询和服务端解析器。目录只保存模板，现有工作空间 MCP 表继续保存独立配置快照；不新增目录数据库、安装器或执行通道。

**Tech Stack:** Go 标准库、现有 PostgreSQL/sqlc、React、TypeScript、Zod、TanStack Query、共享 UI。

**Status:** IMPLEMENTED — 用户于 2026-10-04 授权建 Trellis 任务推进；实现已合回原工作区，验收见 `.trellis/tasks/archive/2026-10/10-04-mcp-deployment-catalog/verification.md`。

---

## 1. 范围与用户流程

本轮按 skill「部署提供」的现有语义设计：目录属于 **API 部署服务器**，模板供连接这一部署的工作空间成员发现。智能体执行机器、API 服务器和桌面端电脑可能是三台不同的机器。

```text
部署管理员投放 mcp.json
    → API 扫描、校验、合并目录
    → MCP 市场：全部 / 部署提供 / 平台内置
    → 工作空间 owner/admin 填写必要参数并保存
    → 保存为工作空间 MCP 配置
    → 用户明确选择智能体并分配
```

- 增删改模板不要求重新构建服务端或客户端，也不要求重启已经配置好挂载的 API。
- 首次启用需要部署新后端和支持新来源的客户端，并配置挂载；后续投放无需升级。
- 投放的是连接/启动配方。它不会把服务器目录里的可执行文件分发给智能体，也不会安装软件包。
- 维持现有权限：人类 owner/admin 可创建；成员可发现模板、按现有权限复用和分配已有配置；智能体身份不能借管理员凭证创建工作空间配置。
- 第一版不增加桌面端目录扫描、ZIP 上传按钮、公开投稿、workspace 私有目录、自动更新已保存配置或自动分配。后续若需要 UI 上传，应接入同一服务端校验器，而不是另造市场。

## 2. 方案选择

| 方案 | 复用与代价 | 判断 |
| --- | --- | --- |
| A. 服务端挂载目录 + 现有 MCP 创建链路 | 复用 skill 的部署模式、MCP 的配置表和分配流程；新增目录解析、来源、陈旧版本校验 | 推荐，完整满足当前目标 |
| B. 上传 JSON 后直接创建工作空间自定义 MCP | 改动较少，但每个工作空间都要重复导入，也没有部署级市场 | 适合一次性配置，不满足当前范围 |
| C. 数据库目录 + 上传/发布管理后台 | 长期适合权限分层、审核和历史版本管理；增加目录实体、文件存储、后台和发布生命周期 | 当前没有这些需求，暂不引入 |

不直接抽象一个横跨 skill、MCP、插件的通用市场服务。复用明确的 UI 和校验模式，保留各自的数据与执行语义。

## 3. 投放格式

环境变量建议使用 `MULTICA_MCP_TEMPLATE_DIR`，保持与 `MULTICA_SKILL_TEMPLATE_DIR` 一致。

```text
部署目录/
  mcp-templates/
    company-search/
      mcp.json
    internal-code-tools/
      mcp.json
```

Compose 默认将宿主机 `${MCP_TEMPLATE_DIRECTORY:-./mcp-templates}` 只读挂载到 `/app/data/mcp-templates`，容器内设置 `MULTICA_MCP_TEMPLATE_DIR=/app/data/mcp-templates`。只读约束作用于 API 容器；管理员可在宿主机更新文件。

示例（远程、需令牌的内网 MCP）：

```json
{
  "schema_version": 1,
  "titles": { "zh": "公司知识检索", "en": "Company Search" },
  "descriptions": { "zh": "检索内部知识库。", "en": "Search the internal knowledge base." },
  "category": "documentation",
  "documentation_url": "https://docs.example.internal/mcp",
  "requirements": {
    "zh": ["智能体所在机器能访问公司内网"],
    "en": ["The agent machine must have access to the internal network"]
  },
  "config": {
    "type": "http",
    "url": "https://search.example.internal/mcp"
  },
  "inputs": [
    {
      "key": "access_token",
      "labels": { "zh": "访问令牌", "en": "Access token" },
      "required": true,
      "secret": true,
      "target": { "kind": "header", "name": "Authorization", "prefix": "Bearer " }
    }
  ]
}
```

- 一级目录名是唯一的模板 `key`，使用现有安全名称语法；manifest 不再重复一个可能与目录名冲突的 key。
- `schema_version` 是文件格式版本，**不是配置内容版本**。第一版仅接受 `1`。
- 标题至少提供一种非空 `zh`/`en`；按请求语言、英文、中文回退。描述与要求可省略；category 使用现有枚举，缺省 `other`。文档地址可省略，提供时须为合法 HTTPS URL。
- 第一版提供 `stdio` 和 Streamable HTTP；自定义 MCP 编辑器既有的其他协议不受影响。stdio 的 command 必须非空、args 是字符串数组；HTTP 的 URL 必须具体、非空，不能包含 userinfo、query 或 fragment。允许内网 HTTPS 主机/IP；如需明文 HTTP，必须设置 `MULTICA_MCP_TEMPLATE_ALLOW_HTTP=true`，默认关闭，只影响部署目录校验，UI 展示该传输要求。
- stdio 的 command/args 是 **智能体机器** 的启动配置，不能理解为相对 `mcp-templates/` 的路径。API 不执行命令、不下载包、不探测 URL。
- 文件不承载真实凭据。env/header 值通过下面的输入声明收集，不提供默认秘密值；初版禁用 manifest 的静态 env、headers 字段。任意字符串是否是秘密无法由正则可靠判定，文档须明确管理员不得将秘密放进标题、描述、参数或 URL。
- 单条 manifest 不包含图标图片、脚本附件或可执行文件。部署模板使用统一 MCP 图标/既有类别图标，避免新增资源托管。

## 4. 受限参数映射

沿用 `template_inputs: Record<string,string>` 提交方式，将当前仅为内置模板实现的输入解析整理为公共内部函数。已有 `project_path` 和 `database_url` 的跨平台/数据库校验必须保留。

部署文件可以声明的 target 只有：

| kind | 允许内容 | 执行效果 |
| --- | --- | --- |
| `env` | 固定环境变量名 | 设置单个 env 值；只适用于 stdio |
| `arg` | 固定 flag，可选 | 在 args 尾部追加 `[flag, value]` 或 `[value]`；只允许非 secret 输入 |
| `header` | 固定 header 名、可选固定前缀 | 设置一个 header；只适用于 HTTP，值直接拼接有限固定前缀，不做模板求值 |

输入名唯一，目标唯一；拒绝重复目标、未知 target、跨协议目标、非法 flag/env/header 名、换行/NUL、超长值和未声明输入。header 默认视为 secret；凭据不可映射到 argv。有限 validator 类型可包含 `string`、`absolute_path`、`database_url`，无需允许运营方填写正则或代码。

不实现任意 JSONPath、`${...}`、shell 展开、动态 command 或动态 URL；用户填写的内容永远只是某个参数值。现有每值 8192 字节、请求输入 65536 字节的约束继续生效。输入映射顺序按 manifest 数组顺序，明确检测目标冲突。

## 5. 来源和版本

### 来源必须持久化

模板身份为 `(source, key)`，source 是 `builtin` 或 `deployment`。两种来源允许同 key，各自显示并独立解析，部署文件不能覆盖内置配方。用户保存时的配置名称仍遵循工作空间内唯一规则。

- 目录响应增加 `source`。
- 创建请求增加可选 `template_source`。旧请求省略时只查 `builtin`；新客户端明确发送来源；未知来源拒绝。
- 工作空间 MCP 表增加可空 `template_source TEXT`，已有 `template_key` 非空的记录回填 `builtin`；自定义配置维持三个来源字段都为空。没有新表、索引、外键。
- 修改 SQL/sqlc 和所有共享、智能体绑定列表返回来源字段。重命名保留来源；替换完整 config 清空 source/key/version 三项。
- 卡片 React key、相关配置匹配、可用性检查、图标选择均按 source+key，不能再仅比较 key。内置专属图标仅限 builtin 来源，避免部署同名条目冒充平台模板。
- 保存后的来源由数据记录决定。删除目录条目后仍能准确标注已保存配置的来源，不能通过当前目录反查来源。
- 为兼容已安装旧客户端，所有返回 MCP 来源摘要的 HTTP 入口使用可选查询参数 `mcp_source_version=1` 协商完整来源（工作空间列表、创建/修改响应、智能体绑定列表等一并覆盖）。新 core 客户端统一发送；旧服务端忽略这个附加参数即可。未声明能力时，部署实例仍返回 id/name/transport 等普通管理字段，但 template_key/version/source 返回 null；内置实例保持原状。由一个共享序列化边界执行此规则，避免旧 UI 将 deployment/playwright 误归为 builtin/playwright。此处不用新请求 header，避免增加跨域预检配置。

### 部署版本由内容生成

不要求部署者手动维护递增版本。对完成校验、填充统一默认值的 manifest 规范化后，用 SHA-256 生成不透明 `version`（例如 `sha256:<64 hex>`）。包含配置、输入约束/映射及全部展示元数据；忽略 JSON 的空白和对象键顺序，保留数组顺序。固定跨进程、跨副本的版本化域常量（例如 `multica-mcp-catalog:v1`）与规范化格式；同一内容在重启、不同副本和不同语言请求中必须得到同一版本。格式升级不得无意复用旧哈希语义。

现有 API 和数据库的 `template_version` 已是字符串，现有前端没有数字比较，因此可直接保存这一版本，无需另加 revision 列。平台内置的 `"1"`、`"2"` 等版本保持原语义；部署来源 UI 展示来源与更新提示，不将长哈希作为面向用户的版本标题。

服务端创建时重新读取选定来源并比较版本，**从同一次读取到的已校验对象**解析配置，不在比对后再读一次文件：

- 文件更新、删除、变坏：旧表单不能保存为新内容，返回明确的模板变化/不可用错误。
- 服务端成功解析后文件再变化：这次创建保存已经接受的快照，下一次创建再读取新版本。
- 删除和更新目录不会修改已保存的工作空间实例，也不会改变已有智能体绑定。
- 回滚到完全相同的已校验内容会得到相同版本，这符合内容快照语义；不把哈希声称为发布时间或可执行包版本。

错误复用现有 `writeErrorCode` 和 `ApiError`，不靠英文错误文本判断：

| 状态 / code | 情形 | 客户端动作 |
| --- | --- | --- |
| 409 / `mcp_template_changed` | 条目存在，但当前内容版本不同 | 禁止旧草稿保存，刷新目录，要求重新检查新模板 |
| 409 / `mcp_template_unavailable` | 所选条目已删除或单条校验失败 | 禁止新建，提示撤下/不可用；已有配置仍可正常管理 |
| 503 / `mcp_catalog_unavailable` | 目录级 IO、权限或总量限额错误 | 保留草稿和缓存列表，提示稍后重试；不得判断为删除 |
| 400 / 现有参数错误 | 未知来源、非法身份组合或输入不合法 | 保留草稿，定位输入错误，不自动重载模板 |

## 6. API 与后端边界

增加小型 `McpCatalog` 服务值（例如仅含 `Directory string`），由 handler 初始化时注入；零值只返回内置模板。列表与创建解析统一经过这个对象，避免“列表能看见但保存找不到”。不要将文件扫描散落在 handler，也不要让纯解析函数自行读取环境变量。

现有接口继续使用：

```text
GET  /api/workspaces/{id}/mcp-servers/templates?language=zh
POST /api/workspaces/{id}/mcp-servers
```

部署模板目录响应只返回展示元数据、`source`、`version`、`transport` 和公开 `inputs` 元数据。**不返回 manifest config、输入 target、服务器文件路径或凭据**。内置条目继续返回当前公开 config，以维持旧客户端兼容；新客户端先读 transport，内置旧响应缺字段时仍从 config 推导。

新的部署保存示例：

```json
{
  "name": "company-search",
  "template_source": "deployment",
  "template_key": "company-search",
  "template_version": "sha256:<catalog-returned-digest>",
  "template_inputs": { "access_token": "<user-entered-value>" }
}
```

请求不得同时带原始 config 与模板身份。source 存在但 key/version 缺失也拒绝；不能通过省略或伪造 source 选择另一个目录。部署模板不提供退回自定义编辑器的旁路。

### 文件读取与诊断

- 每次目录请求读取；不引入 watcher、定时任务或额外索引缓存。第一版按小型部署目录设计。
- 空变量、目录缺失或空目录，成功返回内置内容。目录权限/IO 错误记录简洁日志，并让目录 GET 和依赖该目录的部署创建返回 503 / `mcp_catalog_unavailable`，不能用“200 + 仅内置”假装目录撤空。GET 首次失败显示目录错误；已有缓存则保留上次完整列表。工作空间已保存配置管理及内置模板创建不依赖部署目录健康。
- 单条坏文件跳过并记录 key 与错误类别，其他合法条目仍可见；单条条目因损坏被跳过，当前成功快照中它即不可新建。
- 建议上限：单条 64 KiB、256 个一级条目、总读取 4 MiB。读取前及读取过程中限流量，枚举也有上限；目录总数或总读取超限返回同一 503，不随机展示一部分。目录读取预算包含坏条目的已读字节。
- 拒绝符号链接（目录与 manifest）、路径逃逸、非普通文件；使用根目录约束的文件打开方式避免先检查后读取之间逃逸。具体 Go API 在实施时按当前 Go 1.26 文档确认。
- 拒绝未知 schema、未知字段、重复 JSON key、尾随 JSON 内容、超深结构；不要依赖 Go 普通 Unmarshal 对重复键的默认行为。
- 投放建议先写临时文件，再在同一目录原子 rename 到 `mcp.json`，避免读取半个文件。即使错误投放也不保留可供新建的过期配方。
- 不打印 manifest 原文或输入值；日志给部署管理员定位，普通 UI 不显示服务器路径。
- 多副本 API 必须挂载同一份同步内容；目录不一致时可能安全地拒绝保存，但不能假装支持跨副本强一致发布。

## 7. 前端体验与兼容

复用当前 MCP 市场骨架，增加与 skill 相同的来源筛选：

```text
工作空间 MCP       MCP 市场
[搜索 MCP……]
[全部] [部署提供] [平台内置]
[全部类别] [开发] [数据库] ……
模板卡片：名称、说明、来源、传输方式、配置按钮
```

- 来源计数独立于搜索/分类；分类来自当前来源可用条目。来源切换后重置失效类别，保留搜索内容。
- owner/admin 点配置继续沿用 `McpSetupDialog`；填写参数 → 保存 → 独立分配。相关实例明确列出，复用由用户选择，不自动选同 key 的旧实例。
- 部署来源为空时提示“暂无部署提供的 MCP，请联系部署管理员添加”，不向普通成员显示环境变量与挂载操作。
- 模板查询挪为/接入 core 的 query options，key 包含当前 API base URL、workspaceId 和 language，使用公开平台 API 获取地址，不在 core 读取 process.env。
- 只在市场可见或智能体发现目录可见期间每 30 秒刷新；窗口聚焦、重连、重新进入和选择「部署提供」触发刷新。隐藏页面不轮询，避免多个桌面保活页签持续读取目录。
- 后台刷新失败保留列表并提供重试；首次失败不伪装成空目录。被移除/变更的选中模板不能继续提交旧草稿，提示更新后重新检查参数；普通短暂刷新错误不销毁用户输入。
- 若用户明确接受重新载入新模板，清空旧参数值，重新展示要求与输入；已保存后的分配步骤不因市场刷新而丢失状态。
- 新客户端遇旧服务端缺 source：按 builtin；缺 transport：沿用 config 推导。显式未知 source、坏 inputs 或部署缺 version 必须不可操作，不能默认成 builtin 或退到自定义保存。
- 旧客户端遇新服务端：部署条目没有 config，当前旧 `usableTemplates()` 会过滤它们；已保存部署实例经 `mcp_source_version` 能力协商隐藏来源 key/version，以普通配置显示和管理，不能误出现在内置同 key 模板的相关实例中。内置模板继续工作。这是一次客户端升级边界，未来新增部署模板无需再次升级。
- Web、桌面端共用 views/core，现有入口不新增 Electron IPC。真实更新部署模板不启动 MCP 程序。

## 8. 实施顺序与文件

每一阶段先补对应行为测试，再实现并运行本阶段检查。实现任务开始时重新确认工作区未提交 MCP 参数化模板改动，不覆盖其他工作。

1. **目录契约和解析器。** 新增 `server/internal/service/mcp_catalog.go`、`mcp_template_dir.go` 及同名 `_test.go`；整理 `builtin_mcp_inputs.go` 的纯输入解析，使现有内置测试继续覆盖相同行为。测试 tempfile 目录的增删改、坏条目隔离、哈希稳定性、目标映射、读取限额、路径与特殊文件处理。加无真实密钥的示例 `examples/mcp-templates/company-search/mcp.json`。
2. **持久化与 API。** 新建下一可用编号的 migration，不提前占用正在开发的编号；修改 `server/pkg/db/queries/workspace_mcp.sql` 后运行 `make sqlc`。更新 `handler.go`、`mcp_template.go`、`workspace_mcp_api.go` 及对应 tests，注入同一 Catalog；覆盖完整创建、来源、版本冲突、稳定错误码、来源能力协商、权限、只写摘要、rename/replace/delete。旧内置创建不变。
3. **core 协议与查询。** 更新 `packages/core/types/mcp-template.ts`、工作空间 MCP 类型、`api/schemas.ts`、`api/client.ts`、`workspace/mutations.ts`、`workspace/queries.ts`。为相关请求一致发送 `mcp_source_version=1`，将创建响应的身份核验从 key/version 扩展到 source/key/version。新增或扩充 boundary/request/query tests，覆盖旧响应、未知来源、坏 inputs、opaque version、API 地址切换、窗口刷新和轮询可见性。私密输入继续只留组件草稿，已结束 mutation reset 且 gcTime 为 0。
4. **共享界面。** 更新 `packages/views/settings/hooks/use-mcp-server-templates.ts`（可保留薄 locale wrapper）、`mcp/mcp-market.tsx`、`mcp/mcp-setup-dialog.tsx`、`mcp/mcp-agent-discovery.tsx`、`settings/components/mcp-tab.tsx`、`common/mcp-template-icon.tsx` 和 en/zh-Hans settings 文案。新增来源过滤的纯函数测试；组件只覆盖来源筛选、草稿陈旧、明确复用、保存后分配不中断等行为，避免重复纯函数矩阵。
5. **投放与验收。** 更新 `docker-compose.selfhost.yml`、`scripts/offline-bundle.sh`、`SELF_HOSTING.md`、中英文自托管文档、`docs/mcp-catalog-publishing.md`、`.trellis/spec/views/frontend/mcp-market.md`、builtin creating-agents skill/source-map。扩充 `e2e/mcp-market.spec.ts` 和 `e2e/mcp-desktop.spec.ts`：测试自己的临时部署目录，不依赖开发者本机目录。

可将阶段 1 和阶段 3 的基础类型设计并行；阶段 2 依赖目录解析和字段契约，阶段 4 依赖 API/core 契约，最终验证必须针对合并后的完整路径。无需新应用依赖。

## 9. 验收与验证命令

核心验收：

1. API 不重启，创建一个合法模板文件，Web/桌面端进入「部署提供」或下一个 30 秒轮询可发现。
2. 同 source+key 更新得到新版本；JSON 排版变化不改版本；重启/不同副本/不同语言保持相同版本；同 key 的 builtin/deployment 互不覆盖和误复用。
3. 模板浏览到保存之间发生修改、删除或损坏，不能落地错误配置；已保存配置和分配完全保留。
4. 分别验证正常空目录、单条坏文件和目录级不可读：空目录成功，坏条目隔离，目录错误返回稳定 503 并保留缓存/草稿；超限、符号链接、特殊文件不能导致任意读取或请求无限阻塞。
5. 公开目录、错误、日志、工作空间列表和 mutation/query 缓存不泄露输入值或 manifest 原文；只写参数在运行配置正确注入。
6. 权限保持原状；保存不分配，分配不代表启用/连通；不会在 API 或桌面渲染时执行 MCP。
7. 新旧客户端/服务端组合符合上一节兼容契约；专测新客户端保存 deployment/playwright 后，旧客户端打开 builtin/playwright 不会误匹配该实例。中英文、窄弹窗、后台失败与空目录有准确反馈。
8. 临时目录中的 stdio 与 HTTPS 模板都完成发现、保存和明确分配，测试无需运行第三方 MCP 程序。

计划执行的检查（本轮设计未运行这些应用测试）：

```sh
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test ./internal/service -run '^(TestMcpCatalog|TestMcpTemplateDir|TestMcpServerTemplates_)' -count=1
make check-mcp-catalog
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test ./internal/handler -run '^(TestMcpMarket|TestListMcpServerTemplates_)' -count=1 -v
pnpm --filter @multica/core test
pnpm --filter @multica/views test mcp/ settings/components/mcp-tab.test.tsx locales/mcp.test.ts
pnpm typecheck
pnpm lint
pnpm knip
pnpm exec playwright test e2e/mcp-market.spec.ts
```

Go handler 检查使用隔离、已迁移的 PostgreSQL，并确认目标测试实际运行，不能把 TestMain 跳过当作通过。桌面 E2E 按该测试文件现有 launcher/environment 配置启动，覆盖实际 Electron；实施者应记录具体命令与产物。补充 Go vet、触及的部署 shell 静态检查及格式检查。无需为设计文档运行整套应用测试。

## 10. 需要明确保留的限制

- 本方案解决整个部署共享的模板。如果要求不同工作空间看到不同模板，应重新设计目录授权，而不是以文件名隐藏。
- 目录模板不承担软件包镜像、二进制分发或运行时依赖安装；内网 stdio 程序和 HTTPS 信任链仍由运行环境准备。
- 新增来源列需要正常数据库迁移和发布流程。回滚不应删除已有部署实例的来源数据或将它们冒充内置模板。
- 当前工作区有其他 MCP 开发中的修改，文件位置与 API 细节需在开始实施时再次核对；本设计引用的是 2026-10-04 的现状。

## 11. 设计核对记录

2026-10-04 完成两轮独立只读评审，最终 PASS。第一轮发现并修正四项：旧客户端对同 key 的来源误关联、目录故障被当成空目录、缺少稳定的陈旧模板错误码、内容哈希只说明单进程稳定。相应恢复动作与验收条件已同步写入上文。

以上为设计阶段记录。随后用户授权创建 Trellis 任务，现已完成实现、独立复核和验收；最终结果见任务 verification.md。
