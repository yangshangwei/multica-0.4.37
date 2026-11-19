# S3 人工上传回执核对（2026-11-18）

**只读核对通过。** R04 的 Web 人工上传场景有三个独立的原始请求：丢失成功响应后通过 GET 恢复、用相同字节重试、拒绝同名同大小但内容不同的文件后重选原文件。R04 整套仍是 **2 通过 / 1 失败 / 3 未执行**；下一轮的成功不改写这一结果。

本次在 API 停机后，用 `REPEATABLE READ READ ONLY` 分别读取 R03、R04 的专库。两次查询都核实库名、只读事务和其他连接数为零，未启动服务或重发请求。按每个原始 `(uploader, draft, request)` 核对，而非只比总数：

| R04 Web 证据 | 结果 |
| --- | --- |
| 原上传与磁盘对象 | 3 个请求各对应 1 行、1 个不同的私有对象。逐一验证认证标签、解密大小和 SHA-256，均与原 multipart 文件及实际 API 下载一致。三种情形的上传 POST 数分别为 1、2、2。 |
| 评论与回执 | 每个对象有 1 个当前任务引用、1 个当前评论引用；3 条原评论及其版本、事件 ID/序号、原操作键和完整回执全部匹配。普通附件表为 0。 |
| 共享目录与历史 | R04 库有 5 个附件（含 2 个撤权夹具）；同一目录另有 R03 的 2 个。磁盘的 **7 个**对象恰等于这两组，不把旧对象算成本轮新增或孤儿。执行/宿主开关保持关闭，创建开启，epoch 2。 |

R05 是另外一次完整运行，使用独立上传目录：Web/Desktop 各通过上述三个上传子场景；6 个原元组各有 1 行、1 个对象、2 个有效引用。另有 4 个撤权控制附件，全库 10 行对应 10 个对象，无孤儿。R05 校验的是 6 次实际 API 解密下载；**离线解密核对仅在 R04 执行**。人工上传不是智能体运行输出，不据此关闭 TC-14。

[独立审阅](extras-a-r05-independent-check-20261118.md)指出 R05 原审计只保留查询摘要、没有 SQL 正文。为让这部分能复核，已另存并执行[参数化只读查询](extras-a-upload-receipts.sql)：在对应停机专库的只读事务中，`$1` 使用原 `*-extras-uploads.json` 的 `issue_id`。左连接保留缺失的回执/评论/事件，不因内连接漏掉失败行；每个任务返回恰好 3 行。R04 Web 加 R05 Web/Desktop 的 **9 条原操作回执、9 个原评论/事件、18 个有效引用**与原始客户端记录相符，磁盘对象长度亦匹配。此查询是新的独立复核，不声称重建了旧 SQL。

原始证据根：`/Volumes/artisan/.multica-acceptance/pay241-s3-clients-20261118-x4c5mjep/`。未输出附件明文、加密密钥或账户凭据。

| 相对于证据根的文件 | SHA-256 |
| --- | --- |
| `independent-check/retained-upload-r04-r05/r04-result.json`（同目录 `check.cjs` 为只读审计脚本） | `abf4507142d132ea3b56456bf503ddcb64570774290a6644835abd66cfa86365` |
| `runtime/extrasA-r05/upload-row-object-reference-audit.json` | `45cbe382c22582a4242bd6cba7151f0c3a19e6c05fcead75dea7c28b16b3cfc2` |
| `runtime/extrasA-r05/playwright-report.json` | `0de570cd9ddf958ee216de00918f632ea8299d737099dcb414769de28f232527` |
| `runtime/extrasA-r05/seal-final-r05.json` | `98398e457aab7bbebd3632dbc41652fe7e38f28860dfcb7cc1175a112311d470` |
| `independent-check/retained-upload-r04-r05/original-receipts-sql-result.json`（上述 SQL SHA `1b4ed596020b3e55d1c5edfa80994605c28b2ce84533fe20f84f4c222a293501`） | `26c62ca1813d462950c7e617203fd2e81d6c56881bf82db8598d7ae130b46a64` |
