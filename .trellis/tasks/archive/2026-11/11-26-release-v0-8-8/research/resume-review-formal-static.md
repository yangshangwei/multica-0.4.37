# V0.8.8 正式构件静态独立复核

日期：2026-11-26（Asia/Shanghai）。**PASS：正式 Linux 完整归档、保存镜像、CLI 及唯一 CI feed 的静态来源和完整性通过。** 本轮仅文件系统与一次只读 GitHub 工件下载，未调用 Docker，未干扰正在执行的升级演练。最终运行验证和 Release 全资产仍待主会话收齐，本结论不是总放行。

## 确切交付来源

- 源码：d2c9240eaf9f768446faba93cac625bd63e9cf53；版本标签 v0.8.8。
- 完整包：multica-server-upgrade-v0.8.8-linux-amd64.tar.gz，308025472 字节，SHA-256 eedd38f3ef0237f202bffc1c7ca555c94d2d757eba297c663fa6c4f090254e64。
- 内层镜像包：SHA-256 f286b5f6159fc6f58ce0781967feaca1a17ce1bf06ae06d320e17e63e786f229。
- CI run 45721755253 attempt2 的 release-changelog 工件 ID15078827132，headSha 与冻结源相同。独立通过 GitHub API 下载原 zip 并核验 GitHub digest 9bea48a55d3440a09ab3dc72b0916d286480fdddf6cb20c3ddb5b9fea9320372；zip 精确三文件与主会话下载副本逐字相同。
- changelog.json SHA-256 05b876cbab66ea1434d2c08f1057f3091262b71bf4f7805fc1dc8924c95371b7；metadata 的源码、基线、仓库、版本、日期身份及 JSON.stringify(entry) 摘要准确。markdown 摘要亦匹配。

## 静态复核结果

独立 Python 流式审计记录在主目录 .artifacts/release-v0.8.8/resume-review/formal-static-review.py/json/log。

1. 外归档 23 个普通文件，与实际解包目录及打包清单大小/SHA 一致。唯一预期根，无绝对路径、穿越、符号/硬链接、设备、AppleDouble、真实.env、开发代理状态或管理员凭据文件。README.md 和 操作文档.md 均逐字等于冻结 Git 的服务端手册；配置、主/overlay Compose、升级/发布/初始化工具和附带指南逐项等于 Git 对象。
2. 保存包全部 47 个 blob 的内容摘要等于地址；三个 linux/amd64 镜像 config 对应报告，全部 38 个压缩层解压后的实际 SHA-256 等于各 rootfs.diff_ids。不是只依赖 docker inspect 或字段声明。
3. 直接扫描 backend 保存层，1754 个实际迁移文件（877 up+877 down）的完整文件集、大小、SHA 与已对照 Git 对象的清单一致。旧 layer 中不存在被忽略的 app 白化覆盖；entrypoint/LICENSE/NOTICE 对应冻结源。
4. 实际保存的 server 二进制内包含本次 CI changelog.json 的完整确切字节。外包 changelog 文件也相同；没有将预览 feed 当正式源。
5. 五个后端可执行文件的保存层 SHA 与提取文件、provenance 一致；均 ELF amd64，独立 go version -m 输出与记录相同、Go1.26.11、CGO0、GOOSlinux/GOARCHamd64。server 与 multica 的 ldflags 包含完整 source commit 和 v0.8.8。
6. Linux CLI archive SHA-256 6a285abe62caab25c3cfab01bf376d857b8009e444bfe9b3e2e4cf18dd9907ad，仅 multica/LICENSE/NOTICE/README.txt 四个常规文件，逐字等于提取物。CLI SHA-256 8e8f0d0286be3c4d543b4044b2c03fec10483843e29283c466693fb8815607ce，确实来自本次保存 backend。
7. 应用两个镜像所有历史层中未发现禁止的私有状态路径；runtime Env 无 JWT_SECRET、DATABASE_URL、POSTGRES_PASSWORD、GITHUB_TOKEN、OPENAI_API_KEY、NODE_OPTIONS。此为实际路径和配置检查，不宣称扫描过任意第三方内容中一切可能秘密。

共 51 项检查通过。web 的应用层使用 BuildKit 缓存，build log 显示当前冻结源码和原配方的 cache 命中；不能仅凭层创建时间早于正式构建判错。无本地 Dockerfile 修改或 heap 配方变体。

## 后续准确性要求

- Linux Docker 构建上下文不含 .git，Go binary 无 vcs.revision/vcs.modified 字段；来源来自干净源码、配方、完整 ldflags SHA、保存镜像和哈希绑定。Windows CLI 的 vcs.revision/vcs.modified 是另一套确实存在的字段。最终汇总不要把 Windows 的字段推广到 Linux。
- CLI version JSON 的值为 v0.8.8（带 v）；这是仓库 Docker build VERSION 的真实值，不要把实际二进制改写为声称 0.8.8。文件名仍为 multica-cli-0.8.8-linux-amd64.tar.gz。
- 条件扩展、旧 PG、账本/schema、配置/身份/卷保留和健康/登录/失败/恢复必须引用正式运行证据。SQL/config public inventory 的 pending 字段仍应在最终演练后更新。
- 未重复 lint/typecheck：本轮无 tracked 代码变更；前一报告已核验相同 SHA 的 main CI success。未运行构建/容器/测试，运行验收由 server_delivery 独占。
