# V0.8.8 发布前手册准备

## 变更边界

现有服务端手册仍把本次源码称作未发布协作上下文候选、迁移上限停在763，桌面手册仍使用0.5.1示例。实际 V0.8.8 目标为 Linux amd64 与 Windows x64，最新正式基线为 v0.6.7。仅修订 docs/offline-upgrade.zh-CN.md、docs/desktop-intranet-update-runbook.zh-CN.md 中的本次版本范围、迁移核对、操作示例和 CLI 来源限制；保留历史版本行为、认证/密钥/数据恢复说明。未变更运行代码、脚本、依赖、数据库或验收标准。

额外纠正一处已证实的文档矛盾：7.6 的知识默认 false 与 router.go、.env.example、Compose 和文档配置表不符，应为 true，已有显式 false 仍保留。

本修订不证明任何构件已完成，实际构建、签名、Windows安装与目标内网结果由本次发布记录单独提供。

## 已完成修订

- 服务端范围改为 V0.8.8 / 0.8.8 / v0.8.8，明确 Linux amd64 与 Windows x64 目标；保留其他架构仅作为辨认说明。
- 最近正式基线明确为 v0.6.7 完整 SHA；877 up/877 down、最大前缀889及各历史起点差异表均用于源码清单核对，不冒充实测路径。
- 要求逐文件名/哈希对比源码与构件，条件跳过须检查实际 schema 和索引有效性。
- 配置说明区分最近 v0.6.7 的四项新增传入与更早历史变化，增加既有服务端 LLM 的项目知识用途；修正知识默认值矛盾。
- 增加独立 CLI/daemon 受控替换、版本校验、身份保留、默认关闭公网自动更新及 upstream 来源限制。
- 下载手册统一当前操作示例为0.8.8，保留v0.5.2行为和2026-09-24的历史测试事实；记录相对v0.6.7无下载服务功能变化。
- 强化服务端先行、CI原始构件复用、download.json单独核对和blockmap源摘要比对；不预先声明构建、签名或安装通过。

## 验证

- `git diff --check`：通过。
- `node --test --test-name-pattern='upgrade archive includes publication tools' scripts/offline-changelog.test.mjs`：1 PASS，0 FAIL，0 SKIP。该既有测试实际生成 fixture 升级归档，核对 README.md / 操作文档.md 与修订源手册相同、包内链接、配置保留和启动顺序；使用 fake Docker，不是真实镜像构建或数据库演练。
- 通过 Git tracked 文件列表复核877个up和877个down；两份手册Markdown代码围栏配对；旧763范围和0.5.1当前示例均已移除。
- 文档修改未运行全量lint/typecheck/Go测试；父会话正在负责当前冻结源码和release脚本的更广验证，本子任务未重复执行。

## 后续

正式产物完成后主会话仍需核对两份包内手册与最终源文件哈希，更新外部发布记录中的最终源码、镜像/文件摘要及真实通过/失败/未测结果。未commit、push、触发CI或构建。
