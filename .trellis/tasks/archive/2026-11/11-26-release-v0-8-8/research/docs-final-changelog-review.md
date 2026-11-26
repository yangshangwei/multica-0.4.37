# 正式 changelog 与中文发布说明复核

日期：2026-11-26（Asia/Shanghai）。Release run 45721755253 / attempt 2 的 verify（实时漏洞扫描无绕过）及 changelog 已成功；镜像与发布尚待最终结果。正式 Linux 构建已开始。

独立重新计算三个 CI 工件摘要，与主会话 exact-ci-changelog-verification.json 及 metadata 全部一致。14 条累计发布历史；本版 fork 身份、d2c 完整提交、v0.6.7 完整基线正确。本版生成 322 项，分为 features 88 / improvements 7 / fixes 164 / other 63。

中文说明是操作相关摘要，不宣称逐条翻译。协作授权/上下文/Stop恢复/知识门槛、桌面登录恢复、下载页配置、自动化运行环境并发删除修复和停用 Cloud 等候字段都有生成条目支持。SQL 数量和配置默认值另依赖 SQL/配置清单及冻结源码，不从提交标题推导。

原始 changelog 存在同名条目（不同提交），属于生成器按 commit 收录的结果；未合并或改写。metadata status=published 与时间戳是候选正式 feed 的内容身份，不代表 GitHub 与全部附件已经完成。公开构建记录已明确区分。

已更新 final-docs 三份 Markdown 和 build-source.json 中的事实：verify/changelog 成功、唯一三文件摘要、Linux 正式构建中；正式 server/Release/资产核验继续 pending。首次 OMP 夹具失败及未修复结论保留。没有重新生成/改写 exact-ci-changelog-attempt-2。

## 支持摘要的原始条目

### collaboration

- preserve authority through handoffs and account revocation (`cc85172d98ad11a3e4e123f379639431f8df0f08`)
- keep shared runs authorized and recoverable (`0155ee7353a02c082e6c49d61e7c652e7ad97ff4`)
- keep source edits within shared authority (`5c6ec11c763c3c246a130ba90fae200cdfa1391a`)
- preserve shared control with ordinary host permissions (`8bfc84ad17fb529baf8a82a7dfd42b3e3d59b6f8`)
- preserve context across interrupted and handed-off runs (`0e1ee8e1e957b46a5bbd5d91a84772d69dd4d64b`)
- keep approved knowledge authorized through every launch (`96cedf4850319a06c98fdda3d67d82595f01b6b2`)

### desktop

- open the first workspace after a fresh sign-in (`c84a1678121b1861102227ade9344de7e14ffcd7`)
- keep a deep-linked thread through workspace rehydration (`614acecfe5b619fb40201c5a3c48a5adc8f26e56`)
- read the Windows boot identity from a marked PowerShell line (`b5b9f394ccdd174df1d4cd6d86414e35cc673bd3`)
- read the Windows boot identity from a marked PowerShell line (`750efbdb40c6b99848763ae6a71aa00755f76cec`)
- keep explicit native renderers from losing isolated API access (`b8035617de06cf164354ff75f122f1e28c990a8b`)
- wait for cookie teardown before clearing the active session (`fbd19d3018d458ef9c854a3391541606ad864a01`)
- let Windows release verification reach retained source paths (`7a99271341910b58c35a99d5f38ffe1ae75c6959`)
- launch Windows acceptance with one resolved Node executable (`7a954ae0123db2c1ff2320aca06a16f761d042cc`)

### download

- publish the desktop download page through /api/config (`8a642bf6a7325df7737cd34e9426154170dc6a7f`)
- link desktop downloads to the configured download service (`19ffd75de8ee9da56315d088cc82904430edbe5c`)

### automation

- 自动任务调度遇到运行环境并发删除时，正确跳过不可用目标，避免误报数据库故障。 (`5a48e91b385967309e53f0ceb211931eecdef873`)

### cloud

- drop the unused Cloud waitlist end to end (`a06ccda8dbda3e9046a1f04a9b31f4d127086540`)

## 后续构建与发布事实更新

Release attempt 2 总体 success：verify/changelog/4 个镜像构建/2 个 manifest 合并/publish-changelog 成功；release/desktop/helm-chart 是 fork 条件跳过，未误报为执行成功。GitHub publishedAt 为 2026-11-26T01:37:58Z（北京时间 09:37:58），区别于 feed 时间。已读取父会话保存的 release-initial 与 CI receipt。

正式 Linux 完整包静态核验通过：308025472 bytes，SHA256 eedd38f3ef0237f202bffc1c7ca555c94d2d757eba297c663fa6c4f090254e64；CLI 归档 9255266 bytes，SHA256 6a285abe62caab25c3cfab01bf376d857b8009e444bfe9b3e2e4cf18dd9907ad。构建 191.398 秒 exit0。已记录内镜像摘要、47 blobs、三 config 摘要，区分 config/index/manifest。Linux CLI 实际 version 是 v0.8.8，未声称 VCS buildinfo；来源采用冻结上下文+配方+构建回执+ldflags commit。README 同步区分 Windows 0.8.8/Linux v0.8.8。正式运行与完整远端资产仍 pending。
