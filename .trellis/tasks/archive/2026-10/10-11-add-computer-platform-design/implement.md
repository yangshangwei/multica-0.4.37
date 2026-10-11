# 执行计划

## 授权与边界
用户在本轮明确批准现有设计进入实现。最小行为缺口位于共享 ConnectRemoteDialog 与公开配置链路；保持现有安装器和两步流程。

## 顺序与职责
- [x] 审阅静态设计、当前代码与规范；补齐 PRD、技术契约及上下文。
- [x] 配置实现：server config / core schema+store+initializer、环境转发和文档，覆盖新旧/畸形响应。
- [x] 弹窗实现：packages/views/runtimes/components/ 与 runtimes 中英文文案；命令、安全地址、真实连接、复制与可访问性回归。
- [x] 集成检查：配置→shared UI；连接确认、旧电脑重连和断线；修复检查发现。
- [x] 自动验证：相关 Vitest；core/views/web/desktop typecheck；相关 ESLint；Go config 测试及 go vet。
- [x] 浏览器检查：初始、三系统、地址错误、帮助、连接成功、窄窗口；与原图对照，记录 visual verdict。
- [x] 更新规范与 verification.md，复核 diff，只提交本任务相关文件并记录会话。

## 验证与回滚
使用仓库现有 pnpm/go/Playwright 工具，不添加依赖；测试不能执行真实用户 Agent CLI。API 配置为可选字段，无 schema migration；整体撤销功能提交即可回滚。若完整环境不可用，记录实际验证范围与具体阻塞，不声称三系统真机验证通过。
