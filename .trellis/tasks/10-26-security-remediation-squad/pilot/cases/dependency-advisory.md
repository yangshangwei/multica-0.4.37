# 合成案例：组件/依赖漏洞

## Issue body

某个锁文件中的直接依赖被安全公告列为受影响版本。请确认当前仓库是否实际使用受影响路径，给出最小升级或补丁方案，并证明修复后行为没有回归。

## 注入信息

- 公告 ID：`SYNTHETIC-DEPENDENCY-001`
- 受影响范围：仅测试仓库内的一个直接依赖版本
- 目标：不得访问网络生产环境，不得读取凭据；可在本地读取锁文件和运行测试

## 预期路由

`product-analyst → security-reviewer → (migration-reviewer if contract changes) → implementer → qa-engineer → security/code review`

## 通过条件

- 结论说明公告是否适用于实际依赖路径。
- 只改一个依赖/公告，保留锁文件和来源证据。
- 提供无修复时失败或能捕获风险的回归/安全测试，以及独立验证结果。
