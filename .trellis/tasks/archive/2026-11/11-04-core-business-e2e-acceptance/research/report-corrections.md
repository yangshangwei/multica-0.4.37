# 最终报告修订清单

最终版本应完整重写现状，不继续附加阶段流水。先写“是否达到完整验收”，再给版本、实际分组结果、未通过项、证明局限。

- `report.md`旧“Go service已过/其余81未完成”替换为82包实际多段执行和原始中断说明。
- 旧“性能正在执行”替换为2PASS/1FAIL，closure2155.025>2000。
- 不再写“夹具尚未应用”：15个E2E文件变化，产品文件保持初始manifest；记录夹具每次来源变化与复验。
- 区分唯一用例数、实际执行次数、首次全量结果、最新框架状态和带后置审计的验收状态。不要把后续多轮补跑写成单轮make check或Playwright全绿。
- 用更新后reconcile的acceptance_status统计验收结果，保留framework_status。AC33原中断不能直接计验收PASS。
- 441是Playwright库存；knowledge标签46次、wire8项、Linux10顶层、性能3门是独立专项，不能混入441或把同一知识ON/OFF当两个独立原始身份。
- 主报告默认Go20SKIP与后续精确补测分开。可给最新补测视图但不得改原默认包报告：wire1、Linux7router/daemon和agent2顶层、性能3分别替换同身份；逐身份扣除已补测的13个顶层skip后，当前仍有7个顶层未覆盖：Webhook数据库错误占位、公开skill导入、HTTP MCP无recipe、2个Windows行为、pg_bigm、CLI未知字段占位；Linux补测已补齐唯一子例skip。后续新增证据需重新对账。
- macOS已通过完整Main复验时可写本地ad-hoc目录包/真实Main/IPC/受控Host；仍不证明新装升级、正式签名公证或自动更新。Linux静态DEB结果与实际安装/运行结果分别列。
- 披露Git内TMP误识别原仓库及.cursor现状、第一次父进程中断、Rosetta数据库崩溃；不能写“全部原工作区未受影响”。
- 最后记录cleanup实际完成的数据库连接、API/Web/renderer/daemon/容器，及保留的证据/专库/工作树。不能把全量未清理状态写成结束。
- 保留原前端一次超时与浏览器两项无改动复验通过的稳定性风险，不称已修产品。
- 用户明确真实模型延后，列NOT_RUN/by agreed scope，不能当成本任务执行阻塞或借用其他会话授权。
