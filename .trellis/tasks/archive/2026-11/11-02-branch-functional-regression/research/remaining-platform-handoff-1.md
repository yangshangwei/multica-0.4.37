# 剩余平台验收执行入口

候选固定为 7af9ab9905c98c77bb091223ebe687c639475bb4，候选版本 0.6.8-rc.20261102。本文是后续执行入口，不是平台通过证据；本次没有远程派发、签名、公证或商业账号调用。

## Windows x64

入口为 .github/workflows/desktop-smoke.yml 的 windows-acceptance job。现有 apps/desktop/scripts/accept-windows-installer.ps1 明确只允许 disposable GitHub-hosted Windows x64；普通个人机器或持久 VM 即使是 Windows 也不满足该脚本边界，不能伪造其环境变量。

下列命令仅在后续远程执行获授权、对应远端分支 HEAD 已核对为固定候选时运行：

    gh workflow run desktop-smoke.yml --ref codex/cross-workspace-collaboration \
      -f windows_acceptance=true \
      -f windows_business_acceptance=true \
      -f windows_collaboration_acceptance=true \
      -f candidate_version=0.6.8-rc.20261102 \
      -f previous_tag=v0.5.1 \
      -f require_signed=false

必须显式传 candidate_version：工作流默认 RC 数字版本仍为 0.5.3，不能沿用它验收当前 0.6.8 候选。--ref 是分支/标签，不能把它当成 SHA 固定器；执行前后分别核对远端分支 SHA 和 run.head_sha，漂移结果不计当前候选。

业务/自动更新还依赖工作流 36996436086 中的 windows-x64-accepted-candidate：
multica-desktop-0.5.2-rc.11.1-windows-x64.exe，
SHA-256 f0b5a5eeded51fa7d1336bd2b46975a0c9ddc6d2292cebba457fd0b6e090fd47。
该构件能否继续下载必须先确认；不能用另一份同名安装器替换。上述 business=true 命令只下载这份功能基线；稳定 previous_tag（例如v0.5.1）的 EXE 与 .sha256 前置仅适用于 business=false 的另一分道，不是上述全开关命令的额外前置。

工作流已串接真实安装/升级/卸载、托管运行时、业务、协作和自动更新；要求保留 native JSON/JSONL、原始失败、截图、安装 EXE/ASAR/CLI 指纹和清理结果。BusinessAcceptance 与 CollaborationAcceptance 的登录账号/Provider 均用任务受控合成夹具；不代表商业账号验收。require_signed=false 的结果只能关闭运行时子门，正式 Authenticode/离线信任仍需已签名候选与 require_signed=true 的独立验证。

## 原生 Linux ARM64 桌面

已完成的 Debian 容器 dpkg 安装升级不重跑。下一门需要独立 Linux ARM64 VM/主机及实际 X11/Wayland 桌面和窗口管理器，且安装前后使用同一任务专用用户/服务身份。

候选 DEB 位于任务私有 resume-linux-deb-1/package/，SHA-256：
44f3a4c5f74e91205bea4d398a81ed737efa49d9062a9f4bc4c2234c588fb01a。
升级基线 DEB SHA-256：
7e23d62da6189d128bb08dd2caa011a8ca61ea6ad5256d346c331ec794b1c346。

在实际目标 runner 先记录：

    uname -a
    dpkg --print-architecture
    printf '%s\n' "$XDG_SESSION_TYPE" "$XDG_CURRENT_DESKTOP"

要求 architecture=arm64；完整验证安装包 SHA-256 与 dpkg-deb 控制信息后，用真实 dpkg 安装基线、启动并建立合成账号/任务状态，退出，安装候选后再启动同一路径。同一任务专库的服务端认证方式在升级前后保持一致，自动执行/更新使用显式受控配置。

实际桌面断言：/usr/bin/multica-desktop 对应候选；窗口 WM_CLASS 与 StartupWMClass=Multica 一致；图标、任务栏/托盘、关闭行为、窗口焦点/弹出、multica:// 打开目标、登录/IPC、Stop→退出→重启保留暂停→显式Resume、原任务/配置保留。X11 可用现成 xprop 检查 WM_CLASS；协议用 xdg-mime query default x-scheme-handler/multica 核对，不把 .desktop 文本替代运行时归属。

当前 e2e/fixtures/collaboration-installed-layout.mjs 的原生控制器只接受 darwin/win32；Linux 路径是 Docker/Xvfb。不能设置 COLLABORATION_INSTALLED_MAIN_EXECUTABLE 就把该容器路径当作原生 Linux 自动化。使用真实桌面记录上述交互，或另行实现并审阅 Linux 原生控制器。缺 runner 时此门保持 BLOCKED。

## iOS 实际 UI

当前机器只有 CommandLineTools，simctl 不可用；不会安装完整 Xcode。具备完整 Xcode 的目标机器上先核对固定源码，使用任务命令级 DEVELOPER_DIR，不切换其他会话的全局 xcode-select：

    DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer xcodebuild -version
    DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer xcrun simctl list devices available
    DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer pnpm --filter @multica/mobile ios

真实设备使用同一 ios wrapper 的 --device 参数。apps/mobile/scripts/ios-run.sh 会依次 expo prebuild -p ios --no-install、expo run:ios；产物身份、bundle/version、目标设备/OS 和任务 API 配置必须记录。不得沿用未知生产 .env 或真实账号。

检查登录/退出/会话恢复、工作空间切换、任务状态/生命周期、邀请/角色撤权、通知与未读计数、深链和后台恢复。与同账号同专库 Web 对照计数/权限/枚举/数据 ID；特别核对 packages/core/inbox/queries.ts 与 apps/mobile/lib/inbox-display.ts 的去重与归档过滤。196 单元通过不替代这些 UI/语义断言。

## macOS 运行时与签名

文件安装替换和合成文件保留已通过，见 research/mac-upgrade-acceptance-1.md。旧版 dirty 构件只限定为已钉住的实际 ZIP，不推广到所有 0.6.5。

实际 Main 重启需隔离 HOME/userData，并处理应用启动时的系统协议注册：先记录 multica:// 当前精确默认应用路径和同 bundle 注册列表，仅启动任务副本，结束后注销新任务路径、恢复并复核原默认路径。当前预检默认为 /Applications/Multica.app；不得启动或替换用户安装。若系统恢复 API 实际要求 OS 同意或不能恢复，保留具体错误并停止后续协议变更。见 evidence/next-mac-protocol-preflight-1/。

Developer ID、Gatekeeper 与 stapled notarization 需要对应已签名发行包。当前两版只有 ad-hoc 签名；codesign 深度结构成功不能关闭信任门。未来验证 exact DMG/ZIP/app bytes、codesign --verify --deep --strict、stapler validate 和目标机实际安装信任；本次不读取签名私钥或提交公证。

## 商业 Provider

需明确账号、供应商/模型、调用次数与费用上限及授权范围，才执行真实安装账号的兼容/模型质量验收。当前商业调用为 0。受控 fake Provider 只证明参数、物理输入、权限与生命周期，不能替代付费账号或模型理解证据。

## 统一证据与清理

每门保存候选 SHA、构件/程序/ASAR/CLI 哈希、OS/架构、精确命令与退出码、原始框架事件、失败与重跑编号、截图/回执以及任务进程/端口/挂载/数据库连接清理。自动重试为 0，不放宽阈值。任何 FAIL/BLOCKED/NOT_RUN 都保留开放状态，完整任务不归档为发布通过。
