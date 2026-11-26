# Windows Provider callback socket 独立定位与审查

日期：2026-11-26。基线fc9b9cc9027f4b93aa492f58a61730a752d71d6b；run45702305203。**最终PASS：三文件可提交并进行下一Windows原生验收，未发现剩余阻断提交问题。** 下文保留初始定位及实施审查过程。

## 事实与根因边界

直接读取公开phase报告：boot/runtime/login/stop/restart_while_paused存在，失败坐标host-runtime.ts:446的waitForVisit及spec:193第三visit等待。parent已从授权私有取证确认expected count3/actual count3、exitnull，但errors中有Provider callback socket failed；本审查没有读取私钥或私有解密原文。公开resume里程碑false仅表示尚未执行后续证据赋值，不能单凭它否定先前真实Resume请求。

现有socket error listener不保存code、socket与visit/run关联或事件时序，把所有错误追加同一文本。close listener只记录对应visit.closed_ms。第二provider在收到真正业务Stop后被终止；它一直阻塞等待callback控制字节，Windows强杀可能引发连接reset，但**历史只保留generic错误，不能确定哪一socket或错误码**，也不能把第三visit已到视为此前所有网络错误无害。

## 确定性本地TCP对照

无副作用Node net小复现：服务端收到唯一run首帧→发送ACK，客户端等ACK后分别end或resetAndDestroy。结果：

- 正常FIN：identity→close(hadError=false)，无error。
- 未登记Stop的RST：identity→error(ECONNRESET)→close(hadError=true)。
- 已登记Stop意图的RST：identity→stop_intent→error(ECONNRESET)→close(hadError=true)。

真实TCP验证两个RST形状相同，因此error code/closed标记不够证明业务因果；必须另用前置意图与真实服务端receipt/proof限定。此复现模拟RST传输，不是假称重现了该历史Windows强杀。

## 可接受的最小实现契约

父会话已授权spec在发Stop前登记exact visit的一行观察；登记时该socket须已通过identity、open、无既有error、未release、无pending action。记录之后仅该socket的ECONNRESET可成为待分类事实，不立刻丢弃。

真实Stop receipt必须成功且target正确；同一run随后由服务端证明settled/proof_state=host_exit_observed/forced_disposition=stopped，socket也closed，才可把该后发reset列为受控Stop连接终止。保留原始错误事实和归类依据，不清errors或所有pending错误。未登记/提前发生/别的socket/不匹配run或epoch/缺receipt或proof/未closed/非ECONNRESET必须继续阻止waitForVisit通过。

不能扩大provider协议、暂停权限、忽略动作回执丢失、增加timeout或重试，不能为了通过将所有socket错误改warning。正常回调、非Stop reset与不完整Stop proof要有明确负对照。

本轮未改实现或产品，未跑重型构建。仅本地临时loopback TCP控制已完成并关闭全部自有socket/server；后续独立复核具体实现和证据。


## 实施复核（当前三文件）

只改host-runtime fixture、其Node测试及完整installed spec的受控Stop观察点；未改Provider或产品。spec在Stop前调用expectControlledStop(second)，原control成功/status/input/receipt断言保留，随后取得真实waitHostSettlement同run并等待callback closed，再用stop.receipt确认。

createHostCallbackErrors以HostVisit对象而非仅run字符串绑定具体socket。expectStop要求poller拥有该visit、socket未destroy/未closed、未release、无pending action、无既有错误且未重复intent。error发生时原事实append-only记录code、afterStopIntent和idleAtError；仅ECONNRESET保留具体码，其他为other但仍报错，不公开raw错误文本。

confirm要求Stop receipt code/target_id与visit匹配、run.id匹配、progress settled、proof host_exit_observed、forced_disposition stopped、socket destroyed且closed、未release且无pending动作。只有win32+ECONNRESET+afterIntent+错误当时idle+同对象confirmed组合才从未解决错误视图排除，原failure记录和callback_errors证据不删除。正常平台/非reset/未绑定visit/早期reset等仍为错误。

本审查发现的三点已修复：after_stop_request改名after_stop_intent避免把“请求前登记”说成已发送；错误时pending动作状态固定下来，不能在finally归零后被追认无害；seedHostRuntime的对象展开会冻结getter，现wrapper显式保留动态get errors。最后summary.errors===[]新增，确保第三provider释放后的晚发错误仍可失败，而非只在waitForVisit检查。

## 独立验证

- 当前host-runtime Node套件12PASS/0FAIL/0SKIP，包含真实TCP resetAndDestroy与原close/控制/权限目标测试。
- 与installed-main组合27PASS/0FAIL/0SKIP，git diff --check通过。
- 额外调用从实际TS源码转译的tracker：同run ID不同socket错误不可被另一对象proof豁免；未closed、proof unknown、错误receipt code均拒绝且错误保留。
- 实现负例覆盖非Windowsreset、Windows EPIPE、intent前reset、没有identity、pending-at-error后来归零、released/closed/动作占用。原30秒等待与业务所有断言不变，未增加重试或skip。

此实现允许的分类是“符合受控Stop关闭边界的reset”，不能倒填旧fc9日志的具体socket/code或宣称已复现其Windows OS强杀原因。下一native必须保留callback_errors并验证完整三次运行、Stop/restart/Resume及最终summary，才可关闭本次验收阻塞。


## 最终确认

实施者已将本审查的notclosed、unknownproof、same-run-ID/different-socket负例落入既有测试；独立最后重跑12PASS/0FAIL/0SKIP，diff检查通过。实施者另报告两TS根及依赖严格program零diagnostic，本审查未重复该检查，不冒称自行运行。没有改Provider或产品代码。提交可行不代表fc9历史传输因果已确认，也不代表新native已通过。

| 文件 | SHA-256 |
| --- | --- |
| `e2e/fixtures/collaboration-host-runtime.ts` | `fa6157a171c3bd646703d0d2a73e14fdfb9768e805ef3ee17d389277a5439237` |
| `e2e/fixtures/collaboration-host-runtime.node-test.mjs` | `b5bb7548c8b5496840e252a5bf881114019e7e719f9f55a978df852eba5b2bf4` |
| `e2e/collaboration-host-installed.spec.ts` | `aa4ee4ec46285de0529065b076792f61066d0496b37ece186d6ba391e40581a2` |
