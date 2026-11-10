# 完成门禁当前交接

## 当前阶段（2026-11-11）

D1–D5 已逐项确认，用户明确授权后续推荐执行；D6/D7 与技术合同已按该委托收敛，最新 PRD/design/implement 为准。独立 P0 readiness PASS，task 已 in_progress，先跑修改前回归再唯一核心下沉；P1–P4 仍待实施及最终全入口验收。v1 skip_supported=false，不能宣称可用 skip 或完整 M1。专属验收环境与日志见 progress/research/runs，私有 registry 不提交。

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
