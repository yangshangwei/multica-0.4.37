# Windows 服务生命周期与安全探针独立审查

日期：2026-11-26。结论：**最终实现可提交并启动新Windows原生验证，未发现剩余阻断提交的问题。** 启动时间JSON往返问题已由实现者修正并经本审查复核；这不是协作/安装通过声明。

## 已确认事实

直接读取run45673226507的完整日志与公开phase报告：source09fa55bf94f0e4a6d92bbaeb5cfca87c9dee2f3d，Playwright失败、expected0/unexpected1/skipped0；坐标指向installed-main.mjs:489的/api/config fetch，boot/runtime/login等里程碑false、transport存在。两条cleanup记录分别表明business和collaboration API在各自cleanup时已不存在；business此前同一步业务请求仍200，collaboration最后活动早于消费步骤。

这些事实支持将长活服务的启动移至消费它的PowerShell步骤，避免依赖跨step进程寿命，但没有证明Windows Job Object、runner清理、终端行为或basename kill是哪一种原因。没有通过增加重试/timeout或改业务断言来假造原流程通过。

## 生命周期调整

原Prepare步骤现在只安装浏览器。Accept Windows installer lifecycle中先启动collaboration服务，再启动business服务，随后运行原安装生命周期；这样立即写入MULTICA_COLLABORATION_STATE_DIR供当前进程消费，并把business环境最后恢复给业务测试。collaboration driver自己从其私有setup读取并覆盖环境，保持两套DB/API隔离。

全部原signature/version/PE/source/业务/完整OFF协作断言和1PASS0SKIP仍在；20/40分钟step与45/75分钟job门槛不变，未新增重试。原always清理分别保留，Web按PID/path/start、业务API按原记录路径进行自有对象检查，没有basename kill或停外部服务。

## 安全探针

PowerShell -Probe只读原有API/Web私有进程记录，以PID查进程，返回存在/存活/路径匹配/启动时间匹配及source匹配固定布尔。它在创建服务/Stop分支前return，不启动、重启或杀进程。

Node调用明确pwsh.exe -NoProfile -NonInteractive -File，不用shell拼接；source、API/Web所有权各项必须全true。HTTP probe仅访问已验证setup.api的health/healthz/api/config，redirect:error、每次5秒；health要求实际PID/commit，config要求密码/managed/deployment一致。公开投影只有status整数、valid布尔、允许错误名称与transport cause code，不复制地址、PID、部署ID、body、messages、环境或秘密。失败继续非零并进入service_probe诊断；不是把dead service归作skip。

## 提交前发现：时间值类型

实际record通过ConvertFrom-Json读取，ISO时间会默认转为DateTime，而当前比较左侧为ToString('o')字符串；PowerShell会按左侧类型转换右侧，导致文化格式字符串与ISO不相等。原新增PS fixture直接内存hashtable字符串，未覆盖真实JSON往返；Stop-OwnedWeb存在同类比较，需要同一修正以免存活服务cleanup被误拒绝。

建议：统一将字符串/DateTime/DateTimeOffset安全规范化为UTC瞬时值后比较，invalid应返回false；不要取消start校验。使用PS7.2兼容方法，不无意引入7.5-only DateKind。测试应经过ConvertTo-Json→ConvertFrom-Json真实roundtrip，再覆盖正确/错误start、路径不符、无进程。

依据：[Microsoft ConvertFrom-Json](https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell.utility/convertfrom-json?view=powershell-7.4) 的日期转换说明；[PowerShell比较运算符](https://learn.microsoft.com/powershell/module/microsoft.powershell.core/about/about_comparison_operators?view=powershell-7) 的右操作数转换规则。本机无PowerShell，该发现来自真实调用链和官方类型语义，不冒称原生复现。

## 自动更新步骤依赖判断

已完整阅读windows-update-acceptance.ps1与windows-update-probe.mjs。下一独立step的updater不读取MULTICA_BUSINESS_API或server-private；它在自身Node进程内createUpdateFeed，desktop.json的apiUrl/appUrl/wsUrl/updateUrl均指向该loopback feed，并设置autoStart:false。所测试的是安装后的updater检查、下载、quitAndInstall、重启版本/配置/history marker，与业务登录验收分离。

因此business API在上一步结束后消失**不是此updater的前置依赖阻塞**，不需要为猜测风险新增业务API跨step重启或改更新验收流程。feed和消费者已经在同一生命周期。现有404业务请求不等于updater失败，最终仍须实际原生run检验。

## 已运行验证

- 独立全workflow十个Node suites：50 PASS /0 FAIL /0 SKIP。
- 单独config/workflow/update-probe：16 PASS /0 FAIL /0 SKIP，包含真实loopback HTTP服务、foreign PID拒绝、停止listener错误白名单和无秘密投影。
- driver/services node --check、git diff --check：PASS。
- 新PowerShell函数测试目前只静态审阅，因本机无pwsh/powershell未执行。没有全仓lint/typecheck/重型构建或Windows进程运行；没有据本地Node通过宣称服务生命周期已修复。

仅写本审查报告，未修改实现。最终原生run必须证实同step进程仍活、source/path/start匹配、HTTP门槛通过、完整业务/协作/更新旅程与always清理成功。


## 发现已修正后的复核

实施者新增 `Test-ServiceStartTime`，接受实际ConvertFrom-Json产生的DateTime，或用InvariantCulture、精确o格式及RoundtripKind解析字符串；双方转UTC ticks严格相等。不合法类型/字符串返回false，不去掉身份检查、不使用PS7.5-only DateKind。实际Probe和Stop-OwnedWeb共用此函数，因此新探针与清理的时间边界保持一致。

PowerShell fixture现在执行真实ConvertTo-Json→ConvertFrom-Json往返，验证当前进程完整proof、差1tick拒绝、malformed拒绝、foreign path拒绝及无pid全false；另保留原string输入用例。该结构可证明测试目标已覆盖原缺口，但本机无PS，**实际fixture通过仍须下一Windows run**。已修正产品/测试文件由实施者维护，本审查只读，未代改。

独立最终 `git diff --check`、driver/services `node --check`通过；先前全10个Node suites50PASS仍适用，之后变化仅PowerShell时间比较和对应fixture，没有改Node。正式原生结果出来前，不得把此修复叫作进程跨step消失的根因已证实，或Windows协作已恢复。

## 最终文件摘要

| 文件 | SHA-256 |
| --- | --- |
| `.github/workflows/desktop-smoke.yml` | `19aad0bc50b5b667cc745f554369b1040dabf6e9369b41a27d4a59558d440a7e` |
| `apps/desktop/scripts/accept-windows-installer.test.ps1` | `e05538cca32a743e83f32e6d656e0f5ed523d807f5f50f3cf4f8ad5da2ac8fa5` |
| `apps/desktop/scripts/windows-business-server.ps1` | `694f03ca8d1431a356758619a81d307104782c6dc867486dc2cf3ef22c7266c8` |
| `apps/desktop/scripts/windows-collaboration-config.node-test.mjs` | `93dff4534e6a81d1bb2bc1f45dca38c464b75244e3f7c14cc50a4d78ea5cf706` |
| `apps/desktop/scripts/windows-collaboration-diagnostics.mjs` | `209354168a015a8e2e4a26ed4283b4d227d00bcb829e001a1cc49f65007c3d8a` |
| `apps/desktop/scripts/windows-collaboration-server.ps1` | `3e0ad99a196d582201ffaa0546be356aa1010a0f77d8d531855894446dd1c4de` |
| `apps/desktop/scripts/windows-collaboration-workflow.node-test.mjs` | `c980c73a4f3900957129002f4a315b18a6aaace332fdf7a65584e38e7c678ba7` |
| `apps/desktop/scripts/windows-collaboration.mjs` | `7c2f24eb687ac577d47d664a94dcbfbacdd693f09a485709a9da7a989632757b` |
| `apps/desktop/scripts/windows-collaboration-services.mjs` | `e759b404688c29201e960e4224fcfd604b6b89799f718e3a1f37b2a2a96c68cc` |
