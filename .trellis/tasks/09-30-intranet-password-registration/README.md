# 内网用户名密码注册与设备账号迁移

状态：in_progress（待合并与部署验收）。2026-10-01 开发和本地验收完成：9171 项 TS 测试、Go race/vet、生产构建、126 项全站 E2E（另有 27 项条件跳过）与 4 项密码专项通过。首版仅本地/自托管运行环境。

## 已确认的产品流程

服务器地址 → 用户名、密码、姓名注册 → 自动登录 → 现有欢迎引导 → 用户自行创建工作空间 → 连接运行环境。

不需要注册邀请码、邮件验证码、单独激活或管理员批准。创建自己的工作空间不需要管理员分配；已有工作空间的访问权限保持原有规则。

开发工作树：`/Volumes/artisan/code/2026/multica-password-registration`，分支 `feat/intranet-password-registration`。最新结果见 [verification.md](verification.md)。

## 文档导航

- [prd.md](prd.md)：需求、范围、验收标准。
- [ux-design.md](ux-design.md)：页面流程、状态、文案、异常处理。
- [design.md](design.md)：技术方案总览、取舍和接口契约。
- [architecture.md](architecture.md)：模块边界、数据模型、认证与安全设计。
- [security-contract.md](security-contract.md)：评审后的凭据撤销矩阵、迁移时间窗、限流及服务器切换契约。
- [research/implementation-readiness-review.md](research/implementation-readiness-review.md)：代码复核发现及处理去向。
- [migration-rollout.md](migration-rollout.md)：旧设备账号迁移、发布与回滚。
- [implement.md](implement.md)：实施顺序、文件落点、验证命令。
- [test-spec.md](test-spec.md)：验收矩阵与回归测试。
- [research/current-state.md](research/current-state.md)：当前代码依据与实现前检查点。

- [research/security-review.md](research/security-review.md)：独立安全复核及竞态测试证据。
- [research/password-acceptance.md](research/password-acceptance.md)：密码专项浏览器与 Electron 验收。
- [research/e2e-regressions.md](research/e2e-regressions.md)：既有全站测试契约修正。

## 阅读说明

产品和工程方案已评审并实现。方案文档保留规划时的候选路径；实际文件、迁移 462–464、验证结果及尚未执行的部署验收以 verification.md 为准。

本任务是一个端到端功能，按 implement.md 的阶段推进，暂不拆成容易产生认证契约漂移的独立子任务。JSONL 为后续实现和检查提供上下文，不自动启动代理实现。

## 暂不包含

用户管理后台、企业 SSO、邮件服务接入、邀请码系统、自动合并历史账号、移动端密码登录、新工作空间引导。已实现无邮件的运维密码恢复命令；完整管理员 UI 留给后续任务。
