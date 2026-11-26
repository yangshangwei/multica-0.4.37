# Release OMP fixture 失败独立因果审查

日期：2026-11-26。目标tag v0.8.8仍指向d2c9240eaf9f768446faba93cac625bd63e9cf53。当前为证据审查，未重跑CI、未修改tag/源码/产品，不读真实账号。

## 原失败与源码事实

Release run45721755253 attempt1唯一明确顶层失败TestOmpExecuteDefaultsToOmpBinary，结果failed/omp prompt write failed: write |1: broken pipe；changelog/images/publish全skipped，artifacts为空，Release不存在。原始失败不能因随后绿色被改记通过。

omp_test.go的fake executable用#!/bin/sh，并调用裸cat读取stdin，但同一个测试把PATH设为仅有fakeDir，目录只写入omp没有cat。/bin/sh可以由绝对shebang启动，cat却不可解析；脚本没有set-e，失败后继续printf协议事件并exit0。该fixture不满足真实Pi先读stdin到EOF的契约。

pi.go先Start真实进程，再用goroutine写prompt并关闭stdin，与stdout读取并行避免大输入/输出死锁；等待进程结束后读取writeErrCh，任何写失败仍将completed改为failed。这个保护正确，不能为通过测试吞EPIPE或改成成功。已有pi_test.go注释也明确退出未读stdin会产生与测试业务无关的负载相关EPIPE。

## 已独立读取的控制证据

source-binding.json的omp_test.go、pi.go、go.mod/go.sum及agentCLI guard五文件，均独立与d2c Git对象及当前工作区逐字/哈希匹配。

shell-proof.py在私有目录创建真实/bin/sh进程，PATH仅fakeDir：原fixture先等待child退出，再写prompt，真实write errno32/EPIPE，stderr明确cat缺失而child exit0；builtin-read对照在写之前仍阻塞等待stdin，读完后exit0且write无错误。它确定性证明机制，不假装完整重放CI精确调度或Windows行为。

Go private overlay用于控制，未改tracked文件：builtin版本只把cat一行换成POSIX builtin read循环，原prompt/断言/超时不变；negative保留错误fixture，扩大prompt并要求实际产品返回failed且含prompt write failed，用于证明产品保护仍在。不能把overlay测试当作未修改tag测试通过。

初始未修改tag的normal100次：100run/100PASS、packagepass、无skip/fail；该结果说明已知fixture时序性，不说明cat问题消失。其余结果待实施者回执后补。

## 同SHA原样重跑的判断原则

当控制结果确认此为fixture前置缺陷、产品拒绝写失败保持，允许主会话对**同一个不可变tag/SHA、完全相同workflow和全部门槛**发起一次正常重跑，不移动tag、不注入私有overlay、不修改PATH改善CI、不跳过测试/漏洞检查、不上传手工生成feed绕过依赖图。这个动作是重执行原验证，不是修复fixture。

若原样重跑成功，可记录“attempt2原门槛通过，attempt1已确认的测试夹具缺陷保留”；不能宣称fixture已修复、所有测试无缺陷或第一次失败已排除。已知测试缺陷需要单独后续修复提交。若重跑仍失败、出现其他产品/安全问题或需变更源码才能通过，则不能无限rerun刷绿；主会话须按不可变标签策略处理新版本或明确阻塞，不能替换已标tag的源码与构件。

本结论目前待最后normal/race/控制原始事件回执核验后确定。没有权限/安全测试规避建议，产品stdin保护明确不可改变。


## 最终证据核验与裁决

**接受这是已证实的测试夹具前置缺陷；允许主会话保持同tag/SHA和完整原workflow不变发起正常重跑。** 不免除任何最终门槛、不批准未验证发布、不将重跑绿当修复。

24项manifest文件长度/哈希逐一重算匹配；源码5文件、Git对象、当前tag仍d2c，tracked工作区无改动。五份原始Go JSONL独立解析：

| 运行 | 次数 | 实际结果 | 来源 |
| --- | ---: | --- | --- |
| original normal |100|100run/100pass，packagepass，0fail/skip/unfinished|未修改tag测试 |
| original race |100|同上|未修改tag测试 |
| builtin normal |100|同上|私有overlay，仅cat替换builtin read |
| builtin race |100|同上|同一私有overlay |
| write-failure negative |20|20pass；实际生产session均failed且含prompt write failed|私有大prompt/负断言overlay |

平台均Go1.26.11 darwin/arm64，不冒称Linux runner完整重现。normal/race小prompt200次原测试通过不证明cat存在或fixture正确；操作系统/调度不同仍可能触发CI observed EPIPE。确定性shell控制证明cat缺失导致先退出，而大prompt负例证明真实piBackend错误保护拒绝写入失败；builtin只读stdin修正对照不改产品。此前source reading与唯一CI失败相符，未发现需要改产品的证据。

原stdout统计cat_missing_events=0不是反证：原Go fake stderr经过logger处理，当前输出没有匹配该计数文本。确定missingcat证据来自单独保留shell-proof.stderr；不会把Go零统计解释为缺陷消失。negative日志统计prompt_write_failure_events=0同样不替代测试断言；负例源码明确检查结果failed且包含prompt write failed，原事件20pass和实际statusfailed日志匹配。

Final report必须单独注明：Release attempt1失败，fixture内裸cat在PATH-only环境不可靠；该已知缺陷仍存在于tag。若后续原样完整CI通过，只说明该次门槛通过，不能声称测试修复/所有历史通过。后续应以独立提交修复fixture。若本次仍失败或新证据变成产品问题，应停止将反复retry作为替代修复，遵守不可变tag。

本审查未发起CI重跑、未使用overlay执行发布或改任何产品/标签。原研究报告release-omp-fixture-analysis.md已读，范围及保留限制一致。

| 证据 | SHA-256 |
| --- | --- |
| `summary.json` | `b329a0074f25b7b82b337818995f974c8caea482a7471b5379ae31fdc731c610` |
| `source-binding.json` | `8c3cf0123ba8d0048dc6a08bd620b42ffa3d9cfde7a0c133ad9cf558ef93c2af` |
| `evidence-manifest.json` | `40d1d26ce3092543ee8dfdea04654aedb7036269a91ae6d889c532f2d2c65224` |
| `shell-proof.py` | `c82fb134ac919273e257ef115b5f4d903b6d83fa4debe73c773d1d0b61a05c9c` |
| `shell-proof.json` | `cd8d78ca33e659429f6e477b760b08e0b704f677fe808c64b3a41dccc5d65dcf` |
| `original-normal.jsonl` | `09bc993e8ce2c68c3e61b57ff097ce2523300b8a8068a674293b8d8de9fd5b9b` |
| `original-race.jsonl` | `80b139e8133025352427088a9b5d357a3773f1d71af963f38b6c3b0b009939f4` |
| `builtin-normal.jsonl` | `b1df2877f59fe2899a57ef23f66d15ff596c26ac4e7e03af15e0340633924861` |
| `builtin-race.jsonl` | `29cb270abef235f71e7cd5327f9323f1f4b0607a0ca51d95ee6a1b6f95ca8110` |
| `write-failure-negative.jsonl` | `5494b23d2e56de27e8fcc6f29b5985fa69fae30224c5b708454bbcfb81725ae2` |
