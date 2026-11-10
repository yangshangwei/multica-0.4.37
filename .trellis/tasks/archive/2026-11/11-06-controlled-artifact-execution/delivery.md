# 受控制品执行交付 — 2026-11-10

本批实现审批绑定的本地发布候选制品打包。HTTP创建时冻结规范化规格及可信输入清单；人工确认精确摘要后，领取事务先落unknown，只有确定提交的唯一执行者可运行无shell的确定性tar打包。成功回执与旧审批executed同事务；不确定结果503/unknown，不能自动重跑。

提供受限详情、当前凭据/成员/任务/审批人权限复核、一次性幂等、工作区删除保留审批与回执、旧自报接口拒绝绕过。Linux/macOS采用no-follow目录描述符和文件身份验证；其他平台不启用。默认未配置目录时关闭。

## 验收证据

| 范围 | 最终结果 | 证据 |
| --- | --- | --- |
| 纯规格/文件系统macOS | 普通及race各78具名PASS，0fail/skip | research/packager/result.json |
| Linux实际纯包执行 | 78PASS，0fail/skip；Windows仅拒绝实现交叉编译 | research/packager/packager-linux-native02.json |
| 实际SQL及迁移 | 状态/并发9race、索引失败恢复3、规范4通过 | research/database/verification.json |
| Handler含旧审批兼容 | 独立新库普通/race各84PASS，0fail/skip | research/final-handler-normal.json、final-handler-race.json |
| 真实路由/认证 | 独立新库普通/race各23PASS，0fail/skip | research/final-router-normal.json、final-router-race.json |
| 认证兼容、技能合同、runtime扫描 | 48、21、1PASS | research/*compatibility*、builtin-contracts-01、runtime-lookup-contract-01 |
| 构建/静态 | 全后端build/vet0；gofmt无差异；sqlc131文件重复生成一致 | research/final-build.json、final-vet.json、final-sqlc.json |

具名计数包含子测试，不将不同轮次重复相加为独立覆盖数。原始失败保留；首次测试设施失败与实际产品red→green分列在各lane记录。

## 原验收矩阵对应

1. 创建冻结、幂等及严格解析：ArtifactSpec + ControlledArtifactLifecycle/SameKeyConcurrency。
2. 真实身份及读权限：RouterTaskIdentity/Scope/Restricted/Password/Managed、AgentReadRechecksInvocation、ActualAdmissionBeforeRevocation的独立发起人/执行者子场景。
3. 决策/取消/撤权竞态：AuthorityLocks/CredentialRevocation/SeparateApproverRevocation、ActualAdmissionBeforeRevocation、TaskCancellationBeforeClaim。
4. 唯一实际副作用：OnePhysicalInvocation及ActualAdmissionBeforeRevocation的提交前0Pack/提交后1Pack。
5. 冻结绑定漂移：StoredBindingDrift、纯规格及文件系统输入/根身份矩阵；回放旧规格同样校验。
6. 过期/撤销/unknown：Rejections/JWTExpiredAtWrite/UnknownNeverRetries、真实认证后撤权。
7. 文件边界：ArtifactPackageInputBoundaries/Mutation/RootAndOutput/IOUncertainty及输出inode替换。
8. 确定性：ArtifactPackageDeterministic，独立内容和USTAR头检查。
9. 提交歧义：CommitUncertainty真实commit前后注入；两成功写入间失败回滚；FinalizationLockConflict为503unknown，保留1Pack且重放不再调用。
10. 旧行为/删除：旧审批测试、WorkspaceDeletion八状态、CreateBeforeWorkspaceDelete、生产CancelTaskInTx真实路径。

C1/C2的直接SQL锁辅助记录不冒充完整HTTP证明；最终另有实际Execute提交屏障、独立发起人/当前执行者/审批人/发起源权限及PAT/任务token测试。安装绑定使用真实中间件验证后的epoch/撤销/退役/组织映射变化，未伪造认证上下文。

## 清理与范围

任务独立PG17容器已停止，9个数据库及日志保留；停止前任务库连接0。未触碰其他业务环境，真实模型调用0。代码未添加依赖，未引入shell/网络执行器，没有部署、发布、UI或CLI typed-flow实现。现有审批UI未提供新规格确认界面，调用者需使用独立详情/API流程。

全部后端测试不是全仓验收；真实Windows执行不支持，其他M1证据门禁/持久化委派、M2/M3及真实模型系列仍开放。独立最终审查结论见research/independent-review.md/json。
