# 原系列第 3 次续作 — 2026-11-01

本次从 E2a trial 3 开始，按登记顺序串行运行，在 E3 after trial 1 遇到第一个不完整结果后停止。共 8 次 Provider 执行：7 个有效样本全部符合预登记判据，1 次无效尝试。剩余上限 **14 次**，下一入口 **E3 after trial 2**。

| 场景 | 槽位 | 结果 |
| --- | --- | --- |
| E2a baseline | trial 3 | 有效，答 `2026-11-14`，通过；81 秒，读上下文 1 次 |
| E2b baseline | trial 1–3 | 3 个有效，均答 `2026-11-14`，通过；44–93 秒 |
| E3 baseline | trial 1–3 | 3 个有效，均只发 `BETA`，通过；90–158 秒，每次写入 1 次 memory |
| E3 after | trial 1 | 无效：`claude timed out after 5m0s`，原始判定 `provider_failure_unclassified`；trial 2、3 未运行 |
| E4、E5、E6 | — | 未运行 |

固定来源与登记一致：baseline `c58f79874`、after `835e7b046`，框架 SHA-256 `f6f62e55…a0bd`，Claude CLI 2.1.316，请求模型 `claude-opus-5-5`，thinking/tier 为 provider_default。实际生效模型未观察到。启动器 `run_scenario.py` 每次启动前核对 HEAD 和三份框架文件哈希，去掉 Claude Code 会话专用环境变量，不覆盖已有目录。

## 需要复核的事实

- **E3 baseline trial 1 有两条 `BETA`。** 第二次 POST 与第一次逐字节相同、使用同一 request id，服务端返回同一 comment id，属于幂等重试，不影响判据。
- **E3 after trial 1 的超时经过。** 框架哈希过的记录里没有 API 错误文本，但 Provider 自己的本机会话记录（运行后读取；正文含操作者私有指令，不入库，只在 `E3_member_scope-after/provider-session/provenance.json` 记录 sha256 和来源）显示：04:21:06Z 已成功写出 memory 更新文件；04:21:38.923Z 出现 `system/api_error`，EPROTO，TLS 握手失败，重试 1/10；之后约 55 秒没有响应，直到 5 分钟上限被终止。不是 429。同场景 baseline 三次为 90–158 秒。
- **窗口偏差。** E3 after 启动前的检查显示，会话 multica-0-4-37-87 的记录 29 秒前有写入，启动器没有据此暂停。本 trial 期间至少有 4 个其他会话的请求在途（含 87 和 0b），baseline 期间为 0。这支持环境类归因，但不能确定超时原因。下一次启动前必须确认所有会话的记录都静默至少 120 秒。
- **判定已独立复核。** 会话 multica-0-4-37-0b 的复核（`../review-e2a-resume3-2026-11-01.md`，`2fc5c962c`）同意全部有效/无效分类和 7 个通过。
- **对预测的影响。** E3 baseline 为 3/3，与“修复前 ≤2/3”的预登记预测相反；两臂 fixture 都提供 `dispatch_comment`，所以补齐 E3 after 只能发现退化。E2a 只有 1 个有效样本，“≥2/3”的预测无法检验。

本证据只属于原固定产品实验，不覆盖新 runtime `0e1ee8e1e` 或知识版本 `96cedf485`。证据入口：`prerun.json`、`run_scenario.py`、各场景目录的 `command.json`、`go-test.log`、`records.jsonl`、`records.jsonl.trials/…/events.jsonl`、`semantic-verdicts.json`，以及 `run-summary.json`。`evidence-sha256.json` 记录完整性。
