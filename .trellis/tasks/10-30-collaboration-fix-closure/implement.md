# 首轮协作修复交付闭环实施计划

1. 创建独立 worktree，记录 85b98c3bb、8 个修复链、工具版本和主工作区状态。
2. 将 8 个修复合入实施分支；解决只影响集成 spec/生成文件的冲突，保留 85b98c3bb 的 core action changes。
3. 修复并验证迁移唯一前缀；运行 sqlc 和 generated diff。
4. 先写附件预算边界失败测试，再实现引用必保/可选历史尽力补位；覆盖小 FileCount、PerFile、Total、Git 保留、引用超限、引用丢失和小队不同 runtime。
5. 修正文案动作映射；补接收方 handoff e2e 可见性和正常状态选择断言；补 comment 64 KiB+1 服务层不落库回归。
6. 在最终 HEAD 运行 migration、sqlc、Go vet/build/targeted tests、pnpm lint/typecheck/test 和相关生产 Web e2e。
7. 硬化实验框架：固定 source/framework/model/CLI metadata、trials=3、完整 prompt/tool/outcome/proof、durable verdict；用 fake provider 验证。
8. 串行运行正式 E0，成功后按 E1–E6 预登记顺序运行并保存原始证据。
9. 更新问题→修复→验证→限制矩阵和首轮验收报告。

首轮合入门槛：迁移 lint 与 sqlc 无漂移；附件矩阵通过；B/C 证据补口通过；最终全栈检查有版本化日志；正式 E0 通过，或环境故障被完整记录且明确不宣称产品通过；E1–E6 结果不被环境失败伪装成成功。
