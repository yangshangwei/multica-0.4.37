# 第三次完整检查的浏览器失败归因

结论：已核对全部 **47 个失败 + 7 个未运行项**。42 个失败有直接 PG 容量报错；另有 **2 个 Go 启动适配环境错误、2 个过时的收件人控件断言、1 个未决原生焦点失败**。因此只修复容量问题不足以关闭本轮失败。前三次完整检查继续保持 failed，不作追认。

被测源码 `639bf6ef85ad894553b244873c2ed8d9f9dd29b2`。原始完整 console SHA-256：`89c2d13730864522770980ff3fb32f5b7fa246a536bd80dae8718c46a0f78b74`。原 make 退出2；439项实际汇总为296pass、47fail、89skip、7did-not-run。原运行未配置 JSON reporter；配套 JSON 是从 console/源码/保留附件导出的复核矩阵，绝非补造的 Playwright 原始结果。`.last-run.json` 的47个失败ID仍保留。

| 类别 | 数量 | 实际证据与处置 |
| --- | --- | --- |
| 直接 PG 容量失败 | 42 | 每个独立失败块都有 `sorry, too many clients already` 和 pg-protocol连接握手栈。原日志共43处，第43处是Go模板清理，不是第48个浏览器失败。保留失败；连接预算恢复后仍需新运行验证业务断言。 |
| Go适配器环境契约 | 2 | #12/#13 preparation records 在fixture96行调用go run，PATH命中私有adapter，但98行子环境白名单删去其必需 `MULTICA_TASK_REAL_GO`，adapter14行KeyError，真实Go尚未执行。此错误独立于PG，容量修复后仍会存在。 |
| 旧收件人控件契约 | 2 | #14/#15 workbench126/243行仍寻找不存在的 `Exact recipient`。当前Transfer快照及AccountPicker源码均使用Recipient按钮→搜索→完整账号查询→Use this account；旧 `Selected account:` 文本也已过时。应更新测试操作，保留真实handoff及原键冲突断言。 |
| 原生窗口焦点未决 | 1 | #5 account-desktop233行预检期望native/contents/document均true，实际false/false/true，5秒超时；尚未执行后续剪贴板/弹窗/重启部分。88条保留API观察为86×200及2×admin/me403，无PG/500证据。根因仍需独立原生焦点诊断，不能归容量或宣称产品回归。 |
| 同suite setup失败后未运行 | 7 | knowledge计划433–439；其首项432在beforeAll的gate连接遇PG拒绝。Playwright1.58.2本地worker源码明确将同suite剩余项标为setup-failure未运行。七项没有业务断言证据。 |

#15还记录originalResponse超时：源码刻意用continueOriginal扣住首个完成POST，随后并发transfer又卡在旧收件人控件，响应等待因而超时。这是同一旧测试操作链的连带错误，不能另计为产品/API故障。

JSON逐项保留47个测试身份、源码哈希、console原行号/原失败块哈希、错误类别与建议，以及7个未运行项和89个跳过项的源文件/guard位置。PG连接事件栈没有保留每个测试的准确用户调用栈，因此不声称所有业务断言已到达；它们也没有通过。

下一入口：修复两类确定性harness/旧断言问题，处理独立原生焦点未决项；将所有自有API池和直接测试连接纳入总预算，避免新建API与完整检查重叠；重新绑定源码/适配器并开启新的编号完整运行。不得提高全局PG上限、终止非本任务会话、放宽产品断言或追加超时。go1.26.6 darwin/arm64工具链缓存存在，无需下载。

单独password mandatory组的memory嵌套fixture关闭顺序/gate泄漏及后续Leave namespace409属于另一专用数据库；该修复由memory-runtime-gate-closure负责，与这里47项不混同。默认运行中89个skip（含Host/installed/password等可选入口）及7个未运行项不能替代相应强制门验收。真实Provider消耗仍为0。

本复核只读原console、源码、JSON和保留附件并计算哈希；未执行测试、DB、浏览器、服务、CLI、容器、Provider、子代理或git/source修改。
