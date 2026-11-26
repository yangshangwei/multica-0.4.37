# Windows CRLF 与原生测试夹具独立审查

日期：2026-11-26。范围：当前三个测试文件差异及 `research/windows-crlf-fix.md`。仅审查、窄测试及临时副本验证；不修改产品、工作流或原始证据。

## 结论

**可提交并用新SHA重新触发Windows原生构建/验收。未发现阻断提交的问题。** 本地结果不替代Windows的Git sh/cygpath、只读属性及CreateProcess真实执行，下一原生run必须通过；原失败保留。

直接查询 run45660381726，headSha为7a99271341910b58c35a99d5f38ffe1ae75c6959，checkout已成功，失败步骤是Validate acceptance scripts。读取其失败日志，workflow多行契约出现null匹配/TypeError，符合CRLF输入与LF-only正则的失配；不能称此run已完成后续构建或安装。

## 逐文件判断

1. `windows-collaboration-workflow.node-test.mjs`：仅在workflow文本读取边界归一化CRLF为LF。未改多行安全正则、独立opt-in、cleanup always、禁止上传私有状态、安装后协作和数据库先验检查。
2. `collaboration-host-runtime.node-test.mjs`：在实际Host源码读取边界归一化，仍抽取并执行真实close体。当前真实close在EOF前调用mark并返回timeline；vm原来缺这两个词法依赖。新增依赖注入与调用顺序/返回同一引用断言匹配实际代码，并保留EOF、Windows无POSIX信号、POSIX SIGCONT、20秒fallback、SIGKILL退出9、错误与账本、socket/server及私有目录清理、重复清理拒绝等原断言。时间线stub仅验证事件与顺序，没有假装验证真实单调时间值。
3. `collaboration-installed-main.node-test.mjs`：被测发现器确为helper生成的Linux容器用无扩展名#!/bin/sh脚本。Windows以明确的 `%ProgramFiles%/Git/usr/bin/sh.exe` 加脚本文件参数执行原内容，通过同目录cygpath.exe的参数数组转换脚本/marker路径，避免误用WSL/default bash及shell字符串拼接。工具或环境缺失、转换/启动错误均失败，不skip。保留原version成功、无marker、任务调用exit97、固定marker且无敏感输入、cleanup断言；目录含空格增加实际路径压力。POSIX仍直接执行原脚本，新增0555精确检查。machine-id0444断言未放宽，新注释正确限定Windows只读属性，不能当ACL隐私证明。

没有删测试、设置skip、增加超时/重试、改workflow门槛或修改产品helper。修复界限针对测试运行环境与已存在的观察依赖，未替换生产生命周期实现。

## 独立验证

- 按workflow使用 `node --experimental-strip-types --test` 运行全部10个相关Node测试文件：**44 PASS / 0 FAIL / 0 SKIP**。包含真实本地provider子进程、HTTP/WebSocket、更新feed、受控发现脚本和cleanup/错误传播。宿主为macOS，Windows专有sh/cygpath分支未执行。
- 用临时目录复制实际workflow、读取的PowerShell文本、Host源码和两份测试，分别转换全部输入为LF/CRLF，原测试不改：各 **12 PASS / 0 FAIL / 0 SKIP**。
- 负例保留断言能力：仅在临时workflow移除协作cleanup的always，LF/CRLF各2 PASS / 1 FAIL；仅在临时真实Host close移除mark调用，LF/CRLF各8 PASS / 1 FAIL。预期失败均退出1，没有把它们计入正常通过数。
- 三份mjs `node --check` 均PASS，`git diff --check` PASS。
- 未运行全仓lint/typecheck及PowerShell原生测试；此三文件没有TypeScript生产改动。没有声称整条Windows Validate acceptance scripts步骤已经通过。

## 冻结与后续

提交会改变源码SHA，保留45660381726原失败并用新SHA重新dispatch。最终所有构件来源应绑定新SHA；原来的44 PASS和本报告不能代替Windows之后的构建、安装、自动升级或业务验收。

## 审查文件摘要

| 文件 | SHA-256 |
| --- | --- |
| `apps/desktop/scripts/windows-collaboration-workflow.node-test.mjs` | `4316a864c51d4fbe36e0c83d0d95377e39c038fa93bf3df4626cba36c3470aeb` |
| `e2e/fixtures/collaboration-host-runtime.node-test.mjs` | `1ebbe0a762475f510274430139eb60c9dc5f61083ed0a7523ac0bcd256a4d25a` |
| `e2e/fixtures/collaboration-installed-main.node-test.mjs` | `34d0774868e94ecd3f302e42a6cc85ba6b2f98a543d4f403567669e98e5aa689` |
