# 添加电脑：Windows / macOS / Linux

2026-10-11 · impeccable 设计提案 · 等待设计评审

这是现有「添加电脑」弹窗的局部适配设计。沿用截图中的白底、灰色命令区、克制的按钮和单列两步流程。目标是让用户在电脑 A 上获取适合电脑 B 的指令，并明确知道 B 是否已连接。

本目录包含静态设计稿与可编辑 SVG；没有修改产品代码，也不代表真实电脑已连接。图中的 `example.com`、`Office-PC` 和运行时列表均为示例。

## 设计稿

| 状态 | 预览 | 可编辑原稿 |
| --- | --- | --- |
| 三系统并排比较 | [平台对照](platforms.png) | 见各系统原稿 |
| 首次打开 | [未选择系统](initial.png) | [SVG](initial.svg) |
| Windows | [PowerShell 指令](windows.png) | [SVG](windows.svg) |
| macOS | [终端指令](macos.png) | [SVG](macos.svg) |
| Linux | [终端 / SSH 指令](linux.png) | [SVG](linux.svg) |
| 地址需要配置 | [localhost 提示](address-needed.png) | [SVG](address-needed.svg) |
| 帮助展开 | [无浏览器登录与排查](headless-help.png) | [SVG](headless-help.svg) |
| 连接成功 | [电脑与运行时](connected.png) | [SVG](connected.svg) |

帮助展开稿单独成图仅为便于评审，实际仍在原弹窗内展开。PNG 为展示产物；SVG 使用系统字体，跨系统查看可能有轻微字体差异。

## 设计要点

1. **先选目标电脑系统。** Windows / macOS / Linux 使用分段选择，不根据当前电脑推断远端系统。未选择时不展示可复制命令；选择后在原弹窗中显示对应步骤。
2. **保留两步主流程。** 「安装 Multica CLI」与「登录并连接电脑」。后者准确描述 `setup` 包含配置、登录和启动守护进程的行为。
3. **命令有明确的执行环境。** Windows 显示 PowerShell，macOS 显示终端，Linux 提示终端或 SSH。Windows 的说明中补充：AI 工具装在 WSL 中时选择 Linux，并在同一个 WSL 环境执行。
4. **等待与成功分开。** 等待使用中性时钟和文字，真实接入后才显示绿色成功状态、电脑名及可用运行时。
5. **异常在对应步骤解释。** localhost 影响第二步，因此在第二步提示地址需要配置；保留第一步，不继续展示误导性的连接命令。
6. **帮助默认折叠。** 无浏览器登录分为配置地址、交互式输入令牌、启动守护进程三段。登录命令与后续命令分开复制。

完整交互、边界与后续实现约束见 [Trellis 设计记录](../../../.trellis/tasks/10-11-add-computer-platform-design/design.md)。

## 验证范围

- 已对照原截图和项目字体、色彩、Tabs、Dialog 组件检查静态设计。
- 已检查三系统稿、地址提示、无浏览器帮助和成功态的布局、文案与命令来源。
- 已检查文字颜色对比度以及所有本地图片 / SVG / 文档链接。
- 未运行产品自动化测试；本次没有应用代码改动。
- 键盘、屏幕阅读器、实际剪贴板、窄窗口布局及三系统安装需要在功能实现后验证。
