# 验收环境与证据设计

## 来源与边界

本次产品基线为 `main@103e2bc32`。规划/报告写入任务目录；独立运行记录写入新 QA 产物目录。产品源码不在本任务中改造。历史整包报告及 launcher 可用于研究或复制编排代码，历史 allocation、clearance、数据库和通过结果不能复用。

不同验证层分别记录：纯函数/组件、Go handler+真实DB、浏览器+真实API、SimulatedDaemon、受控真实 daemon/provider/CLI、真实 Electron renderer/fixture Main、完整 production Main、installed artifact、原生 mobile。每个测试结果只能证明实际跨越的边界。

## 环境与资源隔离

- 生成新的 task/run 标识、非默认 JWT_SECRET、deployment ID、私有配置和来源清单；配置 mode0600，执行状态目录0700。报告不打印凭据。
- PostgreSQL只使用明确分配的新库；迁移前在同一环境检查 `current_database()` 等于 allocation 中的名称。service、handler、router、migrate 等 DB-backed package 使用分别迁移的新库，浏览器/API不共享其数据库。
- HOME、TMPDIR、TMP、TEMP、GOTMPDIR使用 Git 外且逐级所有者/mode合法的路径。明确选择 GOCACHE/GOMODCACHE/GOPATH；Go1.27 的 `t.TempDir` 会受 GOTMPDIR影响。
- API、Go普通测试和递归/fixture选择的环境分开；普通Go测试清除API专用 pool/publisher和task root覆盖，允许测试自己通过 t.Setenv 选择状态。
- 端口和process归属使用环境registry或独立可审计allocation，不采纳未知listener。不同 auth/gate lane顺序启动；记录共享PG连接预算，不调整max_connections或停止其他会话资源。
- Web使用生产构建/启动；Desktop资产先构建再服务，API CORS/WS origins与实际Web/native origin一致。当前工作区既有运行Web资产不得被替换。
- macOS production Main在单实例锁之前注册系统URL protocol，私有HOME/userData不能隔离LaunchServices。完整Main启动前以NSWorkspace保存原`multica://`默认应用的精确URL和同bundle现有注册URL集合，构建/安装只使用本任务稳定路径；退出后只对本次新注册路径执行`lsregister -u`并精确恢复/核对原默认与集合。无法证明该边界的subrun不得启动。

## 编排与审查

尽量使用既有 `scripts/check.sh`、`scripts/test-go.sh`、`TestApiClient`、canonical E2E与受控fixture入口。若标准总入口的共享Go库、环境继承或证据计数与当前spec不符，采用任务私有编排适配；它不能更改产品断言、超时、跳过或枚举。

私有编排的输入包括 source fingerprint、工具位置和SHA、命名测试清单、执行脚本SHA与配置摘要、资源allocation。首次SQL/测试之前独立review确认这些输入；旧私有runner不自带放行证明时，本次另生成review/clearance。

Go JSON parser同时处理build events和test events，核对每个包的完整命名terminal，记录失败/跳过/缺失/重复以及stderr。浏览器collector和各lane的报告按file+title+project唯一身份对账。

## 配置契约

legacy、password、device、password admin/managed、namespace-only、host-enabled分别有清楚配置和synthetic actor集合。password必须single limiter、device/Cloud关闭；实际问卷/改密/enrollment不能用伪造profile标识绕过。

runtime-output的历史task身份守卫保持原样；专门新subrun若选择真实归档task，完整allocation仍绑定本次源码、数据库、服务、状态/上传根和摘要，并在本任务的结果账本明确关联。不能把它的旧报告或旧配置当作当前结果。

## 失败、恢复与终审

保存完整原始失败、资源状态和来源；诊断/更正环境使用新run ID和新配置，涉及执行过fixture的Go包重新分配数据库。不得放宽产品守卫或把更小的诊断pass替换为原完整用例pass。

最终同时检查命令结果、命名inventory完整性、HEAD/跟踪源码/生成文件例外、workspace dependency实际来源、build/tool/asset摘要、listener/数据库/进程归属和自有资源关闭。仅记录允许的Next生成声明差异；其他来源变化会使对应验收失效。

完整macOS Main的终审还必须验证原协议默认应用URL和既有注册集合恢复；仅进程退出、只按bundle ID设置默认、保留临时应用或仅比较userData都不足以证明恢复。

清理仅针对allocation中证明归属的资源。失败数据库与证据保留，不强制断开不明连接，不破坏历史QA目录；原生平台/模型缺口保留在报告，不能从单元或模拟结果推断。
