# V0.8.8 执行计划（评审后定稿）

- [x] 创建任务、三条只读研究与独立计划审查；最近实际交付及正式基线为v0.6.7。
- [x] 观察依赖会话完成合并，fetch并验证修复祖先关系：main c920c5e5b0354258ad0a37d500655a6856d5ed5b。
- [x] 创建隔离worktree，修订两份手册；发布Node94PASS/4显式opt-inSKIP、selfhost/entrypoint通过。
- [x] 修复Windows runner长路径检出阻塞，重新冻结唯一SHA。保留第一次CI失败。
- [x] 原生Windows对最终SHA构建及安装/保留状态升级/自动更新/协作验收；未签名与客户内网范围独立记录。
- [x] 标签前核心候选预验：隔离数据库按v0.6.7旧SQL到最终877清单迁移，schema/账本/幂等及历史起点核对。候选/preview不是正式交付。
- [x] 修复当前main CI新暴露的ProjectAutopilot运行环境并发删除竞态，确定性回归和独立复核后更新冻结SHA；保留原失败。
- [x] 最终源码进入main，等待该commit当前main CI及live漏洞扫描成功，不覆盖/取消先前未完backend验证。
- [x] tag前门槛通过，创建并推送不可变 v0.8.8（d2c9240eaf9f768446faba93cac625bd63e9cf53）。
- [x] Release CI 第2次完整运行通过，并保留其唯一累计 changelog 三文件；第1次 OMP 测试夹具竞态失败及复现证据保留。
- [x] 用CI生成的exact changelog.json和同一SHA复用build-offline-upgrade.sh构建正式linux/amd64完整包；记录镜像来源/架构、SQL/配置/双手册字节。
- [x] 从最终包演练password/device旧→新、fresh schema对照、配置/身份/卷保留、healthz/登录、重启幂等、失败退出与健康门槛、备份隔离恢复。
- [x] Windows原工件收集、隔离Nginx HEAD/Range/摘要/配置保留及浏览器验收；同源Windows CLI已提取。
- [x] 从正式 backend 提取 Linux CLI，并完成两平台独立CLI的最终交付。
- [x] 汇总中文手册/版本说明/SQL及配置全清单/构建验证证据和SHA256SUMS，独立最终审查与无凭据检查。
- [x] 监控Release CI到结束，向yangshangwei/multica-0.4.37补上传全部已验证附件，核对线上本地摘要；所有附件完备才宣布交付完成。
- [x] 收尾Trellis并记录真实限制。

## 顺序与风险

标签CI会自动公开基础Release；因此标签前要有候选预验。正式包仍须等该次唯一feed并重新验证，不能把两次独立生成feed说成同一字节。不得移动不同内容标签/资产。

原全产品HOLD仍保存：Linux daemon Namespace安全运行前置未满足、匿名GitHub集成额度0、Darwin后端原路径未重跑。其适用范围独立评审，绝不改写为全产品通过；本次Linux server/Windows Host交付必须有自己的实际证据。

下载服务相对v0.6.7只有页面注释/手册变化，无功能升级；保留历史包和业务配置。独立CLI公网自更新仍指upstream，必须受控交付替换。数据库回滚依赖完整同一备份点，不靠换旧镜像逆转schema。
