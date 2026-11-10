# M1 完成前交付证据提示

## 目标与授权
从05de761de推进观察→提示→全入口强制门禁的最小可独立验收增量：有权人类在普通任务完成前显式取得结构化证据提示，知道证据是否仍匹配。用户已授权任务、规划、实施、Trellis子代理、本地提交和安全集成。本包只交付API提示基础能力；不会把提示当成状态写入许可。

## 已核实背景
此前受控制品、证据观察、持久委派、政策及持久评估均已完成。最新26份冻结源码与HEAD/工作区一致，历史最终及失败日志核对通过。当前项目政策仅off/observe、固定算法、stage=done，owner/admin管理。持久评估限原人类恢复且usable_for_completion=false。独立协作有自己的提交/验收/管理员例外；自定义状态继承不可变category。

## 需求
- R1 提供human-only、显式只读完成前提示API。检查当前主体、任务版本、当前项目政策和目标状态category，普通任务自定义done与内置done同义；独立协作明确拒绝此普通入口。
- R2 observe下done目标可以提交原人类评估key。只复用既有恢复与完整当前artifact授权，输出原观察和当前适用性；只有原pass且当前current提示matching。缺key为attention；失效、过期、hold、unknown均attention，绝不升格或续期。
- R3 无项目、off、非done目标为not_applicable；不读取或附带指定评估。off不等于skip；任何结果都不是完成/发布许可。政策开启observe后由客户端显式调用启用提示；退回off仅关闭此提示的证据消费并保留历史。
- R4 缺失/他人key拒绝读取；撤权、失去原绑定、版本冲突、锁争用、数据库/commit故障按非成功响应，不能以attention吞掉授权/基础设施错误。严格body拒绝自报verdict/skip/actor。
- R5 任务/项目政策读取和评估复核在同一个短事务；响应只代表检查时快照。无持久行/评论/队列/执行/状态变更。旧客户端、全部完成writer行为保持。
- R6 落盘全部完成writer清单、完整强制门禁未决业务合同和明确未覆盖范围，父M1/M2/M3及真实模型门保持开放。

## 验收条件
1. 专属PG新库，真实JWT/password/PAT认证、实际受控tar与SHA、持久评估后调用提示；原观察/账本/任务状态/执行行不变，重建路由可恢复。
2. built-in/custom done、非done、off/no-project、缺key、未知/他人key、跨issue、机器/伪造身份、重复/未知/过大JSON和旧版本。
3. 原hold/unknown不提升，期限不延长；当前撤权、policy CAS/ABA/off、项目/任务/PR/制品绑定变化、锁争用、commit失败/歧义、独立协作拒绝。
4. 复用旧政策/观察/制品与审批回归；handler/router普通和race，build/vet/sqlc幂等、格式及技能合同，冻结源码与实际测试清单，独审PASS；保留所有失败。

## 范围外及待后续决定
无UI/CLI自动提示、无新增policy mode、无skip实现、无任一完成入口阻断、无新观察器/执行器/迁移/依赖、无远程Git、新模型预算、推送/部署/消息。skip推荐仅owner/admin、理由、单任务+policy+提交/制品+期限、一次消费，已问用户；它不阻塞本提示包。全门禁还需明确原评估者与完成人关系、非工程/创建即完成的证据或skip准入、协作发起者验收与R38管理员例外的优先级。
