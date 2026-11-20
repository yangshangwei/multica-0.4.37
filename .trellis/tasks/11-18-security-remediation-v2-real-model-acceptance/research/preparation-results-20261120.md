# 无模型准备结果

- 证据类型：夹具/环境预检，非脚本平台全链路、非真实模型。
- source frozen inventory：16 个文件，`fixtures/freeze.sha256.json`。
- 本地基线：代码 2/2 通过；依赖 3 条中 2 过、1 个预期失败。输出在 `preflight-local-20261120.json`。
- 离线容器最终预检：`preflight-container-r3-20261120.json`。固定已有 Python 3.13 Linux arm64 镜像 `sha256:9d2e5553305c7c7b0097999bb17187c69b921ccd6bc9d40e4bb5ebe652c00285`，没有 pull。两个夹具的计数/退出符合预期；7 个独立角色目录的临时容器均证明自己的读写、其他角色未挂载、Docker socket 不在、无外部路由及合成外网地址连接失败。
- 先前失败保留：`preflight-raw.txt` 是 Docker 子命令 60 秒超时；`preflight-container-20261120.stderr.txt` 是负向主机文件探针把访问被拒误当探针异常。修正为访问被拒也是合格拒绝后，r3 通过。没有隐藏失败或改动夹具的预期。
- 初版超时留下的 Created 容器 ID `715a6da619347bdf0383e9719b1e241973dcb4e3e03071b6751f2eb7ab572571` 经后续精确 inspect 已不存在。更新脚本为每次 20 秒、有唯一名/task label，超时只清理精确 owned 名称；未清理其他容器或重启共享 Docker。
- JSON/Python 语法验证通过；Trellis context manifest 路径验证通过。超长 sibling design 从 manifest 改为本目录有界 source-map，避免 64 KiB 截断。
- 真实模型启动数 0，此前准备阶段没有运行智能体 CLI；后续原生边界探针在无网络容器内运行了本地 Codex help/schema/sandbox/app-server，不访问账号、不发 turn/start 或模型请求。

限制：最终通过的只是无网络临时容器。实际模型/API 网关、角色网络互访规则、生产 daemon 领取和原生 app-server、只读 reviewer 正负操作、私有注册与证据采集还需按 implement 步骤 1–3 完成。不能把 r3 结果写成真实验收环境已就绪。

## 独立复核后的补充

- R2：新增离线 `synthetic_scan.py`，D1 预期加入前后扫描输出和间接依赖明确对照。16 个当前输入已重新冻结，清单 sha256 `c6bf4f474c1b3bb405b78c549ad5bee6c32bae4eeff66ec0efcadff7eddf18d8`；先前 15 文件清单保留为 `freeze-v1-pre-review.sha256.json`。真实调用仍为 0，因此没有改写已执行试验的预期。
- scanner 控制在临时副本只换锁文件后输出 affected=false、indirect=[]；原始基线未改。两份报告分别注明合成与非模型修复。
- `runtime_guard.py`/`gateway.py`/`test_runtime_controls.py`：9/9 合成测试通过（r4），包括并发预留、未知前序、失败扣额停止、进程超时、注册身份、精确路由与模型、API 原始身份、无 CONNECT/redirect、请求计数和时间、完整 real 注册仍被拒绝。最初 r1 一条 socket 超时与 r2 定向重跑保留；r4 全套通过。
- 带 `agentintegration` 的 `ordinary_driver_probe_test.go` 通过 overlay 运行：actual daemon Claim 方法的假 HTTP 边界通过（1 PASS），真实入口无 opt-in 按约定跳过（1 SKIP），不运行任何供应商。没有产品文件修改。
- R3 仍阻断真实准入：原生 Codex 的 shell 若仍在有网容器执行，可以绕过离线测试包装；必须实现强制 tool broker 或 provider/tool OS 分离。准备版硬拒绝 real，不能把字段齐全或一个哈希当成实际隔离证明。
- watchdog 关闭本地连接/进程后，供应商远端生成和计费是否停止没有证明，usage 缺失保持 unknown。

## 原生机制补证

后续 no-model native sandbox/app-server探针见 `native-boundary-findings-20261120.md`：普通及显式加网工具被 managed network=false阻断，dangerFullAccess被拒；stdio MCP能够以合成身份调用固定 API，scope外请求被拒。此证据解决机制可行性，不替代真实 task-token broker与最终 daemon装配。启动过本地 Codex CLI/help/schema/sandbox/app-server进程；model turn=0、供应商请求=0、真实账号访问=0。
