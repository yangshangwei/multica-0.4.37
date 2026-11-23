# 合成案例：程序代码漏洞

## Issue body

某个受工作区边界保护的读取操作出现授权检查缺口。请在不构造可用攻击载荷的前提下确认可达条件、影响和最小修复，并补充权限拒绝回归测试。

## 注入信息

- 触发材料：本地测试 fixture 和脱敏请求形状
- 受影响面：一个 workspace-scoped handler/query
- 目标：禁止真实账号、生产数据、凭据和外部系统验证

## 预期路由

`security-reviewer → diagnostician (if cause/trigger is unknown) → implementer → qa-engineer → code-reviewer + security-reviewer`

## 通过条件

- 报告授权检查发生在副作用之前，并说明租户/工作区范围。
- 无修复时的最小失败测试在修复后通过，且实现者没有自行完成 QA。
- 结论区分实际漏洞和纵深防御建议，保留未确认边界。
