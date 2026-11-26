# Windows 两次完整协作PASS及protocol fixture独立审查

日期：2026-11-26。**两次完整knowledge-OFF协作PASS可在其精确来源范围内依赖；protocol两文件修复可提交并进入下一native。** 整个run45706259050仍为failure，最终foreign-handler卸载和自动更新尚未通过；不得将分项PASS说成整个Windows交付通过。

## 两次协作实际证据

唯一读取目录 `.artifacts/release-v0.8.8/desktop-acceptance/run-45706259050/evidence`。expected-source与run.json head均8f24a5397f935d888955c555def126e86f76561e。upgrade_candidate和reinstall_candidate两报告都status passed/error null、同完整sourceSHA；对应安装版本0.8.8、win32/x64、Electron39.8.7。

独立解析两份约460KiB的完整journey而非只看status：

- 7项构件摘要两phase完全一致。boot/restart/status ASAR摘要、installed exe摘要、bundled CLI摘要与外层artifact map一致；CLI版本与app版本一致，discovery source/hash相同且unexpected_task_execution=false。
- 每phase有3个不同provider run，两个phase共6个不同run。每phase3个sealed/version2/proof_acknowledged/host_exit_observed账本，每个恰好1次authorized provider launch、process_observed/terminal_accepted=true，cleanup为windows_job/tracked_scope_complete。
- daemon exit0/errors[]，claims3/invalid_capacity=false/available_slots1/active_tasks0；每phase18条HTTP事实覆盖claim→permit→termination→outcome→proof，全部非outcome响应200，受Stop第二run outcome403，另外两run outcome200，原终止/结果/证明顺序保持。
- 唯一callback error精确对应第二run，ECONNRESET、after_stop_intent/idle_at_error/stop_confirmed全true。该visit未release但closed，真实stopped_display同run forced_disposition=stopped；不清原error事实、不把历史fc9generic错误回填。
- 真实password form登录200；Stop/Resume回执200、target匹配、paused→active；restart沿用userData、daemon running且版本/ASAR不变；4条历史为3succeeded+1stopped。web_errors和packaged_status.errors为空。
- 每phase19个controller命令，保存的84条timeline无failed/timeout/write-failed/process-error/exit，未截断。此timeline快照在native.close前取得；成功回执写入发生在spec finally完成native.close后，不宣称timeline本身含最终close事件。
- 每phase两张PNG文件实际重算SHA与报告相同，共4张。截图哈希确认材料绑定，不等同视觉审图。
- API/Web source/path/start/present/alive全true，health/healthz/config全200/valid。

摘要：

| 文件/构件 | SHA-256 |
| --- | --- |
| upgrade_candidate report | 6928af1e5dbfa111731c9c1c0266bf7c4096c09cdc0cb7e9794e4b835bd9c77f |
| reinstall_candidate report | 37701a63094dedab3984176b0280ae57449338bca23a36e3abd9f10cad535eac |
| installed Multica.exe（两报告） | e374c0b8af00cf822c082b34fccd7b04dbd9fe9e4fab4a5a310d202373c18ca8 |
| app.asar（两报告） | 53b33d7356d44a0516871ba91eb9775f864f1dae26ba85278758afe9c413b3bc |
| bundled CLI（两报告） | 1577c1cad63bea921de9ba77f93847dd57d7994f56166ab8036730b0bf6c67df |
| installer（lifecycle回执） | 847c56921b60cf640d9d7fc5bafdbf19e25d34d21fe968ce4c3592979ddd234a |

制品摘要是源绑定runner执行时记录，两份独立phase及CLI元数据交叉一致；因run最终失败，accepted-candidate上传被跳过，本审查没有本地安装器原字节可重哈希，不冒称已经完成线上/本地资产摘要交付验收。

## 真实剩余失败

lifecycle回执：previous0.5.2-rc.11.1安装、upgrade0.8.8（business+collaboration）、首次uninstall、reinstall0.8.8协作均passed。最后错误为 `Protocol fixture refuses existing registration`，在设置foreign handler前主动拒绝，尚未执行最后卸载场景，不能称产品uninstaller故障。自动更新step和accepted-candidate上传都skipped，两个always服务cleanup和evidence上传成功。签名NotSigned，知识ON/真实商业账号/客户内网/证书信任不覆盖。

## Protocol修复边界

当前产品packaged Main通过setAsDefaultProtocolClient注册multica://，reinstall后启动GUI自然恢复当前安装的协议项。独立查看[Electron39.8.7 GetProtocolLaunchPath](https://raw.githubusercontent.com/electron/electron/v39.8.7/shell/browser/browser_win.cc)及[AddQuoteForArg](https://raw.githubusercontent.com/electron/electron/v39.8.7/shell/common/command_line_util_win.cc)：无额外args时以exe及唯一quoted %1组成command；本repo生产调用无额外args。

Test-OwnedProtocolCommand只允许当前installDirectory/Multica.exe exact quoted+"%1"，以及不含空白的同一exe bare+"%1"；拒绝非string、非法expectedpath、CR/LF/quote、额外参数、不同路径、错误slot。不是包含basename或前缀匹配，不接受任意Electronargs。

如果已有HKCU key，只有该命令完全匹配才reg.exe export至tasktestRoot私有备份；导出失败即停止。随后写taskforeigncommand并加随机MulticaAcceptanceOwner。原Uninstall-AndVerify仍检查payload删除、exactforeigncommand保留、无剩余Multica注册及Assert-NoExisting；只在command+owner双匹配时删测试fixture，finally也用同一guard。未知或被改变的handler保留，不无条件删除，不修改产品installer.nsh。

PowerShell回归用实际AST函数及唯一HKCU/Software随机测试key，覆盖missing、wrongowner、wrongcommand保留与exactcleanup；不在测试中碰multica://真实键。原PE/命令/进程/路径断言保留。

## 独立检查与下一步

既有workflow/candidate Node窄检查6PASS/0FAIL/0SKIP、git diff --check通过。本机无PowerShell，新增PS语法/真实注册表fixture未本机执行；下一native必须通过它们及完整final_uninstall/updater。当前两文件可提交，不需要再扩大注册表清理或放宽卸载断言。

新commit会产生新sourceSHA，下一候选必须重新原生构建并验收原始字节；旧8f24两完整协作PASS仍保留其精确来源，不移贴到新制品。只新增本审查报告，未改产品或实现，未读取私钥。

## 修复最终文件摘要

| 文件 | SHA-256 |
| --- | --- |
| `apps/desktop/scripts/accept-windows-installer.ps1` | `3337a09c783fe1fa592731c02f6aff74bef20bedada18da485a8f5b2595136d1` |
| `apps/desktop/scripts/accept-windows-installer.test.ps1` | `a1e1e1ddb5811de3f2d996edc7995431f10e1fb6e43efa31495c3f1d0c3d90d1` |
