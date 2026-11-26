# V0.8.8 最终 Windows 原生验收独立审查

日期：2026-11-26。**PASS：run45710224239原生全scope通过，绑定源码d2c9240eaf9f768446faba93cac625bd63e9cf53与其确切构件。** [CI run](https://github.com/yangshangwei/multica-0.4.37/actions/runs/45710224239)。本结论不覆盖签名信任、客户内网、knowledge ON或差分更新。

证据根：`/Volumes/artisan/code/2026/multica-0.4.37/.artifacts/release-v0.8.8/desktop-acceptance/run-45710224239/`。独立读取run.json、expected-source、六份原始业务/生命周期/协作/updater回执、Go/Vitest事件与PNG，核对实际断言代码；没有重复下载、安装或重型测试。

## 来源与本地绑定

run completed/success、headSha与expected-source和全部候选回执一致。两个Accept步骤、两个always服务cleanup、accepted-candidate上传均success。

本地已下载的原安装器独立重算：181985338字节，SHA-256 `b41f87075bdfb6f210e89319d30f398f5aeae88c2e1106708567c43260754d1d`，与真实Windows lifecycle installer回执相同。版本0.8.8；实际旧基线为 `0.5.2-rc.11.1`，SHA `f0b5a5eeded51fa7d1336bd2b46975a0c9ddc6d2292cebba457fd0b6e090fd47`，不是最近服务端交付v0.6.7。

native-binding.json中六份report摘要均独立重算相同。两个协作phase各7项artifact map一致；boot/restart/status中ASAR、exe、bundled CLI与artifact map匹配，业务verify的exe也相同。实际installed exe SHA `ea16341716885541ef51b59a54f715a79edd9208a401bc77784aa369e4688145`。本地PE/ASAR/CLI提取、完整元数据/HTTP摘要由交付代理检查，本报告末尾再补其确切结果，不把原生回执等同于已完成全部本地物料审计。

## 生命周期五步

install_previous→upgrade_candidate→uninstall→reinstall_candidate→final_uninstall全部passed。candidate两次CLI运行报告windows/amd64、0.8.8、commit前缀d2c9240e及Go1.26.11。首次卸载没有留下payload/注册；final_uninstall保留测试foreign协议handler，再由exact command+owner双匹配清理。protocol_fixture记录prior=owned_candidate、owned command已核验、备份已保存，说明实际GUI注册与新fixture路径均执行。

lifecycle自身update_feed_tested=false是该脚本的范围标记；独立下一updater step确实执行并passed，不应将这个局部false误读为整个run未测更新。安装器和桌面均NotSigned，require_signed=false，未建立Authenticode/SmartScreen/离线证书信任。

## 业务九项

verify原回执包含9项实际case：已安装Main/preload/renderer；免重登录保留会话；安装身份/workspace binding；管理员可观察安装；配置/身份/workspace/已完成任务保留；原pending task只执行一次；新candidate结果返回；管理员取消确认及provider退出；WS重连保留同process/identity。

独立比较baseline与verify安装ID、daemon namespace ID、公钥摘要相同；原pending task ID完全相同且由queued→completed；已完成任务保留。实际fixture源码在发布case前等待operation confirmed+succeeded并验证provider PID退出；因此报告taskLastStates中取消任务的早先running轮询快照不能当成最终取消状态，取消结论来自后续专门断言和operation ID。WS disconnect1、daemon连接数增加、same PID/identity断言通过。

业务两阶段cleanup均owned application/observed daemons exited；verify有ownedProfileRemovedAfterVerification=true，状态和影子环境只属于临时runner。Provider为确定性合成程序，没有真实模型账户或商业额度验证。

## 两次完整 installed collaboration

upgrade_candidate及reinstall_candidate都为完整原knowledge-OFF旅程，各真实password form登录200，Stop/暂停状态保留的App重启/显式Resume/最终历史与截图全部到达；不是只跑smoke或跳过案例。

每phase独立解析：

- 3个不同provider run、3条version2/sealed/proof_acknowledged/host_exit_observed账本；每run1次authorized/process_observed/terminal_accepted launch，windows_job/tracked_scope_complete退出范围。
- daemon exit0、errors[]、claims3、active_tasks0、available_slots1、invalid_capacity=false；每phase18条HTTP事实，claim/permit/termination/outcome/proof顺序正确，Stop第二run的outcome403且其他两run200。
- 唯一callback error绑定第二个stopped run：ECONNRESET、after_stop_intent/idle_at_error/stop_confirmed全true；visit未release但已closed，原始facts仍保留。没有将非Stop socket错误清除。
- 4条command history为3succeeded+1stopped；restart userData/ASAR保持，managed daemon running，暂停lane未因App重启自动恢复。19个命令、84条timeline无失败/timeout/截断。timeline在native.close前取得，成功回执在finally清理完成后写出，不声称时间线包含close事件。
- discovery source/hash绑定且unexpected_task_execution=false；两个phase保持同发现器字节，服务探针source/path/start与三个HTTP200通过。
- web_errors及packaged_status.errors空。两张截图/phase的本地PNG实际SHA匹配报告，共4张；摘要核验不等于视觉审图。

## 真实自动更新

独立updater回执passed，初始版本0.5.2-rc.11.1，GUI实际启动、check ok/available/latest0.8.8、下载完成、quitAndInstall替换payload并重新启动读取0.8.8，automaticUpdates偏好、updateUrl和localStorage history marker保留，home/安装清理均passed。

原始requests和processErrors明确显示旧版0.5.2-rc.11.1 blockmap请求404，Electron updater回退完整下载；随后GET0.8.8 exe无Range，进度至100并安装成功。**接受full-download fallback更新通过；差分下载未被证明，不能将blockmap尝试说成差分PASS。** 该feed由测试进程本地提供，业务API并非此step依赖，/api/config等404不影响已验证更新契约。客户内网跨主机传输另需下载服务HTTP验收。

## 其他原生测试与限制

Go原始JSONL独立重解析：managed测试24命名run/pass、无fail/skip、packagepass；identity压力20次同名run/pass、无fail/skip。Desktop锁主套件19PASS/0pending；三次筛选重复各1PASS+4未选pending，不能把pending计作已执行通过。

本次只证明Windows x64一次性GitHub runner和上述基线/配置。未签名、客户硬件/内网、默认knowledge ON、Linux Namespace、付费provider、差分更新和SmartScreen/证书信任都不在通过范围。以前所有失败run保留，不能用最终success回写历史为pass。

## 原回执摘要

| 文件 | SHA-256 |
| --- | --- |
| `multica-windows-lifecycle.json` | `a38a7fb17f2ef164fb9650c8299da3caf277c9e3d512be57b37b89c296654ab2` |
| `multica-windows-update.json` | `2fdef965f11a40914d9becb8c69cc4e1419f5f002ab5c8e9343f0ef3eb4e9636` |
| `multica-business-baseline.json` | `f63d354c9d95d359e0574ff611b1b90360109be70be03e42a9092e6fcbe6430a` |
| `multica-business-verify.json` | `10984ae84e98953e97ae6ce19552e4c42350f08a6c9adb56f21693d6db413b9b` |
| `multica-windows-collaboration-upgrade_candidate.json` | `449e4fd7787a5b8bd146d06e3e486cd32b912d64bc6523ac5ce77b92c73ba6df` |
| `multica-windows-collaboration-reinstall_candidate.json` | `51d658b97e5975807ff186033fc77f71446685846e3b1c54a08165f0476a3b3a` |

本轮只读证据/源码和本地摘要重算；未重跑Windows流程或全仓lint/typecheck，未改产品或交付文件。


## 本地确切构件与下载服务补充审查

**PASS：本地原始candidate、解包payload与原生回执绑定，以及隔离Nginx HTTP发布/重启/保留验收均通过。** 输入为上述同run下载的原始字节，没有重新构建替代；首次解包/文本比较/Docker挂载失败仍保留。

### 原始payload绑定

独立读取inspect-windows-payload.mjs完整检查逻辑和payload-verification.json，重算installer、Multica.exe、app.asar、bundled CLI和blockmap全部原始长度/SHA，匹配报告及两native phase。exe与CLI PE实际Machine0x8664，NSIS loader0x14c仅表示installer启动器，不能误作32位应用。实际版本资源0.8.8；证书表缺失与Windows NotSigned一致，未冒称签名信任。

CLI `go version -m`由本审查独立读取并逐字等于保留记录：main.version0.8.8、完整vcs.revision=d2c9240eaf9f768446faba93cac625bd63e9cf53、vcs.modified=false、GOOSwindows/GOARCHamd64/CGO0。真实SHA为43578ef72a867b3ff86d649a6f3d9b43cf6a86fa5d532ab7523ea7f916b8dffe。

ASAR原始SHA `8b966f97e3c72234d001df1be003f7a68628e31d65cbe16dae9cc7e7f56d944d`，47870个唯一文件、511137480总校验字节、47892块。实施harness逐文件/块核对ASAR SHA256 integrity，并校验exe内ASAR-header integrity；本审查审查循环/断言、重算整体ASAR及确认inventory数量/唯一性/总字节，没有重复47892次重型遍历。未发现私有状态文件；此目录结构检查不宣称能证明任意内容的所有秘密不存在。

latest.yml版本、文件名、181985338大小及SHA512与原installer一致。blockmap原字节SHA `03381d3ad1876b9719466a4cd498482fb82006f3f8a6b877e362b6c12d884b4e`；内部sizes覆盖完整installer、格式version2。harness明确perBlockChecksumVerified=false，因此只证明结构/字节来源，不宣传blockmap分块算法或差分下载已通过。

### CRLF与挂载调整

首次payload-audit.log失败保留。源文本比较发现Windows checkout行尾差异：两locale desktop.json以及LICENSE/NOTICE共4文件。独立从ASAR/资源读取原字节、按d2c Git对象取源，分别重算报告的两份原始hash，确认只把CRLF变LF后逐字相同。没有规范化ASAR/EXE/CLI整体或修改候选字节；ASAR内部integrity与native绑定继续用原始Windows字节。不能把“2locale”概括成仅两份文件，最终审计表还包括LICENSE/NOTICE。

首次http-xw_1f3yw start失败、cleanup成功记录保留，原因Docker不共享Codex worktree目录。后续http-4jhrbutc将5个实际lifecycle输入复制至共享/Volumes私有目录；本审查逐文件SHA与d2c Git对象相同：Compose、desktop-updates.sh、nginx.conf、download.html、download-config.example.json。运行原脚本/Compose，未改Nginx或绕过发布/校验逻辑。

### 下载源实际HTTP与持久配置

实际先collect/publish0.6.7，再collect/publish当前candidate，start原服务、verify、真实restart、再次verify，全部命令exit0。两份原verify stdout逐字段等于verification.initial/restarted；均latest no-cache、installer HEAD200、Range206/1024字节、完整181985338下载SHA512一致。

download.json与collector输出、latest/installer版本文件大小一致，公开platformwindows/archx64。blockmap有HEAD/Range及完整下载；通用verify标checksumVerified=false因为YAML无其预期摘要，但harness另实际下载并用SHA256比对原collected blockmap，source_hash_verified=true。不能只读通用false误称未做来源对照，也不能改它为metadata内建验证。

本审查重算collected与persistent四构件（包括download.json/latest.yml）原字节一致；旧0.6.7安装器/blockmap摘要保留，管理员download-config.json的业务URL及operatorSetting精确不变。页面字节与冻结源一致。Nginx镜像通过明确digest固定为linux/amd64，非生产更新源。

cleanup原日志成功，独立只读查询本project容器/网络为空；持久目录保留用于审计。浏览器交互仍由交付代理另验，本HTTP验收不替代UI点击或客户内网网络。

### 本地审计回执摘要

| 文件 | SHA-256 |
| --- | --- |
| `payload-verification.json` | `ee2b9f501d9e0aee54a2647c0aead7a09537877259f0281333221251bd5d9336` |
| `asar-inventory.json` | `45b6348079f9ccd5ae1503729bec77862f315c0028eb08a4633a3038b2bde6b4` |
| `cli-go-build-info.txt` | `5f9aa82b249cf7dea1983acfc5f2e7a5995f0f42615c4c6b891c0a59aeb001d2` |
| `HTTP verification.json` | `dbaa82b0917b2e91dfd7c79eeeb642b2a7752e09f6f3c146c7e851aaafef1868` |

本轮无重复download/安装/服务启动，仅读取harness及保留原始证据、文件哈希和当前资源状态。原生+本地精确物料门槛通过不自动代表最终Linux正式包/发布CI门槛通过。


## 下载页浏览器补充

交付代理完成browser-verification.json后，本审查读取完整check-download-browser.mjs、原首次失败记录并重算截图SHA。接受此限定范围PASS：同一Nginx实际页面，全部外origin请求被阻断且实际尝试数0；版本/精确installer链接/保留业务URL、FAQ Enter展开收起、模拟clipboard拒绝与execCommand失败后的真实“未能自动复制”提示及选中文本均有行为断言。未改变产品页面。

首次harness误等aria-disabled=false而超时，原页面在available时删除该属性；改用实际data-state=available及属性缺失断言，初始失败与cleanup成功记录保留。此为fixture契约纠正，非产品失败。成功浏览器场景为macOS Headless Chromium，不代表客户内网浏览器/真实OS剪贴板权限。

截图download-page.png独立SHA `83ce52594ba1c3ae66e8a06312585d4cf98e0881f3f602b829ef7ff23e1302ad`与报告相同；browser-verification.json SHA `37eabd2c183f96a414aa765801c4986b3ebc977d4a63d90356ad6b06f5f6c492`。未重跑浏览器或重启服务。
