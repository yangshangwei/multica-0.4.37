# V0.8.8 发布前独立审查

日期：2026-11-26。审查工作区：`/Users/artisan/.codex/worktrees/release-v0-8-8/multica-0.4.37`。范围为两份发布手册的当前差异、计划与已有研究；不修改产品文件，不启动重型构建/验证。

## 结论

**两份手册可提交并冻结源码，未发现阻断文档提交的问题。** 可据该 SHA 启动 Windows 原生构建/验收。此结论不批准标签、Release、生产升级或全产品验收；正式交付门槛仍待完成。

审查时 HEAD 为合并修复 `c920c5e5b0354258ad0a37d500655a6856d5ed5b`。依赖记录包含修复提交的祖先证明；未以会话 idle 代替 Git 来源证明。

## 手册核对

- `docs/offline-upgrade.zh-CN.md`：SHA-256 `47de6290715cb1dd601f665f72f72b484c0ae41177480232e9ade331f6bcdb7b`。
- `docs/desktop-intranet-update-runbook.zh-CN.md`：SHA-256 `c4b65f75a5c54a95c69b4480f5c290614f112414a0908a52dd0839cfa89e7414`。
- 实际 tracked 迁移为877 up、877 down、最高前缀889；基线数量表与 `sql-supported-baselines.json` 一致。手册明确数量/哈希和账本/schema分别核对，不把清单比较当已完成升级。
- 设备/密码模式选择、已有密钥/部署ID/业务地址/卷保留、首次管理员初始化、745数据损失、多旧API停写、恢复需旧备份等约束保留。
- 知识默认 true 与 `server/cmd/server/router.go:432`、模板及Compose一致；不改写已有显式 false。
- 自托管 daemon 自动更新默认关闭与 `server/internal/daemon/config.go:562` 一致。`multica update`/缺失内置CLI fallback仍指向upstream的限制真实存在，手册未假称fork发布改变其来源。
- 先服务端/Web迁移、healthz和原认证登录，再开放Desktop/daemon更新；Windows metadata、blockmap原始摘要、download.json额外核验及未签名/真机范围说明明确。
- 下载服务无功能变化的结论与已保存的逐文件研究一致；手册明确只同步同源页面/手册时保留持久安装包和管理员配置。没有虚构Nginx功能升级。
- 不把2026-09-24旧测试、旧ia32素材或今后CI结果提前写成V0.8.8通过。

## 先前 HOLD 的适用范围

原 `continuation-report.md` 及独立复核仍应原样保留。本评审不追认原失败、不缩减原全产品验收，也不将本次范围内通过合称全产品PASS。

| 项目 | 当前证据与本次关系 | 正确处置 |
| --- | --- | --- |
| Namespace九个旅程 | 缺现有安全策略允许的非特权Linux控制器环境；当前Docker默认策略下未完成。`controller_unsupported.go` 对非Linux明确返回不支持，Windows Host验收不是Namespace验证。Linux服务端容器正常运行也不能证明Linux daemon的Namespace。 | 不阻止文档提交或Windows构建。若正式交付声称Linux Namespace生产可用，则仍是该能力放行硬门槛，需原策略允许的环境及完整旅程。若本次只承诺Linux服务端和Windows Host交付，最终版本说明应明确Namespace未验收，并保持既有执行门槛/真实配置，不可用特权或unconfined绕过。 |
| 匿名公共GitHub导入 | 先前真实tree403/remaining0，本次主会话读取额度仍0；是外部依赖未完成验收，不证明一个新产品缺陷，也不等于导入成功。 | 完全离线升级包/内网更新源不依赖该公网导入路径，可作为明确的外部集成限制；不能声称GitHub完整导入已通过。若客户本次必须使用该功能，则需恢复额度后真实验证。不得借个人token改变原匿名验收身份来制造通过。 |
| Darwin后端经Docker发布DB端口的完整包 | 先前该路径未重跑，Linux normal/race和Mac桌面分范围结果不替代它。 | 本次目标Linux服务端/Windows桌面无需以Darwin后端全包作为平台门槛；保留该未测/失败范围，不宣称Darwin服务端或全平台验收通过。 |

这三个范围判断不自动解除本次的数据库迁移、配置保留、登录、镜像来源/架构、Windows元数据/下载及CI门槛；它们均是直接交付要求，必须继续实测。

## 发布前尚需完成（非文档提交阻断）

1. 冻结SHA及当前main CI。独立查询 CI `45656329181` 的 headSha确为合并SHA，live `go-vulnerability-scan` 和frontend成功，backend-tests仍在运行，故不能称该CI已通过。文档提交若触发paths-filter跳过漏洞job，不能把skip叫作最终SHA的live扫描；应保留相同server树的来源绑定并在最终SHA执行本次live扫描，随后完成tag Release自身门槛。
2. 两份计划的末尾修正明确 exact CI changelog，但前面仍列旧顺序。建议主会话收敛成单一有序清单，明确“标签前候选预验”和“CI唯一feed后的正式包验收”是两道门槛。现有release.yml在镜像和验证成功后会自动公开基础Release；因此不能等公开以后才第一次发现283条新增迁移是否能升级。至少先以同一冻结源码的明确preview候选完成核心升级预验，正式feed包再复验；不得将preview的feed字节冒充最终一致性证据。
3. 构件内全部迁移逐文件哈希与冻结源码一致；实际旧版password/device升级、账本与schema、条件对象/索引、重启幂等、失败退出、healthz、登录及隔离备份恢复。
4. Windows原生结果绑定原始工件与SHA，明确旧版实际升级起点。未签名不能称Authenticode/SmartScreen通过；无客户内网测试不能称客户网络验收。下载源完整摘要、HEAD/Range和download.json须独立完成。
5. 标签/资产存在性检查、origin目标、全部附件上传及线上本地摘要匹配。任何新源代码或依赖修复都会改变冻结SHA，相关构件与验收必须更新。

## 验证

- 本审查执行 `git diff --check`：PASS。
- 本审查独立执行既有窄选 `node --test --test-name-pattern='upgrade archive includes publication tools' scripts/offline-changelog.test.mjs`；结果为1 PASS / 0 FAIL / 0 SKIP。此测试使用fake Docker，只验证归档规则和双手册字节，不是真实容器或数据库验收。
- 已直接读取主会话release Node日志：94 PASS / 0 FAIL / 4明确opt-in SKIP；读取selfhost-config及entrypoint的exit文件均为0。没有将skip计为通过。
- 全量lint/typecheck未重复运行：本次产品差异仅Markdown；此受限审查不启动重型检查。CI的frontend成功是已观察的集成证据，不冒称本审查运行了lint/typecheck。
- 产品文件未修改；仅新增本报告。不需因纯发布说明引入新测试或新spec规则。


## 补充独立审查：Windows checkout 长路径修复

结论：**三文件修改可提交并从新 SHA 重试 Windows 构建；未发现阻断提交的问题。** 实际 checkout 和安装恢复需要新原生 run 验证，17项结构测试不能代替该验证。

审查对象及SHA-256：

| 文件 | SHA-256 |
| --- | --- |
| `.github/workflows/desktop-smoke.yml` | `77fe11125dc72164126df3443fd2e2939468b66d556b6af142d506d853b1b433` |
| `.github/workflows/release.yml` | `e6bcd2fffa7894c7c6af7a98b9607ee040da6f6155df65ad398bd1a89be27366` |
| `scripts/agent-critical-path-workflow.test.mjs` | `c42c71475ae96cce514eaacd19cde8a1217c33a21c585c04ae24cf029c5ba9bd` |

证据与因果边界：

- 直接读取 `research/windows-checkout-fix.md`；实现记录先扩展测试的1 FAIL，再修复后的17 PASS及11项installer契约PASS。
- 独立读取 GitHub run `45659248507`：headSha `53df2dbb60fcb29acebb507f4a6041802bfb27dc`，Windows job在 `Checkout candidate source` 失败，尚未到构建/安装；错误日志显示长路径问题。后续两个always清理步骤也失败，属于源码未完整checkout后的连带失败，不能伪称它们已完成清理；此run未启动相应服务。
- 现有 `ci.yml` 在collaboration-host-native、windows-execenv、installer三个Windows入口已有同样配置。新修改只补齐desktop-smoke的普通desktop、windows-acceptance两个入口与release的desktop入口。
- 每个 `git config --global core.longpaths true` 都位于首次 `actions/checkout` 之前，且条件明确为 `runner.os == 'Windows'`。global仅为临时GitHub托管runner的用户Git配置；本机未执行该命令。Linux矩阵不执行它。没有删除历史证据、缩减checkout路径/历史或改变签名/安装验收。
- 发布workflow只新增桌面job步骤；其上游only release依赖、fork判断、镜像/changelog验证图及发布顺序未变。
- 测试扩展原有唯一契约，解析三份真实YAML，覆盖六个Windows入口，检查准备步骤存在且早于checkout，并校验Windows条件；没有创建重复断言层。

独立验证：

- `node --test scripts/agent-critical-path-workflow.test.mjs scripts/changelog-workflow.test.mjs scripts/windows-candidate-version.test.mjs apps/desktop/scripts/windows-collaboration-workflow.node-test.mjs`：**17 PASS / 0 FAIL / 0 SKIP**。
- `node --check scripts/agent-critical-path-workflow.test.mjs`：PASS。
- `git diff --check`：PASS。
- 此增量仅YAML与Node测试，不含TypeScript生产变更；未执行全仓lint/typecheck或重型构建。原生Windows重试由主会话执行。未修改产品文件，仅追加本报告。

冻结/复用要求：此修复改变最终源码SHA；旧失败run保留。后续Windows与Linux正式构件必须记录新SHA，不可把旧SHA二进制标签重写成新SHA。该修复只解除可操作的checkout配置缺口，不代表其后的构建、安装、业务或自动升级验证已通过。
