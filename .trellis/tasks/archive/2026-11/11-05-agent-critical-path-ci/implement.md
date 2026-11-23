# 实施与所有权
1. 独立计划审查。主会话负责资源分配、scope决定、Git/spec/任务文档。
2. trellis-implement独占新workflow、窄smoke launcher与inventory validator/regressions、独立Playwright smoke config（若需要）。不改业务源/现有e2e测试与fixture，不改lockfile/dependencies；若发现真实缺陷上报主会话。
3. 先为inventory failure形态建立真实red，再实现；验证workflow触发、DB先决、非空库存、退出码、日志保留、进程清理。可用fake进程测shell边界，但不能冒充真实浏览器通过。
4. 本地先静态node/shell/配置type检查，再生产Web构建/隔离API/4路径一次实跑。日志放Git外。主会话授权准备后child按exact配置执行；不自己创建/迁移/清理DB。
5. trellis-check独立检查+实际库存，主会话验收并Lore原子提交。

独立计划审查通过：plan-review.md 的具体环境/启动清理/证据allowlist/最终source身份合同全部采纳。最后实际run前冻结其他lane包括tracked任务文档改动。
