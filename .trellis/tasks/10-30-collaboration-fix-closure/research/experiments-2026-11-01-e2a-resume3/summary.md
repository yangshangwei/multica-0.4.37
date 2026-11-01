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

- **E3 baseline trial 1 有两条 `BETA`。** memory 说明这是同一 request id 的幂等重试，服务端返回同一 comment id。预登记判据只要求每条评论都是 `BETA`，因此仍记通过；第二次 POST 是否可以接受留给复核。
- **E3 after trial 1 没有 API 错误文本。** 模型已在派活评论下发出 `BETA`，同 request id 重试一次，正要写 memory 时到 5 分钟超时。同场景 baseline 三次为 90–158 秒。
- **窗口偏差。** E3 after 启动前的检查显示，会话 multica-0-4-37-87 的记录 29 秒前有写入，启动器没有据此暂停。87 的用户回答了它的提问后，它在约 12:17–12:21 发出了模型请求，与本 trial 重叠。重叠是事实，是否导致超时无法确定。下一次启动前必须确认所有会话的记录都静默至少 120 秒。
- **语义判定是自评。** `semantic-verdicts.json` 由运行本系列的同一会话按预登记判据写出，仍需独立复核。

本证据只属于原固定产品实验，不覆盖新 runtime `0e1ee8e1e` 或知识版本 `96cedf485`。证据入口：`prerun.json`、`run_scenario.py`、各场景目录的 `command.json`、`go-test.log`、`records.jsonl`、`records.jsonl.trials/…/events.jsonl`、`semantic-verdicts.json`，以及 `run-summary.json`。`evidence-sha256.json` 记录完整性。
