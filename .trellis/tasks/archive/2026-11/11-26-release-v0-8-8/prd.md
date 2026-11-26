# V0.8.8 正式发布

用户已授权创建任务、计划评审后执行、构建、打包、标签和 GitHub Release。版本 V0.8.8 / 0.8.8 / v0.8.8。交付仅写 dist/release/v0.8.8/。

## 验收要求
- 等待依赖会话 01a2131f-4424-7ef1-a0c7-328bcd7cb2d0 完成推送合并，只观察；fetch 验证修复祖先关系并记录完整 SHA。
- Linux amd64 离线完整升级包，包括 backend/web/db 镜像和既有工具；Windows x64 安装器、latest.yml、blockmap、download.json。
- 查明上一实际交付与发布标签，逐条审计 SQL 文件、摘要、迁移账本和实际 schema；旧版升级、重启幂等、失败退出及健康门槛实测。
- 配置对照及真实 Compose 传递校验，保留认证模式、密钥、数据库、部署ID、URL、卷与客户端身份；无本地凭据。
- 下载服务有变化则交付升级包，保留历史安装器和 download-config.json；HTTP HEAD/Range/完整摘要验收。
- 后端/Web及登录验证先于桌面/daemon更新开放，补全 CLI/daemon 交付。
- 中文手册、发行说明、SQL/配置清单、SHA256SUMS、构建验证证据同源。Windows真机未测须明确。
- 向 yangshangwei/multica-0.4.37 发布 v0.8.8，监控 CI、校验线上资产，禁止覆盖异内容标签/资产。

## 约束
不得清理或误提交历史 untracked 资源；构建隔离；不改全局 SSH；不抹去既有 HOLD 证据。
