# 辅助抽屉测试 helper 修复独立复核

**PASS：接受隔离候选中的单字符测试修复；没有业务断言降级或产品源码变更。** 此结论只覆盖 helper 修复和其回归，不代表正在执行的整组浏览器验收通过。

候选 `4c817dee9397a42ac903f419af7804b11d2d863e`。`e2e/fixtures/collaboration-disclosures.ts:18` 的模板字符串需要两个反斜杠，才能把数字类传给 `RegExp`。旧源码只有一个，运行时变成 `d+`，因此找不到真实的 `Attachments 1` / `Sub-issues 1` 按钮。

## 源码与断言边界

- 原 helper SHA-256 `1774801a6860018c39c8c7204c67c16f57ca0dbf330dbb3c7defdbd9297665a7`，与冻结 Git blob 及保存的 before 文件完全一致。
- 修复后 SHA-256 `0dad80fa753db8b73db2622e45cbdf86fd8b91f4f7c00cf2e84186f9fc1075d9`，与隔离树实际 helper 及保存的 after 完全一致；文件只增加一个反斜杠字节。
- 独立对比 apps/packages/server/scripts/e2e：另外仅有 `apps/web/next-env.d.ts` 的明确 `.next/dev/types`→`.next/types` 生成导入变化，没有额外未跟踪产品/E2E 源文件。未靠执行者的 `product_files_unchanged=true` 字段推定一致。
- 保留首尾锚点、资源 group/dialog 范围、drawer accessible-name 断言。More actions、AI execution、动作超时、重试和所有业务 spec 不变。当前固定资源名称没有需要额外转义的 regex 元字符。
- 真实产品 `detail-auxiliary.tsx` 的 `aria-label` 正是名称或名称＋数字计数，drawer 名称仍是基础名称。修复对齐既有产品可访问语义。

## 失败与回归证据

原 `legacy-first` raw 明确记录两条实际原生 E2E 在 helper 第18行失败，匹配器分别为 `/^Attachments(?: d+)?$/` 与 `/^Sub-issues(?: d+)?$/`，均0重试。父会话授权停止后 exit130；原结果必须保留。

该轮解释后的数量为 **7 PASS / 2 FAIL / 2 条件 SKIP / 42 未运行**。框架 stats 的44 skipped包含41无result项及1个零时长、无skip注解的未启动项，不应把44全部写成已核查的条件排除。

执行者新增私有真实 Chromium 控制 `helper-browser-regression-2`：**3 PASS / 0 FAIL / 0 SKIP / 0 retry**。已独立读取其源和 raw：它直接导入隔离树的实际 helper，没有复制或 mock helper 实现；四种资源名称分别覆盖无计数/12、关闭 drawer 开启和已打开 drawer 内再次选择，并覆盖 More actions、AI execution 两个未变分支的关闭/打开状态。该 DOM 是合成控制，不冒充应用完整验收。

Reviewer 另外从 before/after 文件中原样提取 `new RegExp(...)` 表达式进行 **56项纯 Node 断言**：四种名称的裸名称、0/1/12计数、错误的 literal d、前缀和尾随文本。原表达式拒绝数字并错误接受d，修复后接受数字且拒绝其余；首尾锚点保持。

首个私有控制 `helper-browser-regression` 因 TS 引号写法错误，0项进入测试、exit1；已保留原始错误结果。这是私有回归脚本前置错误，不计产品失败，也不覆盖后续3项成功。

## 结论限制

修复后的生产 Web / 原生全量用例仍由执行代理取证；不等待它们即可接受此单字符测试修复，但不能提前宣称原业务链路已通过。后续结果需声明 helper variant，并保留原失败。新小队 diagnostics 单元失败和 detail 滚动恢复失败独立存在，此修改没有修复它们。

本 reviewer 仅写 `harness-review.md/.json`，无源码、数据库、服务或模型操作。未额外运行 lint/typecheck；helper 已由3项真实浏览器控制导入/transpile。逐项原始路径、SHA-256、失败身份与检查结论见 [harness-review.json](harness-review.json)。
