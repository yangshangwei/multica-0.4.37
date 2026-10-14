# Research: 客户端设计与实施顺序静态审查

- Query: 审查 design D04/D09–D11、implement P6/P8 与验收归属，找出 Web/Desktop、schema、缓存撤权及受控附件的具体缺口。
- Scope: internal；只审查规划，不修改主文档或产品，不执行产品测试。
- Date: 2026-10-12。
- 输入快照: design.md `90a4dded6aba`、implement.md `361e75727489`（SHA-256 前 12 位）；下列行号对应该快照。R39 已确认仅新建，O11 仍未决。
- 依据: 已完成的 `account-workbench-client-design.md` 及本轮少量定点复核。没有重扫权限/队列实现。

## Findings

### C01 — P1：Web bearer 会话的原生附件请求缺少实际认证通道

- 位置: `design.md:208`；`implement.md:80`、`:128`、`:143`。
- 问题: D09 为 Web 规定认证代理，只对 Desktop 明确按用户/对象/权限版本兑付的短能力。Web 目前仍支持 legacy token 会话：`apps/web/components/web-providers.tsx:21`、`:74`。这类会话可以没有认证 cookie；ApiClient 元数据请求能带 bearer，浏览器原生 `<a>`、`img`、media/range 请求却不能自动带这个 header。现有下载代码明确以签名 URL 解决此问题（`packages/views/editor/use-download-attachment.ts:149`、`:169`）。新设计禁止旧签名 fallback 是正确边界，但必须补上合法替代，否则有效协作者在受支持的 Web 会话中可能只能读元数据而不能预览/下载，影响 R05/R22、AC05/31/67。
- 最小修订: 将同一可兑付、实时重验且只代理字节的短能力明确覆盖无 cookie 的 Web 原生资源请求；文本等 fetch 路径可使用 Authorization。D09 固定元数据/能力兑换/内容流的契约，cookie 与 bearer 均不得 fallback 公共 uploads/S3。P3/P6 增加“无认证 cookie、真实 bearer 会话”的浏览器图片、下载、range 与撤权后重取负例。无需新增登录类型，也不能强制旧会话先登出重登。

### C02 — P2：仅新建的关键 UI 没有明确实施项，AC67 主证据早于附件阶段

- 位置: `design.md:191`、`:216`；`implement.md:118`、`:128`、`:156`。
- 问题: D01/D09 有原子新建服务，但 P6 的界面清单列出工作台、详情、交接、队列等，未明确新建协作任务入口/表单和私有草稿上传接线。R39 排除了转换，不能依赖现有普通任务新建或交接入口自然产生本功能单据。AC67 又整体归 P1，然而“附件首次上传即受控”的实现属于 P3，P1 的 DB 创建负例不能证明它；前端真正点击新建也未列入 P6 的验证落点。
- 最小修订: P6 增加明确的新建协作入口和表单：有权来源空间、默认本人/精确目标、私有 draft 上传、一次原子提交、成功后打开同一 UUID，失败保留输入且无半单。至少在工作台可发现，不暴露存量 enable/convert。AC67 保留一个最终主验收，但注明 P1 原子创建、P3 首次上传、P6/P8 双端实际创建三项证据均齐全后才能判定通过；不需要改写或重复新增 AC。

### C03 — P2：工作台查询键与缓存重验触发还不够明确

- 位置: `design.md:190`、`:222`–`:224`；`implement.md:117`、`:124`、`:128`。
- 问题: D10 规定 server/viewer/issue/session 身份，却没有为账号列表明示 `view`、有效筛选/排序的查询身份；current 与 participating 不能共享一个结果缓存。撤权段明确收到失效后的清理，但没有固定重新挂载/重连的权限快照重验。仓库默认 `staleTime: Infinity`、`refetchOnWindowFocus:false`（`packages/core/query-client.ts:3`），只继承默认值会让错过失效事件的已缓存详情继续被视为新鲜，无法靠普通重挂载发现 403。
- 最小修订: 列表 key 包含归一化 view/筛选/排序，分页 cursor 按 infinite-query 契约管理；详情/控制单独固定挂载、账号 WS 重新认证与重连的 revalidation 触发。明确取消受保护请求使用防回滚的清理顺序，并清除组件局部 transcript/HTML 内容，不能只删 Query entry。保持“临时 500 可保留上次授权内容”和“权威 403 必须清理”之别，不承诺离线收回已下载字节。
- 最小验证: current→participating 切换不会复用错误结果；详情有暖缓存时断开账号 WS、撤权、重新打开并恢复连接，即使没有收到原失效事件，也会通过快照获得拒绝并清理；来源成员身份仍有效时只恢复基础投影。迟到成功和取消请求的默认 revert 均不能复活旧控制权限。

### C04 — P2：新增十进制版本字符串与旧 Issue schema 的边界需固定

- 位置: `design.md:36`、`:193`、`:224`、`:231`；`implement.md:60`–`:61`、`:117`。
- 问题: “revision 等 API 传十进制字符串”未界定是否包含旧 generic issue 的 `revision`。现有 `IssueSchema.revision` 必须是 number，混合列表经 `z.array(IssueSchema)` 校验（`packages/core/api/schemas.ts:1324`、`:1333`）。如果给共享 issue 的旧列表行改为字符串，已安装 Desktop 可能把整个列表降级为空；新协作 DTO 若直接复用该 schema 也会拒绝正确的字符串。直接用 Number/parseInt 或字符串字典序比较则分别有精度和顺序问题。
- 最小修订: D02/D09 明确旧字段 wire 类型保持兼容，新协作业务 revision/control_version/seq 使用独立命名的十进制字符串；禁止把新 DTO 直接填进旧 Issue 缓存。新比较器按无损整数语义处理，关键 schema 不接受小数/科学计数法/越界值。P1/P6 测试旧 ordinary+shared 混合投影、`9`/`10` 顺序和大于 `Number.MAX_SAFE_INTEGER` 的版本，不把解析失败报告为无任务。

### C05 — P2：来源状态目录 revision 是未给来源的前后端契约

- 位置: `design.md:82`、`:94`、`:193`；`implement.md:15`、`:69`。
- 问题: D04 要求目录 revision 变化拒绝旧选择，但当前状态项是 key/category/archived_at/updated_at 等字段，没有可直接复用的目录 revision（`packages/core/types/issue-status.ts:21`；`packages/core/issue-statuses/queries.ts:21`）。D02/D09 也未指定该版本的生产、传输及提交字段。仅有 issue/collaboration revision 不能检测一个 admin 刚归档了所选状态。
- 最小修订: 固定一个本单动作状态选项的版本凭证/摘要及提交字段，或明确比较选中状态的稳定身份与 updated_at/归档条件；服务端锁内重验来源目录。返回 fallback 需重新确认时同样带该凭证。P0 固定后再让 P2 与 P6 消费同一契约；测试弹窗打开后归档/改目录，不能静默采用其他 key。既有七类别和 D04 的交接映射无需重开产品选择。

### C06 — P2：Desktop 还需要解除初次 workspace 查询失败的全窗口门禁

- 位置: `design.md:216`、`:220`；`implement.md:120`–`:122`、`:128`。
- 问题: 计划明确零空间，但“成功取得空列表”不同于“身份已确认、workspace list 从未成功”。现有 Desktop `App.tsx:351` 会在 workspaceListUnavailable 时直接显示全窗口认证恢复页，尚未进入账号 TabGroup；`:305` 的默认空间选择也依赖这份列表。仅添加 account route/sanitizer 和测试零空间，仍可能留下账号工作台被无关空间列表错误挡住的路径。
- 最小修订: P6 点名调整 App 启动/恢复和 Desktop shell 条件：已验证身份下，账号目的地不以 workspace list 成功为前提；账号视图的恢复不会自动换成首个空间或 new-workspace 浮层。认证失败、账号设置/强制改密仍严格阻断。新增定点测试：`getMe` 和 workbench 成功、workspace list 500，仍能打开/恢复账号页并只在空间选择部分提示重试。

### 已覆盖且不应重新打开的选择

- `/account?issue=`、一个持久账号 tab、按 issue/view 的 memento、旧 tab 更新 query、旧布局镜像所有权和换账号清理均已有明确设计；没有发现把工作台误作 WindowOverlay 的问题。
- R39 仅新建已落实到权威关系、专用创建 API 和禁止转换；无需为旧公开附件或旧执行编写迁移实现。
- D04 来源类别映射、层级补充关系、todo 默认值和仅聚合协作单的范围均已说明；未发现它们暗中引入新的状态类别。默认新会话 paused、手动恢复和暂不隐式重试也有独立说明，不当作未说明的行为改变。
- R38 的 UI 只做三个窄代管动作，O11 明确禁止未确认前实现附带转交字段；本审查不替用户选择该分支。

## 文件、规范与外部参考

- 主审文件: `design.md`（D04/D09–D11），`implement.md`（P6/P8/归属）；对照最新 `prd.md` R01–R39、AC01–AC68。
- 源码模式与行号沿用 `research/account-workbench-client-design.md`；本轮仅定点确认 Web legacy bearer、旧 Issue revision schema、原生下载和 Desktop workspace 错误门禁。
- 相关规范: 根 `CLAUDE.md` 的 API Compatibility/Testing；`.trellis/spec/core/frontend/api-client-auth-epoch.md`、`iteration-operations.md`；`.trellis/spec/views/frontend/desktop-core-accessibility.md`；Web `e2e-run-environment.md`；Desktop `renderer-file-access.md`。
- 外部参考: 无新增联网资料或依赖；仍以当前仓库的 Query v5/Zod v4、Next 16/Electron 39 声明及已有实现为依据。

## Caveats / Not Found

- 这是静态规划审查，C01 为需先闭合的支持路径，C02–C06 为可执行契约/验证落点修订；并非已复现的产品缺陷或运行验收结果。
- 未发现客户端范围内遗漏整组已确认 R/AC；主要缺口在如何让若干声明实际可实施、可验证。权限/队列内部正确性由另外两条审查负责。
- 只写本文件，未改主文档、现有研究或产品；未执行产品测试/浏览器/真实 CLI，也未把 planning 标为完成。

## C01–C06 定点回查（2026-10-12）

- 快照: design.md `f96f65b15b07`、implement.md `22d76a1755e6`。只检查原问题及直接回归。

| 问题 | 规划结论与修订证据 |
| --- | --- |
| C01 | 已关闭。D09 `design.md:202` 同时覆盖 Web bearer 原生资源与 Desktop 的重验短能力；`implement.md:82`、`:88` 明确无 cookie 浏览器预览/下载/range 和撤权负例。 |
| C02 | 已关闭。`design.md:214`、`implement.md:125` 明确新建表单、私有 draft、原子提交及未知结果恢复；`:136` 安排双端点击，`:171` 将 AC67 最终归 P8，须 P1/P3/P6 证据齐全。 |
| C03 | 已关闭。`design.md:218`、`:220` 明确列表 view/filter/sort、挂载/聚焦/WS 重验、revert:false 和局部 transcript 清理；`implement.md:132`、`:136` 有对应暖缓存撤权回归。 |
| C04 | 已关闭。`design.md:36` 隔离新增 collaboration_revision 字符串与旧 Issue.revision 数字，禁止混写缓存；`implement.md:123`、`:136` 覆盖旧混合列表、9/10 和超安全整数。 |
| C05 | 已关闭。`design.md:99` 定义 status_options_version 摘要、提交及目录 fence 内校验；`implement.md:15`、`:71`、`:77` 固定生成/消费与目录变化测试。 |
| C06 | 已关闭。`design.md:214`、`implement.md:128` 区分 workspace list 500 与认证失败；`:136` 明确身份/工作台成功而空间列表失败的验证场景。 |

未发现这六项修订直接引入的新回归。“已关闭”仅表示规划缺口已补齐，不表示产品实现或 AC 验收通过。O11 仍未决；本次未修改主文档/产品，未运行产品测试或改变实现授权。
