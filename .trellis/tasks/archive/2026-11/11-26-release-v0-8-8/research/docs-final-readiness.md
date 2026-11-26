# 最终交付文档恢复核验

日期：2026-11-26（Asia/Shanghai）。源码冻结 d2c9240eaf9f768446faba93cac625bd63e9cf53。此次只修改私有 final-docs，未修改产品源码、标签或 dist。

已阅读并核对构建包装脚本、离线升级脚本的 overlay/备份/镜像选择/健康门槛、下载服务 collect/publish/verify 参数和 CLI version JSON 字段。三份文档现有命令均匹配冻结源码；升级顺序、认证保留、下载源切换顺序和客户端身份要求保留。

新增公开失败索引：13 次 Windows 失败、最初 main CI 产品回归、Release attempt 1 的 OMP 夹具失败。机器状态记录同步保留失败和 scope。正式 Linux、唯一 CI feed、Release attempt 2 及附件校验仍未完成，不提前改写为通过。

## 收尾输入顺序

1. 主会话给出 Release attempt 2 的 job 最终结论、唯一三 feed 文件 SHA256 与 metadata commit。
2. server_delivery 提供正式包归档/内镜像/CLI 摘要、源码与配置/SQL/双手册绑定、实际 PG 两模式升级/fresh/restart/failure/health/恢复回执。
3. 基于这些真实回执更新三份文档与 build-source；保留所有历史失败和明确未测范围。
4. 主会话拷贝最终文档到 dist 并生成总清单。总清单不哈希自身；文档不包含会反向包含自身的清单/最终回执摘要，避免循环哈希。
5. 远端上传及摘要核对由主会话执行，最终正式发布状态只依据完整结果填写。若反馈需要再次修改任何字节，重新生成受影响清单并审查，禁止覆写已存在的异内容正式资产。

## 本轮静态验证

读取冻结源码及原生 Windows 公开回执，核验全部 13 次失败 ID；4 文件 UTF-8 可读、JSON 可解析、无个人绝对路径/私钥标记。没有运行构建/测试/安装器；这是交付文档编辑。

- README-v0.8.8.zh-CN.md: 2071c864282babb0b471425b24bf0e1d0c4590bb7d7d7ca61c0773d9213468ce; 10976 bytes; privatePath=false
- RELEASE-NOTES-v0.8.8.zh-CN.md: 97abbfd4cb3be95ee360d613ba641021e5a1685bbb4618e2e23a6412573ba311; 8223 bytes; privatePath=false
- BUILD-VERIFICATION-v0.8.8.zh-CN.md: 0b83b7be5551ade5f58a7975c62f5d23f18a670e915a925ee650bdfdd747e6ef; 13071 bytes; privatePath=false
- build-source.json: 66909e2470d897f078c0e730ce1e6d346b6bc2a9a4e026f50cdeda0cd6ee4267; 1717 bytes; privatePath=false

## 正式文档完成

已读取 sanitized server-verification.json（SHA256 8125cf2d158a12bc03fe5ac33fb97310346cd8007bf3513dfacdd6a2b56f7ae3）和 toolchain-and-base-images.json，核对 parent 与 server lane 回执后完成 5 份 Markdown + build-source.json。12 阶段/cleanup、两模式 PG/身份/账本、fresh/admin、3restart/DDL、112 SQL observations、故障/缺最早历史账本门槛如实记录。SQL 源预验/私有预览/正式包范围分开。

全部文档无准备稿/待填/待补/pending 标记，无个人绝对路径或私钥标记；正式完整远端附件结论统一引用 FINAL-VERIFICATION.json，不提前声明未来上传完成。未改源码、tag、原始 CI feed 或已发布资产。文档与运行记录引用形成单向摘要，不循环哈希。逐文件摘要见 docs-final-verification.json。
