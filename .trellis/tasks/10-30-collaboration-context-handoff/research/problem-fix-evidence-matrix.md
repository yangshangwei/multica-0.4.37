# 首轮问题→修复→证据→限制映射（规划模板）

此表保留原规划快照。2026-10-31 的实际闭合证据和剩余限制以 [续作验证记录](../../10-30-collaboration-fix-closure/verification-2026-10-31.md) 中的映射为准；正式 E0 已通过，E1 受 429/503 阻塞，后续未运行。

| 问题 | 修复提交/计划改动 | 验证证据位置 | 剩余限制 |
| --- | --- | --- | --- |
| A1 附件历史/机器预算 | 7bb077987；补实际 runtime budget selector | 待最终 HEAD Go/DB/integration | 可选文件可省略；引用超限拒绝 |
| A2 escaped comment | d84f217a9 | 最终 parser/service/integration | 解码后业务上限不变 |
| A3 local path | b34701756 | raw/role-cli-current-head.txt | 只影响协作 CLI |
| B1 migration prefix | 重命名未发布 745 | raw/migration-prefix-lint-current-head.txt + fresh/upgrade | 已发布 migration 不可改名 |
| B2 reconsent copy | 0ef4c7858、4c2140b54 + can_consent-aware copy | review-b + Web/Desktop | NULL boundary/部分配置保持 Switch |
| C1 role/step | e6ed3d1c3 | raw/role-cli-current-head.txt + E3 | delegation 正文仍需结构化 |
| C2 smoke allowlist | fd3836da2 | raw/authorizer-current-head.txt + exact E0 | 真实账号/供应商可能环境阻塞 |
| C3 return form | 4b0cbfde7 + recipient e2e evidence | review-c + final e2e | Desktop native evidence separate |
