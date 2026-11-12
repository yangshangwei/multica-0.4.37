# 实验续跑入口复核

本次只修改两个 `agentintegration` tagged 测试文件。产品代码、预登记场景正文、评分、模型、原定三轮及每次超时均不变。

- 修复前的回归证明，非法轮数/起始轮次会进入 Provider 版本探测。现统一在该探测之前拒绝。
- `MULTICA_CONTEXT_EXPERIMENT_START_TRIAL` 未设置默认 1；提供时仅接受 1–3，显式空值也拒绝，防止配置遗漏导致重跑 trial 1。
- 独立完整入口验证：默认执行 `1,2,3`；`START_TRIAL=2` 仅执行 `2,3`。失败后不启动下一轮；E6 只向对应第二跳转交第一跳的 memory，保留原 trial/hop 编号。
- 实施代理与独立检查代理均只调用测试创建的假 Provider。正式入口在 real-agent gate 未设置时跳过；gofmt、tagged Go 编译、`go vet` 与 `git diff --check` 通过。
- 同一最终框架在 baseline `c58f79874` 与 after `835e7b046` 的假 Provider 套件均通过。after 的 server 产品源码与正式 E0 的 `4c833e737` 无差异。

最终框架 SHA-256：`f6f62e55888fa2d3e733ff12dec6054e07f877828363ad0d503ada6db07aa0bd`。各文件 hash 与实际源码 HEAD 见 `framework-manifest.json`；红/绿验证、独立复核、完整入口及 baseline 复核的原始证据在 `verification/`。其中 `implementation-*` 是复核空值修正之前的版本，最终版本以 `review-framework.json` 和根目录 manifest 为准。

起始轮次由协调者依据历史持久化记录指定，框架不自动推断历史。无效样本不补跑，不以 Go 退出码替代语义裁决。续作预算与顺序在 `prerun.json` 运行前固定。
