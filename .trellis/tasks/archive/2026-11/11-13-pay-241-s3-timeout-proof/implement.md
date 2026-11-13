# 执行计划

- [x] 用户授权与现有根因/fixture研究完成。
- [x] 独立审查本增量计划；G01/G02 已写入设计与下列实施约束，哈希复核后已 task.py start。
- [x] trellis-implement先补真实timeout/Stop/回执丢失测试，记录首次执行。
- [x] 实施仅测试poller的有界期限与共享fixture接线，增加E2E场景。
- [x] 定向Go normal/race、Go vet、TS类型/lint与E2E静态检查。
- [x] 主会话构建冻结源码API/daemon/Provider/Web/Desktop并执行受控E2E，保留原始失败、gate前后、所有样本与哈希。
- [x] trellis-check独立审阅当前diff与原始证据，修正本范围问题；更新父S3/总矩阵对应局部断言。
- [x] Lore格式仅提交本增量文件，独立验收通过后归档；S3/父PAY-241继续开放。

复用环境根：/Users/artisan/.multica/acceptance/pay241-20261113-01。测试数据库与源码副本由主会话分配，子代理不得自行选择DB或调用真实模型。新增依赖不允许。

## 独立评审后的强制实施约束

- G01：测试专用停机必须保留相同目录、原ledger、runtime/managed身份；restart复用它们。终态最终close才做一次cleanup，不能调用现有删除目录的close后新建运行。
- G02：fault barrier在真实API已经提交后丢弃响应，并持续到旧poller退出且pending ledger已证实；重启新poller后再恢复ACK。单次掉包被旧poller内部补好不算跨进程恢复证据。
- Stop先/Timeout先分开检验；合法无outcome的Stop终结不强制写失败。记录每种outcome/proof/launch业务事实，而非简单数HTTP重投次数。
- 本增量只可编辑design列明的测试/fixture文件。生产bug若被复现先报告主会话，保持首次失败日志，不默改全局超时或恢复规则。
- 无DB定向命令可使用 `node /Users/artisan/.multica/acceptance/pay241-20261113-01/run-nodb.mjs /Users/artisan/.multica/acceptance/pay241-20261113-01/configs/s3-timeout-tests.json go -C server test ...`；仅collaborationrun与daemon的命中测试，不执行真实Provider烟测。
