# Windows installed journey 后续边界只读前瞻审查

日期：2026-11-26。审查基线d746db2a5544f78113e2a751ed6c585fc08b693e；正在运行的native45681903824未作为已通过证据。范围为完整installed-main.mjs/ts、host-installed.spec.ts及必要的直接调用实现，从init→login→Stop→restart→Resume→screenshots/evidence。未重复renderer URL已审修改，没有改产品或运行重型测试。

## 一个高置信阻塞：验收home与Node/Go home分裂

`InstalledMainClient` native构造器（installed-main.ts:107）只向控制器传MULTICA_PACKAGED_ACCEPTANCE_HOME，保留父process.env的HOME/USERPROFILE。控制器mjs:413启动Electron也继承process.env，仅再次传acceptance marker/home。

产品 `configurePackagedAcceptanceHome`只调用Electron app.setPath(home,userData)。因此运行配置/managed-session使用的 `app.getPath("home")` 指向phase内home，但 `daemon-profile.ts:50`使用Node `os.homedir()`。Windows Node明确优先USERPROFILE；当前driver没有把它改成phase内home。产品desktopSpawnEnv继承process.env、只删除MULTICA_SERVER_URL；Go CLI的os.UserHomeDir也继续选择原runner home。

后果不是仅路径显示差异：profile config/.desktop-user-id/daemon.pid位于runner用户目录，而managed-session的管理文件与验收verifyOwnedManagedDaemon预期在phase home。login后navigate/metadata、managed handoff或restart检查会读不同根，还可能在runner真实profile遗留状态。已有business验收刻意使用runner home，不能当作此独立phase根正确的证明。

### 无副作用真实函数复现

使用仓库现有esbuild把实际 `packaged-acceptance-home.ts` 与 `daemon-profile.ts` 编译加载至VM；传真实Windows path.win32语义，os.homedir按官方Windows USERPROFILE契约提供。仅替换平台环境，不重新实现被测函数。输入：

- USERPROFILE/HOME=`C:\Users\runneradmin`
- MULTICA_PACKAGED_ACCEPTANCE_HOME=`D:\a\_temp\multica-collaboration\upgrade_candidate\home`

实际helper产生Electron home=D:\...\home，却完全不变更USERPROFILE；实际profileDir输出C:\Users\runneradmin\.multica\profiles\desktop-127.0.0.1-23456，fixture预期为D:\...\home\.multica\profiles\desktop-127.0.0.1-23456。两者必不等。仅在局部childEnv把HOME和USERPROFILE都设为已校验acceptanceHome后，实际profileDir与fixture预期精确相等，父env仍不变。

该复现确认源码根分裂，不是Windows Electron已启动后的现场复现，不追认尚运行run失败。依据：[Node os.homedir](https://nodejs.org/api/os.html#oshomedir)、[Electron app.setPath](https://github.com/electron/electron/blob/main/docs/api/app.md#appsetpathname-path)。

### 最小修复边界

在TS native构造器spawn局部env加入HOME/USERPROFILE=已由nativeAcceptanceHome校验的acceptanceHome；mjs原electron.launch及产品desktopSpawnEnv会继承。若需要保障mjs独立native入口，复用同一个已验证的纯env投影，但不改主机/父process.env、不改产品home/profile规则、不删或放宽verifyOwnedManagedDaemon断言。

应补真实child os.homedir回归（Windows USERPROFILE、POSIX HOME）、父环境不变、非法home拒绝、native spawn传递及容器分支不变；修复后完整原生login/restart仍必须验收。已把证据及建议交主会话/实现者。

## 其余边界：未发现可静态确定的新阻塞

- CLI：真实cmd_version.go使用encoding/json输出对象，fixture JSON.parse全stdout，没有按Unix换行切文本；本机CRLF+尾换行小控制可解析。二进制路径来自nativeLayout.cli（Windows.exe），spawnSync参数数组、status0和版本/hash保留。Linux dpkg/硬编码/root支路被native条件隔离。
- profile名：fixture与deriveProfileName均使用URL.host冒号转连字符及小写，路径内部用平台path.join；除上述home根来源外未见不同规则。
- 进程：native控制器spawn在Windows不detached；close只taskkill该owned PID树，负pid/SIGTERM仅POSIX。Host.close避开Windows SIGCONT；完整OFF旅程不调用freeze/thaw。Windows结算scope接受windows_job/direct_process，未强求POSIX process_group。
- 协议：控制器readline与JSONL解析能容忍CRLF，PID及user sidecar使用trim。login经真实表单/200/原user ID；Stop和Resume保留epoch/idempotency/target/历史断言，不建议为Windows放宽。
- restart：app.close→原launch→fresh GET/无登录表单→metadata，autoStop:false为原契约；home分裂修复后仍须验证原身份/daemon及暂停lane未自动恢复，不能仅凭静态检查宣称通过。
- screenshot/evidence：BrowserWindow.webContents.capturePage产生PNG base64，spec用testInfo.outputPath和writeFileSync写图片，路径由平台工具生成，未见raw URL当Windows路径；证据附件的名字与driver期待一致。finally会尝试daemon/native cleanup并保留证据，没有发现需要添加skip或删断言的静态理由。
- GUI可见性、字体时序、权限/路径大小写的实际Electron表现及商业provider行为没有静态失败证据，未列为缺陷，也未修改预算。

## 验证与报告范围

本轮只读源码、官方API契约、真实函数VM小复现与JSON换行小控制；没有启动已安装Electron/daemon、没有访问账号/DB、没有重跑CI。无需全仓lint/typecheck，未运行也不宣称通过。只新增本报告。暂不扩大到产品功能重构；下一修复仍是验收fixture环境边界。
