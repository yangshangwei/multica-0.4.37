# Windows daemon early-exit 诊断扩充独立审查

日期：2026-11-26。仅审查diagnostics.mjs/config.node-test两文件；不改产品/控制器/工作流，不复测home或全量套件。

## 结构与隐私

独立从controller源码提取并逐字比较20个diagnosticCodes、8个log markers、5个cli/connection阶段，与新增投影完全一致。固定字符串交集不会接受对象或未知字符串，去重后结果最多allowlist长度；输入仍由此前64Ki marker单行/128Ki文本/16failure限制保护。

字段读取对应实际来源：top mainErrorCodes；profileFiles的daemon.log/daemon.err.log读取errorCodes/markers；actual.diagnostics.cli/connection读取code/stage/httpStatus；config.json读取serverMatches/deploymentMatches/deploymentPresent/credentialPresent；.desktop-user-id读取userMatches。没有拼错字段或复制原对象。仅固定config/owner文件拥有新增比较布尔，两log拥有日志码/标记，其余文件不被扩大。

HTTP限定整数100–599，其余null；connection非对象/数组返回null；未知code/stage为unknown。布尔严格保留true/false，缺失或错误类型为null。未公开任何地址、路径、PID、用户/部署值、token、原log/text、日志hash、byte数、错误message或stack。

## 定位覆盖发现

controller的recordMainErrorCodes除了20codes，还会把4个固定日志事件映射为managed_setup_failed、pat_mint_failed、cli_auto_install_failed、credential_restart_failed。它们都是现有固定派生码、没有秘密。初始新增main_error_codes只与20codes交集，会丢掉这4类重要启动事件。

已建议Main专用allowlist=20+4，log仍严格20，补四码保留以及不得穿透log的负例。该补齐不需要扩大原始日志公开或改controller协议，可减少下一run仍无可用原因的概率；不是保证所有错误都已覆盖。

## 独立验证

- 当前5个daemon窄测试PASS/0FAIL/0SKIP，覆盖秘密/未知/object/duplicates/畸形/超限marker。
- 额外直接调用实际projection验证HTTP99/100/599/600、浮点/字符串/bool/null边界，数组/primitive connection拒绝，以及config/owner wrong-type→null、显式false保留。
- node --check、git diff --check通过。
- 本机未执行Windows原生、未读取已消失runner私有日志；这些检查证明诊断内容安全与类型正确，不证明daemon退出原因或产品修复。

仅新增本报告，未修改实现。最终补齐复核如下。


## 最终复核与结论

**PASS：两文件可提交并用于新native诊断run，无剩余阻断问题。** Main专用allowlist已补为原20码+4个精确派生码，log及connection仍仅20码；新增测试确认4个Main码保留、同码在log过滤、未知值拒绝。与controller原固定映射逐项一致，没有泛化为任意错误字符串。

独立最终daemon窄测试**6 PASS /0 FAIL /0 SKIP**，syntax及diff检查通过。此前HTTP/strict-bool小控制仍适用。源码/行为未扩到产品或startup policy；既有原生失败仍未判定原因。

| 文件 | SHA-256 |
| --- | --- |
| `apps/desktop/scripts/windows-collaboration-diagnostics.mjs` | `c410392736213ce845a7bc539a243e6a5c635980c7821c2f8a060586ea741b6d` |
| `apps/desktop/scripts/windows-collaboration-config.node-test.mjs` | `089c77913f046f5053f2ded4ea2151d4f518580da558bc686db3aa694db8ef05` |
