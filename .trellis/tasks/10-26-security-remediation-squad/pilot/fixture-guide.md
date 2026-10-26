# 阶段一合成实验室执行指引

`fixtures/` 为实际角色试点准备了可以复现的失败基线。它不代表 AI 小队已经执行、路由正确或阶段二已获准启动。

1. 在确认隔离运行身份和非生产目录后，只把 `fixtures/` 复制到独立试点仓库。保留 `baseline-source-sha256.json`，记录初始 commit/ref。不要把整个产品仓库、个人 HOME 或凭据复制进去。
2. 从副本目录执行 `python3 -B validate_fixture.py`，再执行 `python3 -B -m unittest -v test_dependency test_authorization`。预期基线为结构通过、六项契约中两项失败；错误类型应是断言失败，而不是导入或环境错误。
3. 依赖案例引用 `advisory.json` 的 `SYNTHETIC-DEPENDENCY-001`、锁文件及真实调用路径。先做受影响性与迁移审查，再派实现；固定版本有明确的调用方契约差异。一次只处理这一个虚构依赖。
4. 授权案例先派诊断成员从普通内存记录复现工作空间拒绝失败，再派实现成员做最小修复。不能修改基线断言或把读取审计误当作授权前可执行的无关动作。
5. 其他风险案例引用 `supply-chain.json`。审查员应列明信任边界、缺失材料和 `pilot-build-owner` 的补证验收条件；不能填入猜测的摘要、签名、SBOM 或成功结论。
6. 未修改补丁的 QA 成员在最终副本上独立执行相同命令；保留命令、退出码、测试输出和代码 ref。依赖与授权修复应使六项契约全部通过；供应链案例继续明确保留 `hold/unknown`。
7. 从真实任务评论、activity 和执行结果回读角色路由、停止条件、下一 owner 与残余风险，填写 `evidence-template.md` 和 `trial-results.md`。本实验室的本地测试输出不能代替这些运行证据，也不能用于宣告内置模板固化准入通过。

阶段一期间保持 `fixtures/` 的故意缺陷基线；实际修复发生在隔离副本中，不改产品代码。已有的 `cases/production-escalation.md` 继续仅用于人工审批与交接演练。
