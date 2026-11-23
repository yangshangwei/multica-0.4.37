# 合并 main 前修复与验收结果

## 当前结论

本次限定修复和适用的本地验收已完成，未留下已执行范围内的未关闭失败。已验证的 38 个源码、测试、规范和生成文件已从隔离工作区带回当前分支，逐文件哈希一致；用户随后明确授权本地提交，五组工作提交已完成，第六组保存本任务记录；未推送或合并。

验收及提交前核对基线：

- 功能分支 `codex/cross-workspace-collaboration` 的验收源提交：`73a28ed90fdcfd9eda1ac5d2970df5af2c486f94`。
- 提交前远端功能分支与该本地基线一致；本轮本地提交追加后尚未推送。
- `origin/main`：`dd87e0b33c1d6a64d488f7ee9ecd0a65984a68ff`。
- main 是功能分支祖先，原验收基线有 863 个分支提交；本次五组工作提交已追加在该基线上，main 仍是祖先。
- 已纳入另一个会话后续提交的通知页、市场页及生成类型更新。分支观察记录见 [branch-observations.json](research/branch-observations.json)。短暂稳定未被当作其他会话完成通知。

本地提交完成后，建议通过 PR 对最终提交运行远端 CI，并明确处理现有图片预算例外，再决定合并。当前没有 PR 或该提交的远端 CI 通过记录，因此本报告不是无条件合并或发布许可。

## 本次修复

1. **大型分支使图片检查崩溃。** `check-image-budget.mjs` 改为流式读取 NUL 分隔的 Git 输出，修复约 3 MB 路径列表触发 `ENOBUFS` 和特殊文件名解析错误。保留 300 KiB 规则、已有图片缩小规则和显式 PR 豁免机制。
2. **调整窗口宽度使协作创建弹窗消失。** 将表单挂载到响应式侧栏的稳定同级位置，保留草稿实例。紧凑导航在创建时关闭；取消、Escape、关闭按钮恢复原导航及焦点；创建成功后直接跳转。沿用原生焦点接口，没有新增全局状态或依赖。
3. **旧归档通知链接触发拒绝刷新循环。** 已拒绝的实时订阅再次报 forbidden 时不再重复清空父级通知缓存。第一次权限清除、真实权限变更和合法恢复路径保留。实际 Web/Electron 原始用例验证稳定拒绝及私有内容清除；限定观测窗口内不再产生 503 请求风暴。
4. **测试和生成产物与已交付行为不一致。** 同步旧导航、欢迎页、断点及原生链接的夹具预期，保留原业务断言；限定修正 UUID AST 守卫，使持久通知重放继续复用原 ID；重新生成四份过期内置文档。

本次没有新依赖，没有改写生产通知 ID 或数据库迁移，也没有批量删除、压缩原始验收材料或重写 Git 历史。

完整文件清单和回写哈希见 [integration-result.json](research/integration-result.json)。

## 验证结果

| 范围 | 本次证据 |
| --- | --- |
| 前端静态检查 | 13 项 lint/typecheck 与 2 项 MDX 前置任务通过；31 条既存警告。修复后额外完成 Web/Desktop 消费方类型检查。 |
| 非 Mobile 单元测试 | 包级完整运行与受影响文件重放去重后 15,059 项通过；不声称一次修复后全库运行全部通过。 |
| Mobile | 196 项纯逻辑测试通过，类型和 lint 通过；7 条既存警告，不覆盖真机 UI。 |
| Go 后端 | 86/86 包最新完整 race 执行通过，25,564 个具名 PASS、24 SKIP；无缺失或未完成结果。Redis 补充用例另计 1 PASS。 |
| 构建、迁移与静态分析 | API、生产 Web、Desktop 构建通过；889 个迁移、Go vet、SQLC 149 个输出一致性、文档 157 个输出一致性和适用脚本契约检查通过。 |
| 漏洞扫描 | 当前 Go 1.26.11 的 govulncheck 退出 0，没有可达漏洞；保留 7 个未调用的模块级提示。Go race 测试使用 1.26.6，旧工具链扫描结果另存。 |
| 浏览器 / Electron | 538 个独立用例身份：464 PASS、74 SKIP、0 未关闭失败，零自动重试。初次 22 项失败及整个过程中出现过失败的 29 个身份均已闭环。 |
| 独立审查 | 核对源码边界、断言保留、逐用例结果、哈希和资源清理。最终审查见下方证据链接。 |

浏览器采用一次完整默认运行、完整受影响文件重放和配置明确的条件用例。312 个最新通过身份保留初次构建的适用证据，其余相关场景已按影响重放，不能称 464 项均在最终构建一次性运行通过。最后 MCP 回调参数有一个不改变正文/断言的 lint 整理，运行前后哈希差异已单列，未重标原始运行。

详细证据：

- [浏览器结果与验证范围](research/broad-browser-final-results.md)、[538 项用例清单](research/browser-case-results.json)、[最终独立审计](research/final-browser-evidence-audit.md)。
- [后端结果](research/backend-results.md)、[后端与资源证据审计](research/verification-evidence-audit.md)。
- [前端核算](research/frontend-verification.json)、[独立核算审计](research/frontend-independent-audit.md)。
- [响应式弹窗修复](research/responsive-dialog-fix.md)、[拒绝循环修复](research/notification-denial-loop.md)、[图片检查审查](research/image-check-independent-review.md)。

## 交付前仍需明确的边界

- **图片预算：** 实际检查能够完整执行，但无豁免时仍正确报告 121 张超限验收截图并退出 1。它们总计 52,151,039 bytes，均为原始验收证据。已提供 [逐项清单](research/image-material-inventory.json) 和 [现有规则下的 PR 说明草案](research/image-material-disposition.md)，没有自动批准或发布豁免。
- **远端 CI：** 当前无 PR/check-runs，Windows/Linux 平台任务与远端门禁尚未得到本次最终提交的结果。没有把本地检查称作远端 CI 通过。
- **74 项跳过：** 每项有明确前提，涉及特殊 PAY-241/detail/inbox 分配、安装包与跨客户端环境、性能容量、额外 Provider 夹具、设备认证/迁移等。完整清单见 [remaining-browser-prerequisites.json](research/remaining-browser-prerequisites.json)，它们不是通过项。
- **既存债务：** 普通评论机器身份继承管理员权限的问题在 main 已存在，独立任务仍待实现。大型 PR 文件过滤的 API 上限、部分源文档用语和 Knip 警告也已单列；没有借本次任务扩大重构或宣称已修复。

## 资源与交付状态

32 次任务运行分配的应用/Provider 端口全部关闭，gate 恢复原布尔值。本任务的浏览器 PostgreSQL 和后端 PostgreSQL/Redis 均在核验精确容器身份、零其他客户端后停止，端口已关闭。原始失败、日志、trace 与私有配置保存在受保护的任务目录，没有把原始敏感输出加入 Git。

主工作区的 38 个回写文件都经过原内容检查，未覆盖其他会话改动。最后执行 `git diff --check` 通过。任务保留 `in_progress` 以跟进剩余合并交付条件；本次用户确认的本地提交已执行，不包含 push、PR 发布或 main 合并。完整提交清单与授权见 [local-commit-delivery.json](research/local-commit-delivery.json)。


## 已执行的工作提交

- `153103f75` — fix(ci): keep large merge candidates fully checkable
- `2a8d44e87` — fix(navigation): preserve collaboration drafts across viewport changes
- `e5c843a75` — fix(collaboration): stop denied subscriptions from refreshing indefinitely
- `513417d49` — test(dbid): preserve notification identity through replay
- `f0a3f71cf` — docs: keep embedded guides aligned with current sources

提交后的38个 Git blob 与验收记录的 SHA-256 逐项相同。提交步骤没有重新运行整套测试，也没有将此前测试绑定改写为新提交；代码字节一致性由单独交付记录证明。
