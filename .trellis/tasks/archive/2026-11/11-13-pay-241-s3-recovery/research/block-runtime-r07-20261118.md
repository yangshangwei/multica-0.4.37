# TC-23：结构化阻塞与解除，R07

**R07 仅部分通过。** 在同一候选上，Web 完成 A/B/C/E/F/G 六项；Desktop 完成 A/B/C/E，F 在准备测试账号时失败，G 未启动。共 10 PASS / 1 FAIL / 1 NOT_RUN，Playwright 零重试。没有用前序轮次补齐桌面分母。

两端均实测阻塞表单必填项、带时区的预计解除时间、指定责任人、解除后的状态、原回执恢复和受限历史。Web 还完成嵌套请求逐层返回（F）与保留原待验收结果（G）。R05 的收件人查询被误计为写入、R06 的执行视图协议版本不一致，都是测试夹具问题；两个原始失败均保留。

Desktop F 的第一个账号注册返回 HTTP 429；此前同一轮有 30 次注册成功。它未进入业务逻辑，也不证明嵌套返回失败。匹配的服务端仍执行每来源每五分钟 30 次注册的规则。后续用新专库、相同构建和业务断言，测试账号准备仅依服务端明确的 `Retry-After` 等待；不提高配额、不重试业务写入。

停机后，专库保留 20 个任务、43 条交接、65 个协作事件、80 条操作记录；运行、命令和普通任务队列均为 0。创建开启，执行/宿主关闭，门禁 epoch 2；三个本轮监听均退出，专库其他连接数为 0。没有执行真实支付、模型、远端 CI 或发布。

证据根：`/Volumes/artisan/.multica-acceptance/pay241-s3-clients-20261118-x4c5mjep/runtime/block-information-r07/`。`playwright-report.json` SHA-256 `d05d8a01897899a13d61efc51643fa3fb0334b62b41a203668eb8c5ba2c148c2`；`actual-run-01/database-after.json` SHA-256 `72e864f6ad934a2f2fdcb7db5cdc4ef1d1af28d3bd00a5e569f617dc7a23aff1`。参见[后端独立检查](block-package-r05-independent-check-20261118.md)及[双端构建记录](block-ui-build-r05-20261118.md)。
