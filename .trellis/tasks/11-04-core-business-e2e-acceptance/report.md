# 核心业务端到端验收报告

**核心业务用例已闭环：441/441 项均有通过及完整后置审计证据。** 这是多轮、分环境结果，不是单轮全绿。440项验证冻结产品`1a32ca36c2c4f4455a01bdd39d0e0ab4db20cccc`，最后Linux完整应用验证`1a32＋Linux daemon FD修复`；旧候选的失败记录保留。迭代结转P95原测试失败已由用户明确例外接受。

## 关键修复

Linux应用重启卡住的原因是后台daemon继承了Electron的非标准文件描述符；Electron已退出，但调试管道和文件锁仍被daemon持有。修复仅在Linux后台启动与自更新启动边界设置CLOEXEC，保留显式标准流和ExtraFiles，不关闭父进程描述符。

真实Linux管道回归先失败后通过。原完整Main用例换入修复CLI后47.268秒通过，0重试/0跳过；`restart.close`约52.387ms，随后launch、refresh、metadata全部完成，原跨端身份、六次定向写、控制历史及显式Resume断言保留。新CLI SHA`8ad7a9ac0df22fa916a9d006080effb9409193c3b5b8624b5f3365cb42a395b2`；ASAR、主程序及84个非CLI载荷项与旧包一致。

## 已执行验证

| 验证 | 结果 | 证据 |
| --- | --- | --- |
| 浏览器/原生测试库存 | 441唯一用例均验收PASS；逐用例标记实际产品variant | coverage.csv |
| 原始全量浏览器首轮 | 354PASS/3FAIL/84SKIP、exit1；后续补测不追认首轮 | evidence/legacy-full-1/summary.json |
| 非Mobile静态检查 | lint/typecheck15项及UI导出通过；31warnings | research/frontend-result.json |
| 非Mobile单元 | 13244唯一用例有通过证据；首次1timeout，整文件11例复验通过 | research/frontend-result.json |
| Mobile | 类型/lint、196用例/28文件、3类iOSwrapper通过；7warnings | research/frontend-result.json |
| 默认Go82包 | 10393顶层PASS/20SKIP，10413预期身份对账；vet通过 | research/backend-result.json |
| 受控知识集成 | 23唯一用例normal/race共46次通过 | research/conditional-execution-result.json |
| 真实Go→TS契约 | 1次导出、7类schema通过 | research/conditional-execution-result.json |
| Linux条件测试 | 10顶层及3子例通过 | research/conditional-execution-result.json |
| FD修复 | 真实Linux RED→GREEN，父FD/标准流/ExtraFiles保留，原Main通过 | research/fd-fix-result.md；evidence/fd-fix/ |
| macOS完整Main | 实际登录/Stop/重启/显式Resume通过 | private runtime/runs/final-fixture-replay-1/final-r3-01 |
| Linux DEB生命周期 | 新装/移除/重装/purge95断言通过；包管理范围 | research/deb-install-result.json |
| 性能 | 原2PASS/1FAIL，closureP95=2155.025ms>2000ms；此候选例外接受 | research/performance-result.json；research/performance-acceptance-exception.json |

## 接受例外与范围限制

用户明确“这个2155可以接受”，对应提交877664072的接受记录。只免除此候选/样本的性能验收阻断；原断言、样本及FAIL不改，也不外推未来候选。后续私有归因探针曾因自身schema错误失败，并未完成有效归因；性能接受后停止诊断/优化，不计产品FAIL或P95复跑。

默认Go的7个未补条件仍单列：Webhook占位、CLI未知字段占位、公开GitHubskill导入、目录无HTTPMCPrecipe、pg_bigm缺失、两项Windows行为。iOS实际UI、Windows/原生Linux桌面、旧版安装升级、自动更新、正式签名公证及外部实际SMTP/VCS/消息渠道不在本次已证明范围。真实商业模型由用户明确延期；其他线程的模型验收独立记录，不混入core结论。

Go1.27与CI1.26不同；PostgreSQL17.11的x86_64镜像在arm64主机运行，出现过Rosetta退出133。该轮503原样保留，恢复后新库复验。前三知识组执行目录位于Git内的隔离局限保留；后端execenv因此5例失败后改用Git外HOME/TMP，新库660整包通过。原`.cursor/mcp.json`未有完整前置快照，保留其观察字节/哈希，不猜测恢复。

## 证据与清理

原始失败、测试适配、环境修复与新运行分别留档；零自动重试。框架PASS与后置审计分别计数。原产品快照8203文件审计不含未声明产品变化；FD修复另绑定四源文件及独立工作副本。密码、Cookie、原始trace和数据库URL保存在私有目录，不提交。

最新自有API/Web/renderer/Redis四端口已关闭，无本任务容器，core前缀160个保留数据库连接为0；先前包含知识专库的161库清理记录是不同集合，二者不混算。完整明细见`evidence/fd-fix-final-cleanup/`，构件核验和来源见`fd-fix/package-run.json`及独立UX复核。

交付：`coverage.csv`逐例结果、`conditional-matrix.csv`条件范围、`defects.md`历史/修复/接受决定、`research/fd-fix-result.md`产品修复及验证。没有部署、发布或付费模型调用。
