# Windows Node executable 解析独立审查

日期：2026-11-26。结论：**两文件修复可随统一候选提交，实际Windows恢复须下一原生run确认。** 未发现阻断提交的问题。

## 根因与边界

直接读取保留的 `checks/windows-services-failed.log`：Prepare isolated installed collaboration services 在生产Web构建后报 `Cannot convert System.Object[] to System.String required by parameter FilePath`。原 `(Get-Command node -CommandType Application).Source` 可投影多个路径；Start-Process需要一个字符串。实现报告保留原失败run45663530640，而非重写为成功。

`Get-NodeApplication` 明确枚举Application，取PowerShell执行优先级中的第一项，验证返回string、绝对路径且为存在的文件。返回值同时用于Start-Process与原web-private所有权记录，后续path/start-time所有权判断保持一致。没有以join/stringify掩盖多个路径、未改任务状态目录约束或清理权限，没有改变Web构建/健康门槛，也未修改产品运行代码。

## 回归真实性

沿用既有PowerShell AST提取方法，只执行实际resolver函数，避免测试启动服务。将runner自带pwsh.exe复制到两个含空格目录中命名node.exe，实际Get-Command发现两项并检查首项；交换PATH后检查选择反转；PATH仅保留无node目录时必须抛CommandNotFoundException。若缺失命令未抛出，测试自己的throw不会被指定catch吞掉。finally恢复原PATH及runner变量并清理自有临时目录。原PE/文件名/拒绝持久runner测试完整保留。

复制的exe只用于命令发现，不尝试伪装Node执行；验证的是解析边界，不冒称Node进程启动或版本验证。工作流使用PowerShell7的pwsh，测试PSHOME/pwsh.exe来源与目标环境一致。

## 独立验证

`node --test apps/desktop/scripts/windows-collaboration-workflow.node-test.mjs scripts/windows-candidate-version.test.mjs scripts/agent-critical-path-workflow.test.mjs`：**11 PASS / 0 FAIL / 0 SKIP**。这组测试确认现有流程契约，不能代替新增PowerShell行为回归。

`git diff --check`：PASS。本机 `pwsh` 与 `powershell` 均不可用，未运行PowerShell AST/行为测试，也未启动真实Windows Web。必须在下一原生workflow执行新增fixture并验证服务启动及最终cleanup，不能把本机Node通过说成整个Windows步骤通过。未运行全仓lint/typecheck或重型构建。

## 文件摘要

| 文件 | SHA-256 |
| --- | --- |
| `apps/desktop/scripts/windows-collaboration-server.ps1` | `56f07408d5ffd321f1e00066b24f98fa3042a6240f7ac646d63d721ac1c57938` |
| `apps/desktop/scripts/accept-windows-installer.test.ps1` | `79562755d76012a62ac6b08fef79cf98721d226a9c1617b2472dcd38d9adad74` |

只新增本审查报告，未修改产品、测试或工作流。
