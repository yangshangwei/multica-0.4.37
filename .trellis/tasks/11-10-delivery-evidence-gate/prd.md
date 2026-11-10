# M1 交付证据观察增量

## 授权与目标
用户授权下一阶段必要子任务、规划、实施、Trellis implement/check、独立环境验收及本地Lore提交。基线 a3b722a47，主工作区保留。本包落实既有 observe→提示→阻断的第一阶段；不是完整M1或强制完成门禁。

## 需求
提供只读观察评估API，复用项目/issue lifecycle身份与受控制品回执。固定版本政策检查当前issue/project版本、选定候选提交和制品摘要、证据生产者、审批人、当前权限、结果和有效期。pass/hold/unknown明确区分，不接受客户端自报成功或身份。不得把声明的source_revision当成独立Git HEAD证明。

## 验收
1. 真实受控打包成功后评估能返回绑定policy、issue/project版本、提交/制品/spec/receipt摘要、producer/task/generation、observed_at与valid_until的结果；不产生Pack、状态变更或任务排队。
2. 选定候选SHA不匹配、缺失证据、过期、failed、unknown、篡改绑定不能pass。若没有服务端已观察到的当前PR head，源码新鲜度unknown，不能给整体pass；当前PR head仅代表DB已观察事实，不声明远程实时HEAD。
3. 当前凭据、成员、原生产者/发起人/runtime owner/审批人撤权拒绝或hold；token及伪造header不能扩权。允许已完成任务保留证据，但不豁免当前授权。
4. 事务内锁定issue、project、已观察PR、身份及receipt；修改先行则冲突/hold，评估先行则阻止其穿过同一事务。结果仅表示observed_at快照，不是可复用授权票据。
5. 新API不改变旧完成/更新/审批响应；默认无强制门禁，无UI/CLI改变。旧入口兼容回归。local-synthetic，真实模型调用0。
6. 专属PG库handler与真实认证router普通/race、全后端build/vet、格式及适用sqlc幂等；保留原始失败、冻结文件与日志hash；独立review。

## 后续
完整项目可配置policy、强制done所有写入路径、持久化评估审计、持久委派回执、M2/M3独立交付。AC43/E5/H3a/知识原停止规则不变。
