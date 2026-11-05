# 发布与数据库测试可信度

## 目标
修复审计项 2/3，发布验证有独立测试依赖，必需数据库初始化失败不再显示测试通过。

## 需求与验收
- B1：Release verify 使用显式、相互一致的 PG17/pgvector 数据库、健康检查、Redis 测试配置；迁移先于完整 Go 测试，发布依赖图不变。
- B2：scripts/test-go.sh 在调用 go list/任何测试前拒绝缺失/空白 DATABASE_URL，不再让较早的 legacy helper 接触默认开发库。
- B3：handler/router 必需 DB TestMain 对缺失、畸形、不可达 URL 非零退出，诊断不得包含连接字符串/合成口令；连接探测有界，正常生命周期不受短超时影响。
- B4：testutil 无 URL 时只运行纯函数测试，数据库用例显式跳过且不回落 localhost；已提供 URL 后解析/连接/seed 失败非零退出。
- B5：独立负例测试编译实际测试二进制，使用临时不存在的 Unix socket，隔离 PG*/DATABASE_URL，不连接任何现有库；修复前失败、修复后通过。
- B6：Release 配置回归、数据库前置条件回归和相关文件变更进入主 CI 的 backend 路径/执行；Node 使用显式版本。

## 边界
本批不重构全仓 test inventory 或所有 per-test skip；不声称完整 CI/发布已跑通，不触发标签或远程发布。
