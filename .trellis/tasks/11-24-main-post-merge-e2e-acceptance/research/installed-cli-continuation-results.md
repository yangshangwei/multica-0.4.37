# 旧CLI兼容性专项

候选：`103e2bc329a9fb845d9f45c20b2a9bb3ebb54625`。当前状态：**installed-cli-02实际1个用例FAIL，失败发生在私有relay可执行权限准入，未证明旧CLI业务缺陷；自有资源和来源清理PASS。**

只读Docker检查确认canonical fixture固定的`sha256:8d5ede4df2fa4b66d499803b95428b729fcb6ba753b3800e9b931b968ee16104`已缓存，OS/CPU为Linux/amd64，无声明image volumes。它是v0.6.7旧CLI的准确兼容性候选，不是当前API源码。新转发器从当前`e2e/fixtures/collaboration-installed-cli-relay.go`编译，Go1.27、CGO_ENABLED=0、GOOS=linux、GOARCH=amd64、GOPROXY/GOSUMDB关闭。实际ELF为64位little-endian、machine62，无PT_INTERP；binary SHA为`9750e2c8173dfb679d087514f841b5693b2d43fe26f74f7d676745d8ce2c27f9`。编译没有执行TestMain、CLI、模型或SQL。

沿用原始`collaboration-installed-clients.spec.ts`单个240秒用例，单worker/零retry。正向普通任务更新证明旧CLI与真实认证/转发可用；随后对paused、Stop尚无自然proof、作者被移除三种状态，各执行update/comment/rerun，要求实际旧CLI非零退出、真实API409、原状态/队列/历史不变。synthetic daemon只通过公开claim/permit创建真实授权状态并保留未知proof，不运行daemon或模型。完整Desktop、安装升级和真实商用模型不在此用例中。

新私有文件位于`.omx/private/qa-work/main-post-merge-e2e-20261124-080504/installed-cli-acceptance/`。runner复用已审runtime的真实password/namespace-only API与来源/资源规则，没有修改原runner或产品测试。`docker_guard.py`仅准入canonical命令形状，固定镜像、linux/amd64、readonly、networknone、当前relay复制路径；凭据原样通过stdin，不进入argv或日志。为了捕获version检查的准确containerID，私有wrapper把等价`run --rm`拆为create/start/rm并保留其边界。每次CLI调用的stage/runtime容器和relay卷均标记本任务owner并登记；后续操作使用准确ID。最终必须正向读取Docker库存确认21个已登记容器（version1+10次调用×2）和10个卷全不在，任何fallback清理或未知资源状态令验收失败。

准备证据位于`artifacts/qa/main-post-merge-e2e-20261124-080504/installed-cli-acceptance/`：

- `relay-build-01.json`：完整编译argv、初终源码、工具/ELF/image身份。
- `preparation-01.json`：6个离线命令拒绝测试PASS，AST/tabnanny通过，实际runtime/clearance/extra/permit/wrapper摘要全部一致。
- `installed-cli-01/inputs.json`与`review-template.json`：摘要`26fce6b360363c5efd85996a0cb376a115b16adfcb212ac54fde147ed17e230a`，模板明确REVIEW_REQUIRED；SQL0、容器启动0。

父会话审查发现只读image检查与父级cleanup尚未显式绑定Docker endpoint。修正仅限私有runner：所有父级Docker调用使用`runtime.clean_env`与固定`DOCKER_HOST`；cleanup直接检查已读取的ID/name/image/owner；最终同时复核permit和launcher摘要。旧01输入与源码保留，未启动过容器。新`preparation-02.json`记录6个离线测试、AST/tabnanny与实际全部绑定检查通过；新`installed-cli-02/inputs.json`摘要为`f44736a873643801092d57e838f751829d8fea7567a533b5660fb995da6ec619`。

独立审查后运行命令为`python3 .omx/private/qa-work/main-post-merge-e2e-20261124-080504/installed-cli-acceptance/run_installed_cli.py run --label installed-cli-02`。必须由runtime负责人明确交付顺序执行时段；没有因为本机是macOS就将此可准备的Linux兼容性项记为hardware BLOCKED。

## 实际执行与失败分类

父会话已签发accepted review，runtime负责人在前一批17项全部通过且服务清理完成、PG clients0、排他锁不存在后明确交付时段。上述命令实际执行一次，完整收集与记录1个身份、1 FAIL、0 SKIP、0 retry、0 missing/unexpected。

精确旧CLI version控制已经通过。第一条普通任务正向更新还未执行时，Docker/runc在`/fixture/relay`执行入口拒绝访问，返回1；canonical `collaboration-installed-clients.ts:124`要求0因而失败，调用点是spec第93行。原始错误含`permission denied`。这属于测试执行前置失败；暂停、未确认Stop、移除作者三组业务拒绝均未到达，不能给兼容性绿灯，也不能将此归为旧CLI权限漏洞。

当前已知host relay是受审0700静态ELF，image的Config.User为空；canonical cleanup已删除stage/runtime与volume，没有记录复制后文件的UID/mode或mountflags。因此尚不能断言具体是复制所有权、执行权限还是mount边界。建议在新编号的自有诊断中先捕获复制后的UID/mode/noexec条件，独立审查修正后再重跑完整单例。当前未修改fixture/source/权限断言，也未放宽guard取得通过。

实际只创建3个容器（version1、首次调用stage/runtime各1）和1个卷，全都按准确ID/owner登记并确认不存在；正向最终Docker inventory通过，无fallback删除、无错误。API/Web/renderer全部关闭，原gate booleans恢复、epoch2→4保持单调；源码/工具/relay/permit/launcher绑定一致，后代进程检查PASS。runtime负责人独立确认共享PG clients0后已接回时段继续其他已审用例。

证据：`runtime/installed-cli-02/{collection.json,playwright.json,named-inventory.json,cleanup.json,completion.json}`和`installed-cli-acceptance/installed-cli-02/{resource-cleanup.json,process-cleanup.json,completion.json,failure-classification.json}`，均位于本campaign QA根。完整原始错误与摘要保留，`failure-classification.json`提供不含凭据的边界说明。

## 2026-11-25：离线权限诊断与已审03（等待runtime时段）

父会话独立审查后，实际执行既有`relay_diagnostic.py run --label relay-diagnostic-01`。第一次在读取review时退出：review为0644，而私有manifest要求0600；未创建容器。仅修正review文件mode，内容SHA不变，保留`preflight-attempt-01.json`后再次执行，诊断完整结束。

复制后的两个relay均为UID501/GID20，字节SHA均为`9750e2c8173dfb679d087514f841b5693b2d43fe26f74f7d676745d8ce2c27f9`。实际容器UID0、CapEff全零、NoNewPrivs1，`/fixture`为只读ext4且没有noexec。原0700文件不可读取，runc拒绝执行（容器ExitCode126，Docker start退出1）；0755文件可读取且成功进入Go relay，以预期的`installed CLI relay failed`拒绝空stdin并退出1。因此两个Docker退出码虽然同为1，只有0755真正执行了relay。证据明确定位到复制后所有权与文件mode的组合，而非镜像架构或mount执行限制。

本轮没有传入bootstrap、凭据或API地址；relay在创建配置、监听器和调用`/app/multica`前退出。全部6个自有容器和2个卷已按准确ID/owner删除并确认不存在，cleanup_errors为空，源码和工具绑定复核通过。诊断完成不等于旧CLI业务验收通过。

最小修正只改私有`run_installed_cli.py`的relay选择与绑定：保留原0700二进制，在Git外私有0700目录新增字节完全相同的`bin/installed-cli-relay-0755`，要求准确path/owner/mode/hash，绑定新复制回执。原runner原文保存于`before-mode-fix-03/run_installed_cli.py`，精确差异为`mode-fix-03.patch`。原始Docker guard、fixture、spec、240秒超时和全部断言均不变；未加能力、未改容器用户或放宽只读/network-none保护。重复的`offline_copy_probe.py`已移除，从未执行。

准备完成`installed-cli-03/{inputs.json,review-template.json}`，父会话随后签发`review.json`：accepted、blocking_findings_open=0，准确inputs digest为`7687a6bddac5dd5a184df5564b70ad2747fd15ee6f4c4f448dc1b5f3d696e2bd`。本agent复核review内容及digest匹配，review内容SHA256为`de194e3f44c68804d7992bcb69880d37d7ee7bf84e0bb4511cf10f22428919e9`；文件从0644改为loader要求的0600，内容不变。现有6个command guard测试、7个diagnostic guard测试以及5个Python文件AST/tabnanny均通过，父会话也独立复核13个测试及runtime/input/permit/launcher/byte-copy绑定。再次确认重复probe不存在，03无run-start；没有启动服务或执行旧CLI。当前只等待父会话交付已预留的排他runtime时段，届时才可执行：

```sh
python3 -B .omx/private/qa-work/main-post-merge-e2e-20261124-080504/installed-cli-acceptance/run_installed_cli.py run --label installed-cli-03
```

新增证据均在本campaign的`installed-cli-acceptance/`：`relay-diagnostic-01/completion.json`、原始command输出、`relay-mode-copy-03.json`、`mode-fix-03.patch`、两个`mode-fix-03-*_selftest.py.log`和`preparation-03.json`。02原FAIL仍保留，03当前已审待执行。`preparation-03.json`中的research文件摘要对应准备时原文，本节的后续review状态更新不改写该历史回执。

## installed-cli-03实际结果与最终时段交还

父会话授予排他slot后，复核`runtime/slot-handback-cli-20261125-final.json`及其ledger摘要、accepted inputs `7687a6…`、锁不存在和实际PG49908 otherclients0，记录`installed-cli-03/slot-intake.json`，执行一次已审命令。原始单例最终为**timedOut，240005ms，退出1**；1 planned/observed、0 missing/unexpected、0 global errors。未延长240秒超时、未重试，也不将已访问步骤折算为完整PASS。

资源事件记录1次version、10次volume创建、19次stage/runtime创建、10次relay复制、9次relay start、9次runtime删除和9次canonical卷删除。由此可确认已越过原0700权限前置，9次relay启动，最后一次只完成stage复制/删除，尚未记录其runtime创建。步骤数量只说明实际到达边界；原始Playwright报告和API日志保留，不能据此断言全部权限业务通过。超时的具体耗时归因尚未完成。

自有20个容器和10个卷均已不存在；最后一个卷由父级fallback按准确owner删除，因此`resource-cleanup.json`仍为FAIL、errors为空，不掩盖清理fallback。进程清理PASS、无fallback termination和remaining；API/Web/renderer端口均关闭，gate booleans恢复、epoch2→4。来源/工具/relay/permit/launcher绑定不变，数据库与全部失败证据保留。

最终只读复核确认Docker该owner库存为空、所有记录进程已消失、端口和runtime锁关闭、PG49908 otherclients0。**最终交还slot给父会话供LinuxMain03使用**，回执为`installed-cli-03/slot-handback-linux03-final.json`，记录时间`2026-11-25T08:11:30.256124+00:00`。PG仍由runtime负责人持有运行；本CLI分支不会再启动服务或重跑。
