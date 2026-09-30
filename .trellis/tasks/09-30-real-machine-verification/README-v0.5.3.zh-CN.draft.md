# Multica V0.5.3 内网交付

构建日期：〈TBD〉（北京时间）。源码固定为 `〈TBD〉`，本地标签 `v0.5.3`。本次生成本地安装包，未推送标签、未上传 GitHub Release、未部署业务服务器。

V0.5.3 在桌面端引入 **Windows / Linux 关闭行为选择**：关闭主窗口时可以让 Multica 完全退出、最小化到系统托盘，或每次都询问，并支持记住选择。macOS 行为保持不变。细节见 `.trellis/spec/desktop/frontend/close-behavior.md`。

> **本次仅升级桌面端**。服务端、Web、CLI 与 V0.5.2 完全一致；不发布 server 升级包；不迁移数据库；不增加必填环境变量。

## 交付文件

| 文件 | 用途 |
| --- | --- |
| `multica-desktop-0.5.3-windows-x64.exe` | Windows x64 桌面安装器（64 位，不适用于 32 位 Windows） |
| `multica-desktop-0.5.3-windows-x64.exe.blockmap`、`latest.yml` | 内网下载服务的 x64 更新文件 |
| `multica-desktop-0.5.3-linux-x64.AppImage`（如需要） | Linux x64 桌面应用（可选；默认走内网 only-Windows 通道） |
| `SHA256SUMS-v0.5.3.txt` | 本目录交付文件校验值 |
| `verification-v0.5.3.json` | 构建和升级验证摘要 |
| `desktop-download-http-verify-v0.5.3.json` | 隔离下载服务的 HTTP 校验记录 |
| `changelog.json`、`CHANGELOG-v0.5.3.md` | 累计更新日志及本版本说明 |
| `TESTING.md`（来自 `.trellis/tasks/09-30-real-machine-verification/`） | 关闭行为真机验证 checklist，发布前必须全部通过 |

`_build/` 是本机构建中间文件与详细日志，无需带入内网。已部署 V0.5.2 下载服务时，继续使用原 Nginx 服务和客户端 `updateUrl` 即可。

## 1. 校验文件

同 V0.5.2。把本目录顶层文件拷入内网，在 Linux 上执行 `sha256sum -c SHA256SUMS-v0.5.3.txt`，并确认 `uname -m` 为 `x86_64`。

## 2. 桌面端更新

如果是首次部署：按 V0.5.2 的 `desktop-intranet-update-runbook.zh-CN.md` 第 2 节配 Nginx；本节只覆盖在已部署下载服务的前提下切换到 V0.5.3 的步骤。

```bash
# 拷贝 V0.5.3 安装包到下载服务器的 incoming 目录
scp multica-desktop-0.5.3-windows-x64.exe{,.blockmap,.sha256} latest.yml \
    operator@download-host:/srv/incoming/desktop-0.5.3-windows-x64/

# 在下载服务器上
ssh operator@download-host
bash /opt/multica-updates/scripts/desktop-updates.sh collect \
    /srv/incoming/desktop-0.5.3-windows-x64 \
    /srv/incoming/desktop-0.5.3-windows-x64-collected
bash /opt/multica-updates/scripts/desktop-updates.sh publish \
    /srv/incoming/desktop-0.5.3-windows-x64-collected
bash /opt/multica-updates/scripts/desktop-updates.sh verify \
    /srv/incoming/desktop-0.5.3-windows-x64-collected/latest.yml \
    --expected-version 0.5.3
```

发布完成后，已配置更新源的客户端会在下一次自动更新检查时收到 V0.5.3。

## 3. V0.5.3 行为变化

### 桌面端

- **新增** 主窗口关闭按钮的行为选择（Windows / Linux）：
  - 完全退出（保留 V0.5.2 的默认行为）；
  - 最小化到系统托盘；
  - 每次询问（**V0.5.3 起新装用户的默认**）。
- **新增** Settings → Behavior 设置项，可随时切换上述三种方式。
- **新增** 系统托盘图标（已选最小化时），含"显示 Multica"和"退出"菜单项。
- **降级规则**：GNOME 40+ + Wayland 环境下因为没有可用的托盘基础设施，最小化选项会被自动禁用；如选择过最小化又切到不支持的环境，关闭窗口将退化为完全退出并打印一行日志。

### macOS

V0.5.3 在 macOS 上不改变任何关闭 / Dock 行为。

### 服务端 / Web / CLI

未变更，沿用 V0.5.2 的部署形态。

## 4. 验证

发布前必须完成 `.trellis/tasks/09-30-real-machine-verification/TESTING.md` 中列出的全部场景，包括：

- Windows 10 / 11 上首次关闭弹窗（含"记住选择"），以及托盘菜单的"显示/退出"；
- Ubuntu 22.04 GNOME + Wayland 上的降级路径（最小化选项不可用，关闭照常退出，单行 warn 日志）；
- Ubuntu 22.04 KDE Plasma / XFCE（X11）上的完整最小化流程；
- 已选最小化时双击桌面快捷方式能把现有窗口重新唤起（不重复启动）。

验证记录写入 `verification-v0.5.3.json`，结构对齐 V0.5.2 同名文件。

## 5. 升级 / 回滚

升级：已部署 V0.5.1/V0.5.2 的内网，把 V0.5.3 安装包发布到原下载服务即可。客户端不需要重新配置 `updateUrl`。

回滚：把 `latest.yml` 切换回 V0.5.2 的归档（脚本默认在 publish 时备份当前 metadata），客户端会继续停留在 V0.5.2。桌面端没有持久化数据迁移风险，回滚不影响 `~/.multica/desktop_prefs.json`；新增的 `close-preferences.json` 在 V0.5.2 上会被静默忽略，回滚后再次升级会沿用之前的选择。

## 6. 已知限制

- 系统托盘在 GNOME 40+ + Wayland 上不可用，与所有 Electron 应用一致。
- macOS 行为没有改；关闭窗口仍保留在 Dock（这是 macOS 平台惯例）。
- 双屏 / 高分辨率缩放的 Windows 机器上，托盘图标的具体尺寸由 Windows 决定，应用不参与。
