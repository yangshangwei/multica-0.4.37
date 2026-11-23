# 续验计划（2026-11-02）

继续既有 `in_progress` 任务和已批准计划。候选仍为 `7af9ab9905c98c77bb091223ebe687c639475bb4`，不创建新任务、不重跑 439 项已完成库存、不改产品代码或测试断言、不提交或发布。

## 证据边界

- 旧报告、失败与复验全部保留；`evidence/resume-preflight-1/prior-evidence-sha256.json` 记录续验前文本证据与摘要指纹。
- 新增 run 使用 `resume-*` 名称、新专库、独立 HOME。旧 private launcher 不启动。
- `coverage.csv` 的 439 项结论不因新条件项改写；新增条件检查单列。Go 最新视图只替换原来跳过的精确身份，不将专项结果写成整包重跑成功。
- 三个 Trellis check 子代理在开始工具工作前遇到 429，主会话接手可独立完成的执行；无法获得独立复核时据实说明。

## 本轮可执行工作

1. 核实提交、隔离树、模板库、连接预算、当前工具与构件。模板库空业务表，751 条 migration ledger，保留原配置。
2. 新库 `multica_func7af9_resume_perf_1`，只执行迭代 P95、迭代禁用 P95、分诊规模三项 opt-in；按原冻结门槛判断，性能测量不用 race。
3. 新库 `multica_func7af9_resume_wire_live_1`，只执行 Go triage wire 导出及公开 GitHub skills 导入测试，启用 race；真实 Go 输出再送入当前 TypeScript schemas。
4. 新库 `multica_func7af9_resume_linux_1`，实际 Linux arm64 容器内执行六项 RoutesLinux；另在无外网容器执行 daemon slot/runner 一项。静态编译仅为执行准备，不计平台验收；强制要求真实 namespace，不接受 skip。
5. 核对 Windows/iOS/native Linux runner 和候选安装器，核验 Mac 签名元数据。缺少真实前置就登记 BLOCKED 和具体恢复步骤。
6. 重新对账条件矩阵、Go 新增结果、缺陷与恢复入口，核对旧证据未被改写，确认自有子进程退出、新库无残留连接。
7. 继续用户本次明确授权的 Linux ARM64 DEB 包管理器验证：核验已完成的 `evidence/resume-linux-deb-1/build.json` 与新包 SHA-256，沿用这份当前候选包；在新建、无网络、仅只读挂载安装器的容器中执行旧版 → 候选版 `dpkg -i`、独立新装和移除/重装检查。核对包状态、载荷、桌面入口/图标/协议及合成用户状态保留，保留每步原始日志。该结果仅覆盖包管理器子门，原生桌面会话继续开放。

本次两个原生 Trellis 子代理同样在开始工具工作前返回 429。主会话接手执行和证据检查，不将自身复核写成独立审阅。

## 后续推进（用户要求列计划并按计划执行）

执行顺序与验收条件已固定在 .omx/plans/branch-functional-regression-next-20261102.md：

1. 分析 QA-PERF-001 原始样本和结转事务/SQL/锁等待，必要时用新专库做受控诊断；原 FAIL 不改。
2. 核验 macOS 旧版发行构件与当前候选，在任务专用安装目录和隔离 HOME/profile 推进真实 Main 安装升级及合成状态保留；依实际覆盖拆分子门。
3. 整理 Linux 原生桌面、Windows、iOS、签名与真实 Provider 的准确执行入口、前置和证据要求，不执行远端发布/付费调用。
4. 复核证据与清理，更新报告、条件矩阵和续接说明；仍有未关闭门时保持 in_progress，不提交或归档。

本计划的可本地执行部分现已完成：一次性能归因、Mac文件/真实Main升级及合成状态/正常密码界面补证、外部平台入口和独立复核。原P95 FAIL、认证迁移/自动更新/平台信任门保持开放。复核见 research/next-evidence-review-1.md 的runtime2补充，清理见 evidence/next-cleanup-2/run.json。

## 持续边界

- 不安装完整 Xcode、不请求签名凭据、不访问商业账号、不调用商业模型，也不挪用旧会话额度。
- `pg_bigm` 缺失、Windows 条件和明确未实现断言仍需独立条件；不能因已有整包成功而记 PASS。
- 性能阈值失败保留原样；共享机器/数据库负载的影响属于测量局限，不能用它将失败改判通过。
- 只有已确认的实际 native/install/upgrade 证据才能关闭相应门；容器目录包执行、编译和源码检查不替代这些门。
