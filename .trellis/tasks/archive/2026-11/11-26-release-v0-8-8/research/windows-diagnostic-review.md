# Windows installed collaboration 诊断独立审查

日期：2026-11-26。结论：**四文件可提交并用于下一原生诊断run；未发现阻断此次诊断提交的问题。** 这不是原8.6秒协作失败的根因定位/修复完成，也不是Windows安装协作或Release通过。

## 变更边界

当前diff包含3个tracked文件和新增 `windows-collaboration-diagnostics.mjs`；新测试集中在既有config.node-test中，没有独立新增重复测试文件。报告读取实现说明及整个driver/helper，核对原workflow upload glob和真实E2E用例。没有修改工作流上传范围、超时、重试、E2E业务过程或产品实现。

driver保留既有源码/安装目录/哈希/进程环境限制，将阶段推进记录为固定集合。Playwright失败后raw stdout/stderr只写phaseRoot内private文件，原错误仅供本地projection，公开error改成固定文案。失败报告不复制journey原对象；只使用附件存在的白名单布尔里程碑。成功journey直到源码、结果和截图检查全部通过后才赋予公开report。

## 信息边界

- public diagnostic只包含固定stage、允许的错误名称/代码/signal、受限非负整数退出码/stats、固定结果状态、最多8个已知repo相对文件+行列位置、固定milestone存在性布尔和固定attachment状态。
- stack仅分析frame行，公开路径必须命中固定sourceFiles集合且绝对路径匹配当前repo；函数名、根目录、未知路径、message、expected/actual、源代码片段、标题、附件body、stdout/stderr、环境和命令都不转发。
- result数组最多16，suite遍历深度受限；无效JSON返回unavailable，未知枚举归unknown/null。synthetic秘密测试覆盖错误文本、stack、断言字段、stdout/stderr、附件及未知路径，验证它们不进入公开结果。
- 失败附件只能在当前output目录内、真实路径相等的普通文件且不超过16MiB；异常只产生rejected状态。附件内容即使含秘密也只投影固定milestone presence，不序列化字段值。
- workflow仍仅上传runner.temp下原有 `multica-windows-collaboration-*.json/png`，未加入私有phaseRoot或playwright输出目录。mode0600在Windows不等同独立ACL证明；本次安全边界是任务私有路径和受限上传集合，不能宣传ACL已验证。
- 精确措辞：测试子进程失败分支不会复制截图；如果测试本身已PASS而第二张截图复制失败，既有逐文件复制仍可能留下第一张已生成截图。不能把所有最终status=failed泛化为“绝无截图”，但本修改未扩大该旧有成功路径的截图上传。

## 用例选择

实际spec有原knowledge-OFF完整旅程与另行opt-in的default-ON旅程；原driver要求1PASS/0SKIP，但无grep时发现2例。新增常量精确选择原OFF完整标题并以$锚定，保留 --workers=1、--retries=0、540秒进程预算，以及expected1/unexpected0/skipped0/flaky0、1case、1result且passed的全部成功断言。没有删除或skip任何原用例；ON依然存在但该Windows lane明确不覆盖。

这修复一个可重现的选择契约不匹配，不证明此前exit1由该问题引起。8.6秒与六份制品哈希只能证明已到子进程阶段，真正错误位置需下一run的diagnostic。

## 独立验证

- `node --test` 直接受影响的config/workflow/candidate-version三文件：14 PASS / 0 FAIL / 0 SKIP。
- workflow十个Node文件组合（含experimental-strip-types）：**48 PASS / 0 FAIL / 0 SKIP**。
- 真实locked Playwright `--list --reporter=json`：全文件2例；添加本次grep后恰好原OFF完整例1条，无执行测试体。标题与常量匹配，ON未选入。
- driver/helper `node --check`、`git diff --check`：PASS。
- 未运行Windows安装App/真实协作、PowerShell或重型构建；不把本地projection/selection测试当产品流程通过。未运行全仓lint/typecheck。

## 文件摘要

| 文件 | SHA-256 |
| --- | --- |
| `apps/desktop/scripts/windows-collaboration-config.mjs` | `ced9b00f9998b64622d2ac30f0be18ac95186502db3fe63b3335af7bf19939fc` |
| `apps/desktop/scripts/windows-collaboration-config.node-test.mjs` | `fd3162a9ac61c7184cc04d1d6262a89721d390047ceb80f27003eb82a3bff9d7` |
| `apps/desktop/scripts/windows-collaboration.mjs` | `89a6f9f837a0f13fc44614434f9395cb627b62e79de2d6a7a4ab7c2c4bcf48ef` |
| `apps/desktop/scripts/windows-collaboration-diagnostics.mjs` | `f5030300a008df2f9f3c16a15deba5e247633d3246f70373e03fac8b03f77618` |

只新增本报告，没有修改四文件实现。下一run的失败仍必须保留，若有产品修复需重新冻结并重建受影响构件。
