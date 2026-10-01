# 现状、复用点与需求来源

核查日期：2026-10-01。本页为源码研究索引，不代表运行环境已验证。

## 1. 用户确认记录

- 已实现桌面注册，希望超级管理员统一管理终端、任务执行反馈、统计及监控。
- 确认企业内部和多客户都需要，先做内部版。
- 确认共用账号/密码/认证，平台角色和工作空间角色分开。
- 确认首期规模 100–1000 台。
- 当前授权为新建 Trellis 任务并完成设计与任务分解；未要求实施代码。

## 2. 专项源码研究

- [认证与权限依据](auth-current-state.md)：token类型、JWT转换、密码版本、恢复、路由与角色现状。
- [终端与执行依据](terminal-current-state.md)：device/install/daemon/runtime标识、心跳、claim、cancel/ack和统计。

这些研究中的“缺口”是本轮拟新增能力；不要把方案文件中的接口名搜索不到当成已存在实现。

## 3. 额外复用点

| 位置 | 已有能力 | 本设计边界 |
| --- | --- | --- |
| `server/internal/handler/dashboard.go:13` | 工作空间/项目按日、模型、智能体、运行时长与失败分析 | 新增授权聚合和脱敏投影，不在浏览器遍历所有空间 |
| `server/internal/handler/dashboard.go:32` | 当前成本按客户端模型价格表估算 | 统一平台费率前不声称是实际账单；P1可仅展示Token |
| `server/internal/handler/client_usage.go:48` | 用户/安装/UTC日活跃upsert | 日活不能等于实时在线或可信安装认证 |
| `server/pkg/db/queries/client_usage.sql:1` | 每用户/安装/日期一行，workspace为当前快照 | 不具备组织归属历史事实，不能据此重建所有提交归属 |
| `server/internal/metrics/business.go:43` | 排队、运行、终态、Token/费用等指标 | 复用低基数监控，不把task和安装ID无限加label |
| `server/cmd/server/router.go:1334` | 受保护实时指标接口 | 后台经后端授权读取摘要，不将抓取密钥交给浏览器 |
| `packages/core/client-usage/install-id.ts` | 现有统计安装UUID | 与设备身份、受管理安装密钥分开 |
| `apps/web/app/(auth)/login/page.tsx:79` | 同源next跳转和已有登录流程 | 新后台验证无workspace，避免被引导流程阻拦 |
| `.trellis/spec/desktop/frontend/intranet-updates.md` | 已有内网桌面发布/更新通道 | 不重复建设下载服务；控制批次仍属P2 |

## 4. 相关任务与既有规则

已有注册需求：[内网注册PRD](../../09-30-intranet-password-registration/prd.md)。保留无邀请码/无需审批、注册自动登录、自建工作空间；后台管理不擅自改变员工主流程。

遵守 CLAUDE.md 的 package boundaries、Query状态所有权、API drift防御、UUID边界、无外键与并发索引要求。已有spec中标注“To fill”的文件不是已建立的额外规则，以根CLAUDE及已填充spec为准。

## 5. 已排除的假设

不假设当前已有平台管理员；不假设物理设备数等于runtime数；不假设桌面在线等于可执行；不假设取消业务任务等于停止执行；不假设所有本地日志/文件已经上传；不把客户端自报ID、版本或hostname当授权依据。
