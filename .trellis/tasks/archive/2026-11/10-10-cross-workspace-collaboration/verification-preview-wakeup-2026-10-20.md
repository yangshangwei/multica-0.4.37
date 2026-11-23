# 触发预览与父任务唤醒验证

本轮完成用户指定的两项增量：触发预览的合批/续作事实，以及协作子任务完成后按普通阶段规则唤醒父任务。原跨账号协作父任务继续保持 `in_progress`，不据此勾选全部 97 项验收。

基线：`a4b9f188b`，分支 `codex/cross-workspace-collaboration`。开工时的 38 项未提交改动已单独备份，并与本轮改动区分。命令、日志、截图及生产进程身份保存在 `.omx/reports/cwc-preview-wakeup-20261020/`；本轮没有新增依赖或迁移。

## 已交付

- 预览返回独立的可选 `grouping`，说明本次目标的同批合并、已有有效等待输入、后续批次或当前小队续作。按真实输入顺序、目标授权、批次容量和序列化字节限制计算；不返回先前输入正文或内部批次/运行身份，也不写入队列或恢复暂停。
- FIFO 和预览复用原有的小队兼容性与精确输入大小计算。服务端回归将预览与真实评论提交/FIFO 选择对照，覆盖第二个同队长小队、取消目标、失效作者/资源、暂停/关闭门禁和续作边界。
- core 为预览单独解析新字段，旧响应缺字段仍可使用，畸形/不对称分组被拒绝。共享评论界面用中英文显示每组事实；只有草稿 UUID、正文、回复、取消目标和附件身份全部匹配时，预览才可用于发送。
- 协作完成/取消事务成功后，HTTP 层调用既有普通子任务阶段唤醒逻辑。仍遵守最早未完成阶段、未分阶段兄弟任务、普通 agent/小队队长和自定义终态规则。共享、人工负责、backlog 或终态父任务不会进入普通执行。
- 唤醒信息只含子任务最小编号/标题，不携带私有正文或可打开的普通子任务链接。临时前后状态不序列化；重放、拒绝、回滚和不确定提交不会重复派发。

主要文件：`server/internal/service/collaboration_mention_grouping.go`、`collaboration_mentions.go`、`collaboration_batches.go`、`collaboration_lifecycle.go`、`collaboration_workflow.go`，`server/internal/handler/collaboration_lifecycle.go`、`issue_child_done.go`，`packages/core/collaboration/content-schema.ts`，`packages/views/collaboration/comment-composer.tsx`，两种语言文案及对应测试。内置任务技能和源码映射同步了父任务唤醒契约。

实现提交：`5dd5ccd8b`（触发预览）和 `b6f1e3ec9`（父任务唤醒）。本轮隔离 API/Web 服务已停止，测试数据与证据保留。

## 验证结果

| 范围 | 结果 |
| --- | --- |
| core 协作模块 | 52 文件，808 项通过 |
| views 协作模块与文案一致性，最终代码 | 16 文件，126 项通过 |
| 类型检查 | core、views、Web、Desktop 四包通过；独立修复后 core/views 再通过，最终生产 Web 构建通过 |
| lint | core/views 通过；原有 27 条 views warning 保留，改动文件独立 lint 无误 |
| 服务端预览/FIFO/小队 race | 45 个顶层用例通过；另 1 个输入字节边界用例通过 |
| 生命周期与父任务 race | service 16 个、handler 35 个顶层用例通过 |
| HTTP/路由 race | 6 个评论 handler 用例及 1 个真实工作流路由用例通过 |
| Go 静态与内置技能 | service/handler vet、gofmt、差异检查及 3 个内置技能顶层检查通过 |
| 生产 Web | 最终构建的预览、两条附件/新建恢复、账号工作台共 4 条通过；旧并发冲突用例修正兼容性断言后单独复验通过，共 5 个唯一场景 |
| 视觉 | 最终中英文宽屏/390px 截图检查 94/100，通过；长目标名换行，无横向溢出 |
| 独立复核 | 服务端、客户端、测试与规范均复核；修复 1 个共享草稿导致的旧预览问题，无未解决具体发现 |

上述 Go race 范围合计 104 个顶层用例、209 个含子项结果，均无失败、跳过或 race 报告；不重复累加独立复核的重跑。测试通过 `scripts/go-test-with-agent-cli-guard.sh`，使用 Go 1.26.6 和独立 PostgreSQL 数据库。默认测试未调用已安装 AI CLI、供应商账号或付费 AI。

生产服务通过 `scripts/dev-env.sh up --components api,web --web-mode production` 启动，API 为 `localhost:19000`，Web 为 `localhost:13920`。进程/构建身份见报告目录下 `api.running.json`、`web.running.json`；源码哈希见 `tested-source.json`。浏览器无自动重试。

## 保留的失败与修正

1. 新增回归先重现缺少分组、缺少父任务唤醒，修复后通过。独立复核另重现共享草稿内容变化及相同内容更换草稿 UUID 时沿用旧预览，补完整身份相关性后通过。
2. 首次中文浏览器场景只设置 localStorage；Web 实际使用语言 cookie 和账号偏好。测试改为设置本轮合成账号偏好/cookie，并断言 `html.lang=zh-CN`，未修改产品或放宽超时。
3. 内置说明新增后正文 501 行，超过既有 500 行预算；收紧引用说明并移除多余空行后原检查通过，没有放宽预算。
4. 较宽浏览器复验中的旧并发冲突用例要求响应不含 `status`，但开工前的未提交后端改动已返回 `status:409`。确认该后端文件与开工备份完全一致后，仅让测试接受字段省略或与 HTTP 一致的值，保留真实并发、立即拒绝、输入保留、无自动重试及新键提交断言；单独复验通过。旧无 status 响应仍由 core 规范测试覆盖。

## 限制与后续

- 父任务唤醒沿用普通流程的尽力通知：事务提交后进程崩溃、通知/入队失败或提交结果不确定可能丢失唤醒；原键重放不会补发。此次没有引入持久消息投递或跨任务事务调度。
- 浏览器预览夹具使用真实 API 和明确暂停的合成队列。小队续作由真实数据库服务回归与共享界面测试验证，未据此声称真实模型或供应商执行通过。
- 本轮未重跑完整仓库测试、原生 Desktop 安装包或真实供应商；共享 Desktop 消费端类型检查已通过。没有修改平台路由。
- 运行端附件/Git、通用 provider、非 Linux 隔离、P7 有限代管及 P8 全量验收仍待后续推进。默认创建/执行发布门禁保持原样。

独立报告：[完整复核](research/check-preview-wakeup-2026-10-20.md)；实现报告：[服务端预览](research/implementation-preview-server-2026-10-20.md)、[客户端预览](research/implementation-preview-client-2026-10-20.md)、[父任务唤醒](research/implementation-parent-wakeup-2026-10-20.md)。
