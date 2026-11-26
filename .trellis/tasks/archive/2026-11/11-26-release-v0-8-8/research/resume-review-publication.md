# V0.8.8 38项正式附件发布前复核

日期：2026-11-26（Asia/Shanghai）。**APPROVED：publication-payloads.json 中当前38项确切文件可上传。** 本结论绑定各文件当前大小/SHA，文件变动后须重新核对；不是未来上传完成声明。

本轮90项检查通过：目录恰好为38附件加未上传RELEASE-STATUS；每个文件为普通文件、无symlink，实际大小/SHA与显式计划匹配。正式Linux包/CLI、Windows原生构件、下载同步包和3个CIfeed均延续已独立验证的确切摘要，无重复镜像重扫。

SQL/config源清单原内容未变化；新增正式运行引用绑定server-verification.json SHA8125cf2d158a12bc03fe5ac33fb97310346cd8007bf3513dfacdd6a2b56f7ae3与正式归档eedd38f3...。五份最终说明已反映两模式PG17.11→17.12、594→877账本、正式schema/故障/恢复/配置门槛；保留Windows基线/OFF/unsigned/full fallback、Linux仿真/可选扩展、OMP未修复测试夹具及旧HOLD限制。Linux ldflags绑定与Windows vcs字段、两个CLI版本前导v差异均已准确区分。

32个文本附件未发现本机私有路径、私钥/明显令牌；与39个实际私有fixture秘密/唯一身份值逐一比较均不包含。首次宽泛扫描将固定管理员默认用户名匹配到SQL-INVENTORY而报警；追查显示只是冻结版本早已存在的SQL文件名及其校验清单和通用操作手册文本，逐个git对象存在/原文一致，非泄漏。此误报解释保留，不削弱随机密码/token/UUID匹配检查。

本轮实际只读GitHub核验：v0.8.8解引用仍为d2c9240eaf9f768446faba93cac625bd63e9cf53；Release非draft/prerelease、targetCommitish相同，实际published_at为2026-11-26T01:37:58Z。run45721755253 attempt2 success；全部job结论与公开CI回执一致；Scan Go vulnerabilities success，emergency bypass skipped。此时远端只有3个changelog附件，其大小/digest与计划完全相同，无冲突。

父会话可将计划approved_for_upload设true并使用已审核无clobber助手上传相同文件。最终FINAL-VERIFICATION、SHA256SUMS在38附件完整远端下载核验后生成/上传，再确认其本地远端一致并撤下准备标记。当前未提前声明这些闭环已完成。

详细批准清单及90项检查：主目录.artifacts/release-v0.8.8/resume-review/publication-payload-approval.json。未改源码、标签、公开文件、发布计划或远端状态。

## 最终回执与校验清单批准

38个附件已上传后的独立复核通过：本地保留的38个完整远端下载逐项重新计算大小/SHA，与批准清单一致；GitHub资产元数据38项digest也相同。两大文件并行Range下载保留118份响应头和块文件（EXE44块、服务端74块），逐块Content-Range/长度准确，连续覆盖完整文件且无重叠或间隙，拼接后完整SHA匹配。没有将部分下载成功误作全文件成功。

最终SHA256SUMS恰好39行，覆盖38payload+FINAL-VERIFICATION，不自引用、不含准备标记；每行均按当前实际文件核验。FINAL保留所有范围限制、13次Windows失败和OMP attempt1，正文及来源与既有证据一致。本轮204项文件/摘要/头部检查通过，未重复构建或运行测试。

允许上传最后两项确切字节：FINAL-VERIFICATION.json SHA256 bc3f1a7d309f63c49ca5cd81693c61afe8cb32f122af4fccda5b70423a301dd9；SHA256SUMS.txt SHA256 0483ced5d3cfef21c5d7225e85367a385658e90c24048b5d9f0afdad2926d724。父会话完成这两项远端完整摘要核对后移除准备标记。机器批准记录为resume-review/final-receipt-approval.json。
