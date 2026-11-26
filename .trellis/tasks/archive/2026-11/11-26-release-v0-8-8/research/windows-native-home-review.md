# Native home 边界修复独立审查

日期：2026-11-26。结论：**两文件可提交并从新SHA运行Windows；没有阻断提交发现。** 这是已静态实证的fixture环境缺陷修复，尚非Windows协作通过或d746历史失败的完整根因证明。

## 代码与回归

InstalledMainClient native构造器先调用nativeInstalledLayout与nativeAcceptanceHome校验，然后只在spawn局部env中追加HOME、USERPROFILE=acceptanceHome。父process.env不写入，容器支路不改；后续controller→electron.launch→desktopSpawnEnv/Go CLI原本已继承子进程env，无需产品改动或重复环境抽象。原package/hash/身份文件/daemon scope断言全部保留。

新增测试读取并执行实际native构造器spawn代码片段，沿用现有VM模式。仅把controller命令替换为真实Node probe，保留生产边界生成的options.env；probe导入实际daemon-profile.ts，读取os.homedir/profileDir，并再启动真实Node孙进程验证继承。包含空格/字面%20路径；父HOME/USERPROFILE不变；missing/relative/newline home通过原validator在spawn前拒绝，launch计数为1。没有直接mock homedir返回正确值或只断言实现文字。

本机macOS覆盖真实POSIX子孙进程并结构核对两种env字段，不能称已执行Windows USERPROFILE语义或已安装Electron。实现者保留先红后绿记录，本审查独立运行当前四套installed-main/layout/windows-config/workflow共**32 PASS /0 FAIL /0 SKIP**，syntax及git diff --check通过；不重新跑重型构建。

## 最新运行事实

独立读取run45681903824公开phase报告，source d746db2a5544f78113e2a751ed6c585fc08b693e；服务探针全部true/HTTP200，boot/runtime/login均存在，Stop/restart/Resume尚未达到。失败坐标mjs393是metadata包裹错误的位置，来自daemon state poll或verifyOwnedManagedDaemon身份文件检查；该公开报告不能分辨二者，不能仅凭静态home问题宣称原run已确认该根因。

## 必要的下一轮诊断建议

现有controller已把daemonFailureDiagnostic附在私有错误的固定单行marker后，不需要扩协议或公开原message。可在已有safe diagnostic helper中只解析已解析Playwright error.message/stack的精确 `Installed daemon diagnostic: `行，有界扫描/行长，最多一对象；JSON.parse后从零构造公开projection。

允许：固定expected/actual state枚举、profile/server匹配与authenticated/managed布尔、固定config/owner/pid/log/err_log文件exists/outside/read_failed布尔及capture_failed/capture_timed_out。缺失或未知值用null/unknown而非任意truthy。禁止：PID、路径、用户名、expectedProfile、config值、令牌、原message/stack、任意字段/附件复制。畸形/超长/错误marker拒绝并保留fixed status；新增synthetic秘密与marker前后任意文本负例。此建议已交实现者，最终诊断diff仍应另审后才能合入。

## 当前两文件摘要

| 文件 | SHA-256 |
| --- | --- |
| `e2e/fixtures/collaboration-installed-main.ts` | `bbcc4b97b73f7ff829b527853f85cdf943310248e9f89db8511d936cc1bea7ad` |
| `e2e/fixtures/collaboration-installed-main.node-test.mjs` | `7b394a05a6e5d3dfe0b7a6b5058a3bc6e3fb090e9258c047454ac0f481bd819d` |

本审查只新增报告，未改代码、全局环境或测试。全仓lint/typecheck和Windows原生安装未运行，未宣称通过。
