# Windows installed renderer URL 修复独立审查

日期：2026-11-26。结论：**当前两文件可提交，未发现阻断提交问题；下一Windows完整旅程仍需真实执行。** 此修改是验收fixture路径比较修复，不代表已证明产品窗口加载缺陷或协作验收通过。

## 来源事实

独立读取run45678396231的原phase报告，source为dc632d09c77f8cfaf0b3b903fa91576af14581f8。API/Web进程存在、存活、path/start/source匹配全部true，health/healthz/config均HTTP200/valid；失败位置在installed-main.mjs原429，boot/runtime/login未达到。因此先前服务生命周期前置已在此run得到证明，不能继续把本次失败归为服务失联。

原launch两次把URL.pathname与原生packagePath拼接值比较。Windows原生路径为D:\...，file URL pathname为/D:/...且使用斜线/百分号编码，二者即使指向同一文件也不相等。已有nativeInstalledLayout从显式安装exe构造正确pathToFileURL.href，应复用该唯一表达。

## 具体边界

- 新expectInstalledRenderer抽取唯一条件给firstWindow及reload两处：native使用new URL(page.url()).href精确比较nativeLayout.rendererUrl；Linux容器仍使用原pathname与固定packagePath/out/renderer/index.html。
- 原file:协议检查、安装exe/ASAR/CLI来源和摘要校验、显式native home、启动/daemon/IPC及全部业务断言保留。没有增加超时、重试或skip，没有修改产品Electron/renderer。
- native比较不decode两次，不把Windows原始斜线/驱动器盘符和URL文本混用；空格、字面%20与#使用已有编码规则。错误HTML/ASAR、foreign file host、wrong drive、HTTP localhost及附加query/hash拒绝。
- URL规范化可接受同一合法file URL的标准等价表示；不是允许远端业务页面替代安装renderer。Linux分支仍保留旧pathname语义，不以本次修复暗称加强了容器URL约束。
- 检查当前桌面routes使用createMemoryRouter，主窗口通过loadFile(rendererEntry)，并非以hash router存储路由；新增native精确href对query/hash拒绝与本fixture场景一致。

## 测试真实性与独立验证

新增测试通过既有VM模式读取并执行实际expectInstalledRenderer函数，不在测试重写比较逻辑。真实nativeInstalledLayout提供Windows普通/空格/%20/#及macOS路径；负例检查错误文件、ASAR、foreign host、query/hash和二次解码。Linux两种固定包根保留通过并拒绝错误HTML。

独立运行installed-main、installed-layout、windows-config及workflow四套窄测试：**31 PASS /0 FAIL /0 SKIP**。额外直接执行真实函数确认localhost HTTP、foreign file host、错误盘符被拒绝。node --check及git diff --check通过。

没有执行Electron、WindowsGUI、全仓lint/typecheck或重型构建。本机成功证明路径断言的计算和否定能力，实际page.url在first-window和reload均通过仍须下一native run。原dc632失败及新源码SHA变化均应保留并绑定新构件。

## 文件摘要

| 文件 | SHA-256 |
| --- | --- |
| `e2e/fixtures/collaboration-installed-main.mjs` | `7e6296bbeb53379013188fcff79053566a007fbbe114f967fb797d4cb4a3ff0b` |
| `e2e/fixtures/collaboration-installed-main.node-test.mjs` | `edc59bd935fa2c36bc3d5c97e59e98db70088acba60d79deb8df5b68ebe2d82c` |

仅新增此报告，未修改实现或原始证据。
