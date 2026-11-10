# 完成门禁当前交接

## 当前阶段（2026-11-11 最终验收后）

v1全完成入口已实施，公共enforce已开启并通过实际认证/签名全链验收。最终110源码冻结：handler普通/race各620、router各166、service各272、migration各62，实际删除留证补充各4，全部0fail/skip；52合同、build/vet/sqlc134幂等和客户端Core63/Views91/typecheck/lint通过。完整结果见 delivery.md、verification-2026-11-11.json、research/final-source-manifest.json、final-named-inventory.json 和最终独审。

v1产品与最终验收已在18b743291提交并安全本地集成；c9a070551/fb0558881两个增量均为其祖先。主目录1,057个不重叠dirty路径原样保留、15文档受控合并、110源码匹配；集成记录见research/local-integration-20261111.json。专属PG零连接后停止，65数据库与日志保留。本已验收子任务已归档，journal Session153已记录；不得把下方历史P1/P3进行中段落当实时状态。

M1与六业务父任务仍in_progress，M2/M3仍planned；v1 skip_supported=false、没有真实可豁免检查，不宣称可用skip。AC43/E5/H3a/知识预算与停止规则保持，供应商调用0、未推送/部署/外部消息。任务是否归档与精确集成头以最终 task.json/归档路径及research/local-integration-20261111.json为准。

## 历史进行中阶段（保留当时记录）

任务 in_progress，用户已逐项批准 D1–D5 并明确「后续如果有选择，请默认按照推荐执行」。D6/D7/技术推荐已收敛，不再问流程或选择许可。主线目标是完整完成事务门禁；未完成前持续实施，子包不算 M1 或六业务完成。

独立分支现有本轮提交 c9a070551（P0唯一service观察/authority下沉）与 fb0558881（D6客户端13文件）。尚未本地集成主目录，主目录仍1ce4e8122且并行修改在继续；最后集成必须做新实时私有保护，不能覆盖旧快照后他人变更。

P0最终handler普通/race各339、router各103、合同50/build/vet/独审通过。客户端Core63/Views91及两包typecheck、scope lint独审通过；全Views既有27warnings未压制。P1普通授权handler迭代22、真实router中间编译快照11、迁移修夹具后7通过。P2普通writer迭代34通过，后来新增父通知/移入与迁移测试仍待最终跑。

当前并行所有权：check_resume_evidence负责P1 exact schema/typedexpiry/late-consumption SQL修复和独审，含query/generated与相关tests；implement_collaboration_completion负责协作/R38及P1 core union（delivery_completion.go/authorization.go）接合；implement_authorization_router_tests负责VCS/GitHub/systemwriter/792–793信任revision及真实signed测试。新core must compile before shared tests；可冻结test-c二进制单独跑real红绿，必须绑定原compile源码而非冒充当前全源码。

P1独审实际红：弱schema7case（8fail含parent）→严格readinessgreen17；typed expiry非UTC/纳秒两case（3fail含parent）已修入口，最终纯/完整回归待；late-consume两case在真实最后INSERT前过期仍成功（3fail含parent），当前修SQL消费窗口并由coreowner最后复核consumer。原历史失败全部保留。

P3实际红：Github/Forgejo无授权完成（3fail含parent）；普通协作self/submission及R38无授权完成（4fail含parent）。已冻结对应binary/原始日志，绿/完整正路径仍待。协作必须显式exactordinary-child证据，不能伪造parent task；self prepareDelivery还会绑定附件，已授权拆只读validate与真正bind，gate失败不能部分写。全部writer未验收前公共enforce必须保持关闭。

私有验收registry `.omx/private/enforcement-implementation-20261111/environment.json`，私有launcher `run.py`（不提交）只操本包新PG `mc-completion-enforcement-20261111-315f96e0`，loopback55757，state/log/binaries位于 `/private/tmp/multica-completion-enforcement-20261111-y9c0v8ro`。每包/每normal-race新库；原始日志和JSONrunrecords保留。已有旧database/容器从未启动/借用。root唯一分配/迁移，子代理按exactpermit测试。

继续：收口P1late expiry/schema修复；完成P3VCS+协作/R38真实正负链；真实managedtask grant+consume正反验收；跑P2旧兼容/parentnotifications/789–793迁移；独审修复后P4最后开放policy enforce及evaluation/hint兼容，actualpublicauth端到端所有writer；最后全库normal/race、build/vet/sqlc/format/TSchecks和具名源码冻结独审，Lore提交、安全本地集成、spec/六业务父状态/journal。v1 skip没有可豁免硬检查，明确unsupported，不建无用skip引擎或宣称已提供skip。供应商调用0，AC43/E5/H3a/知识停止预算保持，不推送/部署/外部消息。

## 逐次历史决定（2026-11-10）

用户已针对 D1 明确选择 A：人类验收并承担最终责任，可一次性授权指定人、agent 或 VCS 代办；私有评估不共享，最终事务重验当前双方权限与精确绑定。随后针对 D2 选择 A：仅当前 owner/admin 有理由地批准 skip，指定本人或其他消费者，精确绑定、未消费可撤销、同事务一次消费、原回执只读恢复；最长 24 小时且不超过绑定证据有效期，unknown/缺失/过期/撤权仍拒绝。具体合同见 decision-register.md。

D3 随后也已选择 A：enforce 项目拒绝创建即 done，含自定义 done、普通创建和评论转子任务；按实际项目（含父继承）判断，off/observe 保持原行为，不新增创建豁免。

D4a 已选择 A：非项目变更只有非 done→done 触发新验收；已 done 的同类状态切换、内容/附件/标签/属性/迭代等编辑保留原冻结验收事实，不新增消费或宣称修改后对象已重新验收，既有动作权限保持。

D4b 已选择 A：当前 owner/admin 本人有理由地移出 enforce/解除关联；任一侧 enforce 时已 done 先重开，迁移与完成分开；以后按目标政策新准入，旧授权不沿用；删除 enforce 项目前先显式关闭，含协作任务仍拒绝删除。D1–D4 已确认，均尚未实施。

下一步收敛 D5 协作源政策、跨账号参与权、精确 submission/自完成与 R38；随后 D6–D7 及明确的可 skip 检查清单。两分支续会 HEAD 均为 1ce4e8122，源码和历史证据复核见新增 `research/*recheck-20261110.md`、`resume-head-verification-20261110.json` 与 `resume-check-20261110.md`。本任务仍为 planning，尚未实施。

以下为此前规划检查点交接记录，保留其当时的未答状态；当前决定以本节及 decision-register.md 为准。

## 前次检查点记录

本任务仍为 `planning`，不是强制门禁交付或 M1 完成。业务分支恢复基线为 `a37bee7cd1c05122383a6c40e157ef9ed5890bf4`；源码无新增产品修改。

已完成：加载用户列明的规划、父任务、规格、最新/前序归档与 Session149–151；核对 18 个提交祖先、提示包 13 个 HEAD 文件、完整入口研究的 45 个 HEAD 文件，以及四包 169 份历史运行记录。记录数量不能充当本轮新增测试数。全部原历史失败保留；未启动历史数据库或真实供应商验收。

新 worktree：`/Volumes/artisan/code/2026/mc-delivery-completion-enforcement-20261110`；分支 `codex/delivery-completion-enforcement-20261110`。主目录已有变更另行保护；实际安全本地集成结果以 `research/local-integration.json` 为准（生成前不宣称已集成）。

首先等待 D1：最终验收责任与代办关系。已异步给出三种候选，推荐“明确人类验收 + 一次性受限代办”，但没有收到答复，不能把推荐当批准。完整 D1–D7 状态见 `research/decision-register.md`；owner/admin 管理项目政策已经确定，不重复问。

源码研究补充了协作项目编辑、删除项目后 detach、已 done 内容/附件/标签/iteration 等独立变更入口。它们是否重验由 D4 的业务规则决定，不能只覆盖普通 move。原观察器在 handler，而多个实际 writer 在 service；复用必须进入原事务，不能复制观察器或信任外部预检。

回答收敛后再确定最小实现包和验收矩阵、启动任务并派 Trellis implement/check。源码或本包范围有变化时重新冻结与验证；专属 PG、每 DB 包及普通/race 新库、真实认证/实际 tar、失败留存和独审要求继续有效。

M1 仍 in_progress；M2/M3 planned。六业务父任务不关闭。AC43、E5（19已用/8余）、H3a（12/25已用/13余）、知识（2/17已用/15余）的停止/预算与原始失败不变；本轮供应商模型调用 0。没有 push、部署或外部消息。
