# M1 交付证据观察增量 — 2026-11-10

新增 human-only `POST /api/issues/{id}/delivery-evidence/evaluate`。固定 `delivery-artifact-observe-v1` 政策，从现有任务/项目、受控制品审批和回执计算当前观察结果；绑定版本、提交/制品/spec/receipt摘要、生产者及有效窗口。真实打包成功与当前数据库已观察PR head一致才可能pass；缺失/unknown/过期/绑定变化/撤权不能取得错误绿灯。

不新增执行引擎、数据库表或依赖。复用现有身份、lifecycle与artifact锁及规范化合同。旧客户端done/更新/审批行为保持。观察结果不赋予完成或发布权限，也未持久化新观察回执。完整项目政策、全入口阻断及持久化委派仍待后续。

## 最终验证

| 范围 | 普通 | race | 说明 |
| --- | ---: | ---: | --- |
| Handler | 140 | 140 | 新观察56 + 原受控制品/审批84；四类数据库均独立新建 |
| 真实认证路由 | 34 | 34 | 新观察11 + 原路由23；JWT/password/task/PAT真实认证 |

均0fail/skip，具名计数包含子测试，不累加轮次为独立覆盖。实际执行受控tar并核对内容与SHA，观察本身零额外打包/排队/业务写入；撤权分离身份、PR/VCS phantom插入、摘要篡改重算、commit前后失败均有具体测试。

全后端build/vet exit0；内置技能合同21、runtime直接查询扫描1通过；gofmt/diff检查及6文件冻结hash一致。SQL/schema/generated未变，sqlc无需重新生成；无前端源变更，不声称TS/浏览器/全仓验收。

`verification-2026-11-10.json`索引最终记录，`research/runs/`保留全部32次运行/迁移记录，原始日志在任务私有目录。独立审查见`research/independent-review.md/json`。

## 保留的失败与修正

- router-red01与handler01：macOS `/var`临时路径符号链接遭已有no-follow检查拒绝；改为任务私有canonical TMPDIR，未放宽产品边界。
- handler02：JSONB空白格式不能用原始文本比较，改比较值且保持摘要/版本断言；删除共同observer成员先返回403，精确断言修正。
- handler03：分离originator夹具必须同步accountable，既有数据库约束要求，失败发生在新API前。
- final-router-race第一轮：PAT中间件异步last_used写与NOWAIT锁竞争，正向得到409。通过同一请求body边界等待这次实际写入提交后测试，不重试、不把409当pass。
- 首轮handler140普通/race在凭据检查时钟顺序修正前通过，作为修正前证据保留。最终以02新库记录为准。
- 实现检查修复了项目非空ID/null版本解析风险和PAT验证早于observed_at的时钟空隙；不以设施失败冒充产品red→green。

## 边界与下一步

PR head仅为数据库已观察事实；同优先级不同head为unknown，不能证明尚未接收的远程推送。四张PR/link表短暂SHARE NOWAIT防phantom；既有PAT辅助写也可导致瞬时409，未做吞吐承诺。15分钟窗口沿用制品准入期限作为本政策保守观察窗口，不抹除历史成功。

专属PG容器停止前任务连接0，10个数据库与日志保留；真实模型调用0，未消耗/解除AC43/E5/H3a/知识预算或停止规则，未推送/部署。

后续按父计划：项目可配置证据政策与持久评估、所有done入口阻断；持久化trigger_outcomes回执；M2确定性快照/总预算/调度；M3质量/评测/供应链/模板知识治理。父任务及六项业务仍in_progress。
