# Runtime 集成交付与验收

代码已合入协作开发分支：`0e1ee8e1e`。固定输入快照、独立启动注入补丁和晚到的并行改动均已核对、归档并整合；主工作区与隔离集成目录的 116 个交付文件逐字节一致。

**本轮实现、已列确定性门禁及 Web/原生 Desktop 验收已完成；台账审计发现的补充矩阵已完成，见文末，真实模型及更广平台验收仍独立。** 本任务暂保留 `in_progress`，不把代码检查通过等同于全部模型行为验收通过。

## 已交付

| 范围 | 结果 |
| --- | --- |
| 协议与读者 | checkpoint、metadata seal、stream/ordinal、固定水位分页及旧 outcome 兼容；内部事件过滤不泄漏正文或伪造完整性 |
| 记录保留与恢复 | 有界异步 sender、原 key/payload 重放、单调 seal、未知崩溃尾部；整个 telemetry 恢复共用最多一秒预算，原 Stop/proof 优先且不依赖上传成功 |
| 启动输入 | run 绑定的稳定 handoff/memory 来源；每次真实启动绑定准确 prompt hash 与 attempt/nonce，保持已受理派活及原输入 |
| 来源与召回 | 当前事件水位、嵌套恢复的信息请求原事件、来源属性、读者与唤醒原因；普通评论通过独立有界、按权限和版本校验的 recall/content 读取 |
| 预算与解码 | 256 KiB 材料和 2 MiB JSON 传输分别校验；合法 64 KiB 转义派活完整保留；缺失/null/大小写别名和非法 retention 在启动前拒绝 |
| 客户端 | Core schema/query/分页一致性、共享 handoff/保留水位/评论恢复、处理人名称与稳定 ID；Web 同文档 fragment 导航保留 issue/run |
| 指导与规范 | CLI 分页/全文读取、data_only、权限语义及 source map；启动、恢复和记录规范与实现对齐 |

## 验证结果

| 检查 | 实际结果 | 证据 |
| --- | --- | --- |
| 完整 Go 协作门禁 | 1,380 个顶层测试、2,573 个子用例通过；没有缺失预期测试；一项已有 CLI unknown-fields 配置往返测试按原说明跳过 | `research/integration-verification/final-backend-round2/` |
| Go 静态检查 | 完整 `go vet ./...`、`go build ./...`、71 个变更 Go 文件格式检查通过 | 同上 |
| 完整 TypeScript | 1,054 文件、12,982 tests 通过，五个 Turbo 任务全部实际执行，零缓存；前端源码检查前后不变 | `research/integration-verification/final-frontend/` |
| Lint / typecheck | 全仓通过，已有 lint warnings 保留 | `frontend-fix/`、`full-static-initial/`，位于同一 verification 目录 |
| SQL 生成 | pinned sqlc 1.31.1；120 个生成文件初始无漂移，后续查询变更和生成物同步验证 | `sqlc-initial/`、`server-fix/` |
| 生产 Web / 原生 Electron | 六个独立用例全部通过；0 错误、0 跳过、0 自动重试；包括新 handoff/retention/评论恢复/撤权和既有交接链 | `final-browser/summary-final.json` |
| 晚到改动复核 | 未引入新的生产代码；补充 daemon/protocol race、HTTP ACK、Stop/recovery、Views 测试通过 | `late-reconciliation/` |
| 独立代码与视觉审查 | 所有已报缺陷关闭；最终原生截图与原布局一致，视觉95/100；仅代表已捕获视口 | `final-review.md`、`final-browser/visual-verdict.json` |

Go service 与 handler 使用专属新数据库顺序运行，未与浏览器共用数据库。默认测试有 agent CLI guard；本轮上述测试没有调用真实 Provider。

## 问题闭环

- 旧 fixture 退出阻塞：增加显式释放，不能把清理挂起误当产品死锁。
- recording 原请求未恢复/未封口：接入真实正常与重启路径；录制端持续阻塞时原终止证明仍完成。
- 本地批次拒绝后的序号重排：区分不可恢复 staging 错误与可重试 HTTP 错误，停止后续 staging、继续安全 drain，保留既有有效前缀。
- 健康根任务被派生活动占住：非启动证据中的 `wakeup_assigned` 不再形成假 fence，其余未知/实际启动证据仍受保护。
- 交接包丢失水位/信息请求及反射伪造来源：通过真实授权入口补齐和校验，并保留缺失历史的明确标记。
- 游标与畸形支持版本：拒绝缺失/错误 run 绑定、重复及未知字段；typed JSON 只接受规范字段名，不能用大小写别名改写完整性。
- 跨页 retention 混合：页面链校验流身份及元数据，保留合法刷新与旧接口语义。
- Web 来源链接：Next 的旧 canonical URL 重复拼接 hash；在平台层使用受支持的同文档 History API，保留其他导航语义。
- 最后一项 Web 失败是测试旧文案：诊断证明 URL、展开和 API 均正确；改为 `recording.status=not_recorded` 的实际语义并保留原失败。

## 来源、并行工作与限制

运行从 `e3620721e` 加固定工作副本开始，候选3源指纹为 `5b0262606d143ca44fc2ab19247eece7025087cda2541372f67afc218b76245b`。之后只调整 E2E 诊断/正确文案和补充晚到测试/规范；生产代码保持已验证内容。提交前核对所有交付文件的索引字节，最终形成 `0e1ee8e1e`。

源码、原始失败、命令、数据库标识、版本、源码 hash 和组件运行信息均保存于 `research/integration-verification/`。初始与晚到主工作区备份、逐文件应用核对位于 `research/integration-coordination-2026-10-31/`。没有覆盖未知并行工作、推送远端或发布生产版本。

原模型系列在本报告交付后独立续跑了 E2a trial 2，约 196 秒后返回上游 502，未读取场景上下文，仍无有效 E2a 样本；详见修复闭环子任务 `verification-e2a-trial2-2026-11-01.md`。下一次 E2a trial 3，剩余上限 22 次。原 trial 1 的 429 为 `gateway_concurrency_limit`；本轮仍不能确认上游恢复。新的 runtime 模型验收必须记录自己的源码与实际覆盖路径。Windows、安装包组合和多账号额度溯源仍属于关联交付矩阵，不能由本轮 macOS Electron/fake-provider 结果推导通过。

知识候选正式设计、实施计划及上下文清单已完成，K1–K5 和补充 F1–F4 工程合同均已独立复核。独立工作区已进入首批 schema/protocol/纯策略实施；候选功能和实际注入尚未验收，G2 通过前保持未启用。

## 后续台账复核

逐项核对原实施清单后，补齐了先前未实际覆盖的矩阵，均无生产代码变更。`0bbc6a77a` 记录 checkpoint 与 Stop/撤权/作者退出的六种提交顺序、双成员及 claim 前后唤醒来源，normal/race 与独立复核通过；证据为 `research/integration-verification/acceptance-matrix-addendum/`。`84c4c2b3d` 记录小队四类异构预算、memory 冲突后的显式双事实合并，以及新私有 handler 凭据拒绝/正向对照，共57个叶子场景通过；service race、handler、vet/gofmt 与独立复核通过，证据为 `research/integration-verification/remaining-acceptance-addendum/`。原始失败、复验、代码与证据哈希全部保留，不重复相加容器测试或冒称真实 Provider/整个生产 router 验收。
