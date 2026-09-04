# 桌面端首次启动：后端地址配置设计

## 背景与目标

当前桌面端在主进程启动时读取 `~/.multica/desktop.json`：文件不存在时静默使用 Multica Cloud，文件存在但无效时阻塞启动。这个行为对云端用户透明，但私有化部署用户必须手工创建 JSON、重启应用，且容易把内网用户误导到云端。

目标是让首次打开桌面端的用户在登录前完成环境选择和后端地址配置，同时保持现有 API/WS 初始化顺序、守护进程 profile 隔离和已配置用户的无感启动。

## 用户流程

1. **读取配置**：主进程区分 `configured`（有效 `desktop.json`）、`missing`（首次运行）和 `invalid`（文件存在但解析失败）。开发模式仍完全使用 `VITE_*`，不弹配置页。
2. **首次运行页**：`missing` 时渲染阻塞式 `DesktopEndpointSetupPage`，不创建 `CoreProvider`，因此不会在地址确定前发出 `/api/me` 或 WebSocket 请求。页面保留桌面端 `DragStrip` 和现有登录页的居中、窄表单视觉。
3. **选择环境**：顶部使用分段控件：`Multica Cloud`（默认）和 `私有化部署`。Cloud 显示只读的 `https://api.multica.ai`；私有化显示必填“后端地址”输入框，支持粘贴内网域名/IP 和端口。
4. **验证连接**：输入框失焦做格式校验；点击“测试连接”由主进程请求 `<apiUrl>/health`，5 秒超时。成功显示绿色状态和响应耗时；失败保留输入并显示可读错误（DNS、TLS、超时、非 2xx 分开），仍允许用户保存，便于需要登录后才可访问健康检查的部署。
5. **保存并继续**：保存按钮在格式无误后可用。主进程规范化 URL，补齐 `wsUrl`/`appUrl`，原子写入 `~/.multica/desktop.json`（先写临时文件再 rename），成功后返回新配置并重载窗口。重载后正常进入现有登录页；Cloud 选择也写入配置，避免下次再次询问。
6. **后续修改**：设置中增加“服务器地址”入口，复用同一表单。修改需要确认“保存并重启”，保存期间暂停 daemon；重启后重新建立 API、WS 和匹配的 daemon profile。不要在运行中的 `CoreProvider` 上热替换 base URL。

## 状态与异常

| 状态 | 页面行为 |
| --- | --- |
| `missing` | 首次配置页；关闭窗口会保留 `missing`，下次继续配置 |
| `invalid` | 现有错误页改为提供“重新配置”按钮；不回退 Cloud，避免误连外网 |
| 地址格式错误 | 字段级错误，阻止测试/保存 |
| 连接失败 | 显示失败原因，可修正后重试；保存仍可用 |
| 写文件失败 | 保留表单，提示权限/磁盘错误，不重载 |
| 重载中 | 禁用控件，显示短暂 loading；失败时回到配置页并保留草稿 |
| 已有有效配置 | 不显示首次页，保持当前启动路径 |

## 实现边界

### 主进程与 preload

- `apps/desktop/src/main/runtime-config-loader.ts` 返回 `source`/`needsSetup`，不能只返回“缺失即 Cloud 默认”，以便 renderer 决定是否阻塞。
- 在 `apps/desktop/src/main/index.ts` 增加受发送方校验的 IPC：
  - `runtime-config:test`：只接收规范化 `http/https` URL，主进程网络请求 `/health`，禁止读取/写入任意路径。
  - `runtime-config:save`：接收 `apiUrl` 以及可选 `appUrl`/`wsUrl`，复用 `parseRuntimeConfig`，在 `desktopConfigPath()` 下原子写入并更新内存结果。
  - `runtime-config:reload`：保存成功后通知 renderer 重载；不暴露 token 或文件内容。
- `apps/desktop/src/preload/index.ts` 和 `index.d.ts` 暴露最小 typed API。`runtimeConfig` 仍是启动快照，避免 CoreProvider 在会话中途改变 endpoint。

### Renderer/UI

- 新建 `apps/desktop/src/renderer/src/pages/endpoint-setup.tsx`，由 `App.tsx` 在 `runtimeConfigResult.ok === false` 且 `needsSetup` 时优先渲染；Issue window 也必须阻塞到主窗口完成配置。
- 复用 `@multica/ui` 的 `Input`、`Button`、`Label`、`Alert`；不在 `packages/views` 放 Electron IPC 或本地文件逻辑。
- 文案进入 `packages/views/locales/{en,zh-Hans,ko,ja}`，至少覆盖标题、环境选择、校验、连接状态、写入失败和重启提示。
- 采用键盘可达顺序（环境分段 → 地址 → 测试 → 保存），错误通过 `aria-describedby` 关联；不把 URL 放到 telemetry 或错误上报中。

## 配置与安全约束

- 配置文件格式继续使用 `schemaVersion: 1` 和 `apiUrl`，保持与现有文档兼容；默认推导规则继续由 `parseRuntimeConfig` 负责。
- 只允许 `http`/`https`，清除 query/hash；允许内网 `localhost`、私有 IP 和自签名 TLS（TLS 错误应明确展示），不允许 `file:`、用户名密码或 token。
- 健康检查需限制重定向到同源，设置请求超时和响应体上限；错误只返回分类和安全的简短消息。
- 写文件使用 `mkdir(..., { recursive: true })`、临时文件 `0600`、`rename`；写失败不能覆盖旧配置。
- 私有化发行版应增加构建级 `REQUIRE_RUNTIME_CONFIG=1`（或等价签名配置）开关：开启时禁止 Cloud 选项，缺失配置只能进入私有地址表单，避免企业客户端把数据发往公网。

## 测试与验收

- `runtime-config-loader.test.ts`：区分 missing/configured/invalid；保存后重新读取结果一致；原子写失败保留旧文件。
- `runtime-config.test.ts`：输入规范化、派生 URL、禁止凭据和非法 scheme。
- 新增主进程 IPC 测试：非 BrowserWindow sender 被拒绝、`/health` 超时/重定向/非 2xx 分类正确。
- 新增页面测试：首次运行阻塞 CoreProvider、Cloud/私有化切换、字段校验、测试连接 loading/成功/失败、保存后触发重载；invalid 状态的“重新配置”路径。
- 手工验收矩阵：全新用户、已有 Cloud 配置、已有内网配置、损坏 JSON、无写权限、内网离线、自签名证书、Windows/macOS/Linux 路径和中文/英文 locale。

## 分阶段发布

1. 先落地主进程配置状态和 IPC，默认不改变已有 `configured` 用户。
2. 发布首次配置页，并以 Cloud 默认选项兼容公共版本；私有化构建打开 `REQUIRE_RUNTIME_CONFIG`。
3. 增加设置入口和可观测性（仅记录配置来源、连接结果类别、耗时，不记录地址），稳定后再考虑多环境配置列表。
