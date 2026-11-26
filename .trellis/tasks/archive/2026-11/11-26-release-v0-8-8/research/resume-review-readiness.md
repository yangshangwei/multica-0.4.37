# V0.8.8 恢复发布独立复核

日期：2026-11-26（Asia/Shanghai）。结论：**现有准备成果可继续使用；未发现新增发布阻断。正式交付仍待 Linux 构件及 Release 门槛完成，本报告不作总放行。**

冻结源码和本地 tag 均为 d2c9240eaf9f768446faba93cac625bd63e9cf53，tracked 工作区无改动。原发布会话停在 Release run 45721755253 attempt 2；主会话负责监控，避免并行重复操作。当前任务的详细研究在发布 worktree；主目录早期 task 副本缺少后续记录，收尾时应归并指向。

## 已独立复核

- SQL 1900 个源文件（877 up、877 down、146 queries）逐个通过 git cat-file 对照不可变 Git 对象的长度/SHA-256；完整 server SQL tree 与清单集合相同，1900 行 SQL-SHA256SUMS 与清单相同。24 个配置源文件的摘要全部匹配冻结 Git 对象。
- dist 四个 Windows 更新文件摘要与 native binding 一致；安装器 181985338 字节，SHA-256 b41f87075bdfb6f210e89319d30f398f5aeae88c2e1106708567c43260754d1d。YAML SHA-512、download.json 版本/架构/文件/大小一致。
- 公开 Windows 回执指向确切私有汇总 SHA；五类 evidence 文件与六份原生回执摘要重新核验匹配。原生 run45710224239 的成功状态及 headSha 匹配，5 步生命周期与最终外部协议保留、升级和重装两个完整知识 OFF 旅程均有原始成功回执。
- 自动更新原回执显示旧 blockmap 404 后完整下载、安装并启动 0.8.8，保留设置及历史。其事实仅支持 full-download fallback，不支持差分下载通过。
- 下载服务原回执的首次/重启 HTTP、管理员配置及历史构件保留，与汇总一致；原 payload/HTTP/browser 证据的完整摘要未漂移。5 份已交付通用指南逐字节等于冻结源。
- 下载同步 tar 10 个普通文件，路径无绝对/穿越/链接/设备/AppleDouble；Windows CLI zip 仅四个预期文件，二进制摘要等于已原生验收内置 CLI；三份现有 sha256 sidecar 与各实际构件匹配。
- OMP 24 项诊断证据长度/哈希全部匹配；五份原始 Go JSONL 独立重数为 normal/race 原测试各100、私有builtin对照各100、写入失败负例20，均有实际命名 run/pass，无fail/skip。读取冻结 omp_test.go 89、95 行及 pi.go 466–468 行，缺 cat 夹具与正确写失败保护因果一致；不把诊断 overlay 当标签测试修复。

细项回执：主目录 .artifacts/release-v0.8.8/resume-review/initial-readiness.json，共 66 项本轮检查通过。证据完整性检查不等于重新执行 Windows、全部 ASAR 块遍历、数据库或浏览器验收。

## 未修改的已知问题与最终门槛

1. OMP 测试隔离 PATH 却依赖裸 cat 的竞态仍在标签中。已有独立因果审查支持当前一次完整同 SHA 重跑；保留首次失败，不能以绿色重跑宣称缺陷修好。产品拒绝输入写入失败的逻辑保持。该测试修复需要后续提交，当前不移动标签。
2. SQL-INVENTORY 的 artifact_sql_comparison、database_ledger_schema_verification 以及 CONFIG-INVENTORY 的 live_container_environment_verification 仍明确 pending。最终须引用正式 CI feed 与实际交付归档的运行证据填充，不能直接使用预览结果。
3. 正式服务端包、Linux CLI、三文件唯一 feed、镜像来源/架构、两模式真实旧 PG 升级及 fresh/schema/恢复/失败门槛、包内双指南、总 SHA256SUMS 和 FINAL-VERIFICATION 尚待完成。
4. 三份 final-docs 草稿明确准备状态和待填项，首次 Release CI 失败与 OMP 已知缺陷、Windows 基线/未签名/OFF/full fallback、旧 HOLD 限制均已保留。最终填写后再做一次全目录复核；不要将 prepared-windows 内局部 SHA256SUMS 冒充整个交付目录总清单。
5. GitHub Release、全部附件与本地摘要由主会话核对完成后才可撤下准备标记。CI 自动基础发布不等于 Linux 离线包和全部附件已交付。

## 验证

Lint、TypeCheck：核对相同 SHA 的 main CI 45713882395 中 frontend-build 的 Build, type check, and lint 步骤 success；本轮无代码编辑且按任务要求不重复重型检查。该 run 的 live Go vulnerability 步骤 success；image-budget 为条件 skipped，不应泛称每个 job 都执行。

Tests：原始 Windows 与 main CI 证据、SQL/config/Git 字节、现有归档及诊断事件复核通过；本轮未重跑完整套件或正式 Linux 验收。产品/发布文件未修改，仅新增本审查与 JSON 回执。

一次 exec 读取收到 transport-death 拒绝，保留全部运行状态，改用 Node filesystem/child-process 的只读路径；没有重启或清理他人运行资源。
