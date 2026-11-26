# Windows prelogger 启动诊断独立审查

日期：2026-11-26。范围：新collaboration-daemon-startup-diagnostics.mjs，加controller/公开projection/既有Node测试三文件。只读审查及窄测试，不改产品、startup流程、权限、超时或重试。

## 来源与定位含义

已检查runDaemonBackground、runDaemonForeground的实际顺序及managedProcessProfile、CheckManagedProcessAbsent、ReserveManagedProcess、ReadManagedStartupHandoff、management identity/lock helpers。foreground在logger前先读profile→检查managed PID/锁→读stdin handoff；原始本地错误由cli.FormatError的unknown分支原样trim输出，适合按稳定行前缀分类。ReserveManagedProcess写入真实child.Process.Pid，foreground许可自身os.Getpid，没有发现错写launcherPID证据。management_deployment_id到initializeManagedTransport成功后才持久化，缺失不能倒推Desktop遗漏配置。

分类结果是日志包含某个固定错误前缀或IO目标类别，**不是唯一调用栈或历史根因证明**。一个相同错误literal可从background/foreground或共享helper返回；raw IO只能定位操作/目标，不应当作确切source行。非空未识别输出保留output_present=true+空categories，不伪称没有日志。

## 安全边界

controller只对既有diagnosticFile已经只读取得的有界log tail新增startup分类，不启动/停止/重试daemon，不读取额外文件。新模块最多读取最后65536字符、256行；去开头空白及可选Error前缀，再按固定literal startsWith匹配，不在整段文本任意搜索类别。公共报告通过独立project函数再次只允许固定category和operation/target，构建新对象，无原日志、路径、用户名、PID、token、字节数/hash或未知字段。

已独立执行非前缀控制：无关正文提及OS boot identity、INFO行提及lock owner、任意正文含panic均不触发category。72个固定prefix映射62种category；独立从六个前置源码文件抽出25个errors.New literal出现，全部映射，和实现的source-reading回归一致。该计数不覆盖所有可能OS本地化文案/typed远端错误，因此不能称一次run必然给出唯一根因。

## 发现：IO target误分类，已要求修正

初始实现对整行normalized.includes判断文件名。本审查无副作用调用真实函数复现：`open C:/fixture/config.json.backup: error`错归config；`open C:/fixture/other: error mentions /daemon.pid`错归pid。虽不会泄露秘密，但会误导下一轮源码定位。

建议/已交实施者：先从anchored operation后读取PathError的实际路径段（与错误描述分开，Windows驱动器冒号不能误切），然后精确basename分类；lock只匹配明确.installation-locks路径段。rename等双路径无法可靠拆分应保守other，不从错误描述取目标。补反斜杠/空格正例和上述两负例。

## 已验证

独立startup/daemon窄选择8PASS/0FAIL/0SKIP，source-reading覆盖与秘密/未知值重投影测试通过。node --check和git diff --check通过。未运行PowerShell/Windows二进制、完整home或全量套件。IO误分类已按下节修正复核，不需要其他设计扩张。


## 最终修正复核与结论

**PASS：四文件可提交用于下一原生诊断run，无剩余阻断提交问题。** IO解析已先切出操作后的PathError路径段，Windows驱动器冒号不当分隔；按精确basename或明确.installation-locks segment分类，rename/无分隔/未知路径均other。原两条误分类复现现在均返回other，精确Windows锁目录仍正确分类。实现增加反斜杠/空格/错误描述提及/伪目录/rename正反例。

独立最终startup/daemon窄测试9PASS/0FAIL/0SKIP，两个fixture文件node --check和git diff --check通过。所有类别仅辅助下次判断具体分支，不代表旧run原因已确认；源字段中的source坐标仍是driver/error stack观察点，不能从category伪造新的source行号。

| 文件 | SHA-256 |
| --- | --- |
| `e2e/fixtures/collaboration-daemon-startup-diagnostics.mjs` | `bef19673871a9f9e315a5d2e764e4cc0b955a498a9e3abf9d88f824eefa3311d` |
| `e2e/fixtures/collaboration-installed-main.mjs` | `d4eb7ce666a9ba35462900fedcabb6a7feedbf5d370f81584678eb6b65892c13` |
| `apps/desktop/scripts/windows-collaboration-diagnostics.mjs` | `d2f866480b667ae3e437651bc279bfb7c6fc778d57825f5e2a1190ef960f5b7d` |
| `apps/desktop/scripts/windows-collaboration-config.node-test.mjs` | `084c44c61e024eebd550873d1664f844780cae32976ccc80d3c64f46d8d62529` |

本审查只写报告；未运行Windows二进制或全仓lint/typecheck，没有修产品或重启daemon。
