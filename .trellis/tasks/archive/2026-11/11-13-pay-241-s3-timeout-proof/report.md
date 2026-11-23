# S3 受控执行超时恢复报告

本独立增量已完成六个测试/fixture文件及真实API、production Web、Electron renderer验证，原生trellis-check独立检查结论为PASS（T01–T06，见research/check-review.md）。生产执行、授权和超时默认值没有修改；S3其余范围及PAY-241完整24例继续开放。

## 实际验证

| 条件 | 证据 |
| --- | --- |
| T01 真执行超时 | 测试自建进程已启动；15秒执行deadline终止，比150秒callback更早；原记录明确claude timed out after 15s |
| T02 原账本恢复 | 实际API已存outcome/proof，回复持续被拦；旧poller退出后同目录/runtime的另一个PID恢复相同event/receipt/payload SHA |
| T03 竞争和失败历史 | normal/race分别验证Stop先与timeout先；实际显式新command成功，旧失败保留；恢复没有旧run的新claim/permit/launch |
| T04 夹具兼容 | 默认180000ms不变，1000–180000整数；null/0/负数/小数/溢出等拒绝；原Windows/POSIX close四项通过 |
| T05 实际API与两端 | run2通过，评论、输入附件、内存、命令和结果分别不变；Web/Electron可见具体Agent及原失败；新运行成功 |
| T06 验证与来源 | normal27/race27、Go vet、Node4通过；full typecheck9/9、lint6/6；最终gate布尔值恢复，原API/Web/renderer/CLI/Provider文件哈希未漂移；独立检查补记了定向E2E TypeScript/ESLint的完整argv与配置（research/targeted-static-check.json） |

原始结果：research/implementation-results.json、runtime-results.json、runtime-final-audit.json。私有完整日志/截图：/Users/artisan/.multica/acceptance/pay241-20261113-01/。

## 版本和资源

运行源码是feb87aee7的独立副本加四个runtime/E2E文件；另外两个纯单测文件在根工作区normal/race验证，不声称副本包含全部六文件。API18990、production Web13910、renderer6094，数据库multica_pay241_20261113_01_s3_runtime_api。当前CLI、Provider、daemon test binary与页面构建均有记录；最终只读audit核实原gate标志恢复且epoch单调。

run1通过（32.966秒）；其后只在spec补了显式session Resume后的状态重读，最终字节run2再次通过（27.28秒）。保留两份独立报告，不把不同spec版本合并称一次执行。

## 保留的失败

首轮新增单测错误地把准备与物理启动前的同身份permit再核权当作重复执行；修正为比较恢复前后不新增以及唯一许可/launch/终结事实。测试接口定义前编译失败、E2E lint配置不匹配和一条私有config路径错误均按原日志保留，未归咎生产逻辑，也未削弱权限或重试规则。

## 限制

只使用受控Provider和测试账号，不证明真实模型能力、支付/履约、生产发布或安装版Desktop Main。这里只核对一份输入附件保留，不声称Agent输出附件上传幂等。完整S3的批次补读、指定字段竞争、混合通知和阻塞四项仍由父S3继续推进，不能据本报告关闭S3或PAY-241。

## 结项核对（2026-11-14）

续行会话在提交前核对：六个文件的SHA-256与implementation-results.json及check-review.json记录一致，HEAD仍为feb87aee7。在根工作区用同一无数据库配置重跑 `go -C server test ./internal/daemon/collaborationrun ./internal/daemon -run 'TestHostTimeout|TestCollaborationHostHarness' -count=1`，两个包均为ok，gofmt无输出（日志 logs/s3-timeout-resume-20261114-normal.log）。这次只重跑了normal定向单测；race、vet、全量typecheck/lint及两次真实E2E沿用2026-11-13的记录，没有重新执行。
