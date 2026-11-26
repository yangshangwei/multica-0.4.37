# Windows acceptance 文本边界与关闭夹具修复

## 证据与边界

run45660381726已通过checkout，但Validate acceptance scripts失败。日志显示workflow文本是CRLF，而windows-collaboration-workflow.node-test.mjs的LF-only多行正则/拆分失败。未涉及产品逻辑、Windows安装或发布门槛。

修改前在临时目录复制真实测试及其读取输入，分别转换全部副本为LF/CRLF，不修改仓库换行或Git配置：workflow LF 3PASS，CRLF 1PASS/2FAIL；Host CRLF 6PASS/3FAIL（真实close源码定位失败）。同一Host测试LF也6PASS/3FAIL，错误为mark is not defined；真实close已添加观察时间线的stdin_end_requested标记并返回timeline，测试vm夹具尚未注入这两个词法依赖。

最小修复：两份测试在对应文本read边界将CRLF归一化为LF，保留全部断言；Host closeFixture补齐mark/timeline观察依赖，不删改cleanup顺序、EOF、POSIX信号、强杀退出码或错误检查，补充时间线在EOF之前记录且被返回的断言。主会话已明确授权该同文件夹具修复。无产品代码或工作流门槛变化。

## 标记的真实语义

真实 `collaboration-host-runtime.ts` 的 `mark` 把固定事件名、单调时钟耗时和 daemon 代数加入 timeline。close 在 stdin.end() 之前标记 stdin_end_requested，随后沿用原EOF、20秒强杀上限、退出等待、socket/server收尾、账本摘要和私有目录删除，返回额外的timeline。测试夹具仅收集固定事件名以验证调用顺序与返回引用，不模拟生产时钟，不更改实际生命周期行为。原cleanup和错误断言全部保留。

## 验证结果

- 修复后真实文件的LF与CRLF临时副本：workflow各3PASS、Host各9PASS，均0FAIL/0SKIP。
- 两种换行均保留否定能力：删除workflow清理步骤always()时同一安全断言失败；删除Host实际close中的mark调用时新增timeline断言失败（各1FAIL）。没有放宽正则或省略被测步骤。
- 运行本workflow“Validate acceptance scripts”中的所有10个Node测试文件（统一node --experimental-strip-types --test）：44PASS / 0FAIL / 0SKIP；包含旧例程退出、cleanup错误传播、原生路径、受控provider/HTTP/WebSocket、更新feed和Go事件验证。运行平台为macOS，不能替代Windows原生结果。
- `git diff --check`通过。

## 同一步余项检查

windows-candidate-version已在读取workflow边界归一化CRLF。installed-main两处源码定位使用单个\n，不含跨行LF-only字面串，在CRLF中仍可找到，未改动。其他Node用生成的JSON/流字节、显式写入的\n或JavaScript导入，不依赖checkout行尾；PowerShell检查通过AST parser读取文件，不采用LF正则。未为假设风险扩写产品或配置。

后续：父会话重新冻结并dispatch Windows工作流，保留前次失败。未commit/push、未改用户Git全局设置、未改产品代码或workflow验收门槛。

## 后续发现脚本跨平台调用修复边界

父会话授权扩至collaboration-installed-main.node-test.mjs，仅修测试执行方式。createInstalledMachineIdentity生成的是用于Linux容器的无扩展名#!/bin/sh文件，Windows不能依赖CreateProcess直接执行它。Windows使用Git for Windows安装目录中的明确sh.exe和同目录cygpath.exe；用参数数组运行同一脚本，通过cygpath显式转换脚本与marker路径，不使用默认bash/WSL或shell拼接。缺少shell/转换工具时断言失败。POSIX继续直接执行并检查0555，machine-id的0444精确断言保留。未修改生产helper。

## 发现脚本修复结果与验证边界

- Windows测试明确选择 `%ProgramFiles%/Git/usr/bin/sh.exe` 和同目录`cygpath.exe`（GitHub Windows日志中的Git安装根为Program Files/Git）。缺ProgramFiles、工具文件、转换失败或非绝对多行输出均断言失败，不skip。
- 同一个Linux容器发现脚本通过sh的文件参数执行；脚本路径及marker均由cygpath -u参数数组转换。没有shell:true、bash默认解析、WSL、-c拼接或凭据传入。
- macOS/Linux仍直接执行生成的脚本，并新增精确0555检查；测试临时路径加入空格，原version、exit97、marker内容、不含敏感输入、cleanup全部断言保留。spawn错误显式报告。
- machine-id的0444精确断言未改变。Node官方文档说明Windows只支持修改写位且不区分owner/group/other；libuv源在只读属性存在时映射为0444，所以该断言是只读属性证明，不能声称验证了Windows ACL隐私。参考：https://nodejs.org/api/fs.html#fschmodpath-mode-callback 、https://raw.githubusercontent.com/libuv/libuv/v1.51.0/src/win/fs.c （fs__stat_assign_statbuf）。路径转换依据：https://www.msys2.org/docs/filesystem-paths/ 。

全部修改后的本机执行：`node --experimental-strip-types --test scripts/windows-candidate-version.test.mjs e2e/fixtures/collaboration-installed-layout.node-test.mjs apps/desktop/scripts/windows-collaboration-config.node-test.mjs apps/desktop/scripts/windows-collaboration-workflow.node-test.mjs e2e/fixtures/collaboration-installed-main.node-test.mjs e2e/fixtures/collaboration-host-runtime.node-test.mjs apps/desktop/scripts/windows-update-probe.node-test.mjs apps/desktop/scripts/windows-business-provider.node-test.mjs apps/desktop/scripts/windows-business-proxy.node-test.mjs apps/desktop/scripts/verify-go-test-events.node-test.mjs` → 44PASS / 0FAIL / 0SKIP。git diff --check通过。

本机无pwsh/powershell/wine，因此Validate acceptance scripts内的两份PowerShell测试和两份PowerShell AST检查无法本机执行，未伪报整步通过；它们在run45660381726到达Node失败前已执行，最终新SHA仍须原生重跑。Windows Git sh/cygpath的实际路径转换、文件只读属性与CreateProcess运行结果也只能在Windows runner确认。本机测试覆盖POSIX直接执行分支，不能代替Windows原生验收。未安装新依赖，未修改工作流、产品helper或全局Git配置。
