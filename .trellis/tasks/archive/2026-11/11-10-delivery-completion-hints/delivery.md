# M1 完成前交付证据提示 — 交付记录

从业务分支05de761de建立独立worktree，交付普通任务human-only显式提示API：
`POST /api/issues/{id}/delivery-evidence/completion-hint`。
请求只含expected_issue_revision、target_status、evaluation_key（UUID或null），复用现有off/observe政策、私有持久评估恢复、lifecycle与当前身份/审批/制品权限；没有新观察器、执行器或迁移。

## 可用行为
- observe+done类别+原人类key：完整当前授权恢复；原pass且current才提示matching。其他有效恢复为attention，保留原观察、期限和适用性。
- 缺key提示attention/evaluation_required；off、无项目、非done为not_applicable，不检查或泄漏指定key。
- matching与not_applicable均不表示完成许可。所有成功响应enforced=false、usable_for_completion=false；不创建评估、不打包、不排队、不评论、不改状态。
- 自定义done沿用目录锁与Resolve，已归档/非法目标拒绝；NUL等非法key使用已有语法校验在数据库前400。
- 当前撤权、他人key、丢失原绑定、过时任务版本、锁争用和数据库/commit错误为非成功；不伪装为普通提醒。调用者仍限当前人类，原评估身份不转移。
- 有评估时checked_at与applicability.checked_at相同；事务结束后的完成操作不得把提示当许可。原始评估GET在policy关闭后仍按旧合同恢复stale；本提示关闭后不消费评估。

## 最终验证
冻结13份文件（9份修改的产品/测试/合同+4份复用核心），源码零漂移：

| 最终范围 | 普通 | race | 顶层测试 |
|---|---:|---:|---:|
| Handler |325|325|76|
| 真实认证路由 |103|103|37|

所有最终测试0失败/0跳过。Handler325=89新增提示+236旧政策/观察/制品/审批/删除清单；router103=23新增+80旧合同。具名计数包含子测试，普通/race和多轮不能相加为独立覆盖。合同50=service37（21技能+16本次正则选中的既有Resolve测试）+状态11+迁移静态1+runtime扫描1。

全后端build/vet、sqlc生成幂等、gofmt/diff通过。真实路由使用JWT/password/PAT及实际任务凭据打包，核对tar内容/SHA、持久行、路由重建、实际到期、当前撤权和锁冲突；核对旧普通/批量/创建自定义done仍工作。完整issue/status catalog/project/policy/evaluation/approval/execution/task/comment快照验证零额外业务写入。

4份失败运行全部留存：两轮router夹具（状态key过长；直接插入任务未同步编号计数器）、NUL真实400/503行为red，以及补测期间短暂Go括号格式错误。夹具/格式失败不算行为red。最终新库普通/race验证NUL修复及各分支撤权。详见verification与research/failures、独立审查、源码/测试清单。

## 本包限制与下一包
这是API提示基础能力，尚无UI/CLI自动展示、skip、新policy模式或任一完成入口阻断。独立协作普通POST明确unsupported；普通协作验收与R38管理例外不被此API替代。全入口清单见research/completion-writers.md。

强制门禁须覆盖普通/批量（目前逐条事务）、move/改项目、创建即完成/评论转子任务、GitHub/VCS、协作精确验收/自完成/R38以及自定义done；必须在实际状态事务内重验policy/identity/期限/SHA/artifact/receipt，不能消费历史current标志。skip严格边界已向用户提问但尚未收到回答；原评估者与完成人/机器/协作跨账号共享、create-done及已done修改语义仍待决定，见research/enforcement-decisions.md。

M2保持确定性快照→父总预算→能力/负载调度；M3保持NFR/模型评测/供应链/模板与知识治理。父M1和六业务均继续开放。AC43/E5/H3a/知识预算与停止规则不变，真实模型调用0，无推送/部署/外部消息。

专属容器multica-completion-hints-20261110-pg停止前0连接，10库和全部日志保留。未借用/清理历史环境。PG17amd64/hostarm64仅作功能证据；没有性能、浏览器、全仓、部署或真实供应商验收声明。

独立trellis-check最终PASS、零未解决阻塞；独立核对9份最终日志、4份失败日志、源码13/13及源码派生测试清单。sqlc后未出现源码或generated漂移。

实现提交ca0c8efd3；本子任务可归档，随后单独记录安全本地集成。

## 本地集成完成
实现ca0c8efd3、归档d3b0b17db、Session151交接609219ed3已安全快进合入codex/cross-workspace-collaboration。1,048个不重叠既有路径字节/权限/存在状态全部保留；4个重叠文档三方合并，旧journal作为完整字节前缀保留，父任务旧本地字段和受控制品child链接保留。HEAD中13份冻结文件全部匹配；工作区仅spec/index保留用户原有额外条目，产品/测试/技能/复用helper全匹配。见research/local-integration.json。未夹带已有修改，未推送或部署。

## 变更文件
- server/internal/handler/delivery_completion_hint.go 与同名_test.go：提示与权限/失效/副作用合同。
- server/cmd/server/router.go、delivery_completion_hint_integration_test.go：路由和真实认证验收。
- server/internal/service/builtin_skills/multica-creating-agents/SKILL.md及references/creating-agents-source-map.md：提示消费与非授权边界。
- .trellis/spec/server/delivery-completion-hints.md、delivery-policy-evaluations.md、index.md：可执行合同及复用关系。
- 本归档、父任务、六业务计划和Session151：证据/未决语义/交接。

简化点：复用既有评估恢复与状态校验，无重复观察器、执行引擎、SQL/迁移或依赖。剩余风险：提示只是事务快照；全writer原子阻断、跨主体证据、skip和客户端提示尚未交付。
