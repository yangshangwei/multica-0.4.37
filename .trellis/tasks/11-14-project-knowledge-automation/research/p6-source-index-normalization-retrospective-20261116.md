# P6 源码接入：Git 索引换行校验回归

## 1. 根因分类

主要是隐式假设和测试覆盖缺口：初版执行器用原始文件字节计算 Git blob SHA1，假定其必须等于索引中的对象。仓库已有 `* text=auto` 和 `*.bat text eol=crlf`，索引会保存规范化后的 LF，Windows 工作树保留 CRLF。

实际失败只有 `scripts/uninstall-desktop-windows.bat`：15,794 原始字节、389 处 CRLF，执行位正确。索引对象等于 LF 规范化后的对象，文件本身未变化。失败发生在私有源码接入执行器，不涉及知识生产或消费接口。

## 2. 初始检查遗漏了什么

最初六个隔离夹具覆盖文件类型、权限、部分接入回滚、配置和链接保留、Git 控制文件及索引路径集合，但没有带仓库换行属性的 CRLF 文件。它们通过后，实际完整源码才暴露错误假设。

新增 `test_crlf_bat_keeps_raw_source_and_uses_repository_clean_index_blob` 在旧执行器上产生 7 PASS／1 FAIL。确认根因后，v2 只为需要规范化的对象调用已固定版本 Git 的 `hash-object --path=... --stdin`；原始 SHA256、完整权限、路径发现和工作空间链接仍分别严格核对。

## 3. 已落实的预防措施

| 优先级 | 措施 | 证据 |
| --- | --- | --- |
| P0 | CRLF 与 Git clean blob 分别验证；原文件和 `.gitattributes` 不改写 | v2 GREEN：8 PASS／0 FAIL／SKIP |
| P0 | `test_crlf_source_drift_is_refused_before_clean_blob_acceptance` 保证相同 clean blob 不能掩盖原始源码漂移 | 同一八项夹具集合 |
| P0 | 失败记录和一次许可保留；完整回滚后才允许新接入 | `9b0291c83697` 失败及独立 rollback 核对 |
| P0 | 新执行器独立审查后，用新许可重放完整源码接入 | `a25e174383fa`：26 项操作，source `8877eb98…`，配置和 allocation 不变 |
| P1 | 将 raw source／Git index 双重契约写入规格，包含命令、拒绝矩阵和回归断言 | `verification-environment.md` 新增七节契约，独立检查 0 阻塞 |

## 4. 范围核对

原 `p6-runtime.py` 的 `stage_sources` 已保留原始字节并单独校验 `source_snapshot`，没有该错误的原始 SHA1 比较，因此无需改动产品、原控制器、Windows 脚本或换行规则。Git blob 的合法规范化也不扩大 Next 生成声明文件的既有唯一例外，不允许其他源码或权限变化。

后续冻结、构建和源码接入工具应分别说明“原始文件身份”和“Git 对象身份”，并在真实仓库属性下验证索引。新执行器、原失败、原执行器及旧许可均在私有目录保留；不删除历史证据以获得通过。

## 5. 知识记录

已更新 `.trellis/spec/server/verification-environment.md`，独立规格检查见 [p6-git-raw-clean-spec-independent-check-20261116-c6ff88993205.json](p6-git-raw-clean-spec-independent-check-20261116-c6ff88993205.json)。本应用仓库没有需要同步的 Trellis 模板源。提交仍沿用原任务的独立授权范围。

其他关键证据：

- [原失败后的完整回滚独立核对](p6-failed-adoption9b-rollback-independent-review-20261116-8deb2f1de8da.json)
- [v2 执行器独立接受](p6-root8877-adoption-v2-executor-independent-admission-20261116-00c46c50164d.json)
- [实际 v2 接入结果](/Volumes/artisan/.multica-acceptance/knowledge-automation-root-next-preparation-20261116-8fe979e10bad/reports/actual-adoption-a25e174383fa.json)

本记录证明索引回归修复和源码接入执行结果；生产构建、Web／Desktop 旅程及最终 AC 仍由后续各自的实际证据核销。
