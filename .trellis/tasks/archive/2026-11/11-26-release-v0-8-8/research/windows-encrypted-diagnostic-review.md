# Windows failure-only 加密取证独立审查

日期：2026-11-26。**最终PASS：四文件可提交，用已授权公钥启动下一native取证。** 未发现剩余阻断提交问题。本审查未读取parent本地私钥或任何真实私钥文件；所有加解密测试只使用内存中新生成的临时RSA测试密钥。

范围为workflow可选input/job env、driver失败分支、加密helper、既有config Node测试。没有产品功能、controller协议、startup重试、权限、timeout或upload glob改动。没有把诊断工具通过当作daemon退出原因或Windows协作修复通过。

## 输入与加密

`diagnostic_public_key`是可选string，通过job env传递，没有拼接入shell代码。helper只接受规范base64编码的DER SPKI RSA公钥，长度受限、modulus至少3072；非RSA、弱key、私钥格式、无效base64全部固定invalid_key，无明文fallback。缺key直接undefined且不进入任何文件读取，保留现有报告行为。

使用Node内置crypto fresh32字节key、12字节IV、AES-256-GCM；AAD在update前绑定固定schema_version、完整40位sourceSHA及三个允许phase之一，final后取tag；随机AES key用RSA-OAEP/SHA256包裹。报告只携带固定algorithm、公开AAD、SPKI摘要与base64 wrappedkey/IV/tag/ciphertext。key在finally清零；没有把密码/key/log放入plaintext公开字段。算法/API用法符合[Node crypto文档](https://nodejs.org/api/crypto.html#cryptopublicencryptkey-buffer)。

GCM证明密文与AAD完整性，不是发件人数字签名；接收方仍应以GitHub run/source/artifact来源确认producer，并核对所期待SHA/phase/公钥指纹后解密，不把“任何人可用公钥加密”误说成发布源认证。

## 文件与内容范围

driver仅catch调用，来源是已验证plan.phaseRoot/home/apiUrl。helper要求home严格位于phaseRoot之下，root/home真实路径相同；固定拼出 `.multica/profiles/desktop-<api-host>/daemon.err.log`，不读取报告提供的任意path。只允许既有loopback HTTP endpoint，lstat普通文件/非符号链接、realpath精确一致，open后fstat inode/dev与之前一致，减少路径替换风险。拒绝目录/链接/别处home，缺失与拒绝状态都只放入加密payload。

日志按打开fd的实际大小分配最多65536字节，仅读最后64KiB并用base64保留原始字节；不会readFile整个日志。另取已解析Playwright结果里的error.message/stack合计末尾16KiB，以及subprocess stderr末尾16KiB。完整stdout、业务JSON、附件、截图、env、配置、密钥文件均不收集。原private runner资料仍不加入upload集合。

新helper导入既有report parser只为选择error字段，不序列化report本体；所有raw/error字段只存在加密前内存payload。key/context/capture/encryption失败不回显异常message或原内容，只有固定状态。原public failure error仍为固定文案，upload glob未扩，cipher附在原phase报告。

## 已发现并修正的问题

初始UTF8尾裁剪可能切入多字节字符，toString插入U+FFFD后重编码使16KiB上限变成16386/16390字节。本审查用中文/emoji小复现确认；实现者已在byte尾起点跳过continuation bytes再解码。当前真实加密/解密测试确认两error字段均<=16384 UTF8字节且不产生裁剪replacement。文档已与DER-only实际契约一致，不宣称支持PEM。

## 独立验证

- 实际临时日志70KiB，真实RSA3072/AES-GCM roundtrip，解密得到精确最后65536字节；错误摘要与stderr限额、全业务JSON排除、公开JSON无原secret/home。
- wrong key、ciphertext/tag、source/phase AAD篡改拒绝；再次加密生成不同IV/ciphertext；no-key、invalid/weak/private key、invalid context、missing/outside/aliased home及外部endpoint边界通过。
- 审查附加无副作用控制：固定errlog叶子symlink指向其他文件被rejected且无其内容；AAD schema/IV篡改拒绝；非法phase、EC公钥拒绝。只用临时文件/内存测试key，全部清理。
- UTF8修正后独立重跑encrypted窄测试：**1 PASS /0 FAIL /0 SKIP**，其中包含全部上述组合断言以及中文/emoji解密边界。helper/driver node --check、git diff --check通过。
- 没有全仓或Windows重型测试，没有解密真实run证据。此实现能带回受限原始错误供parent私有诊断，但不保证日志一定包含完整调用栈，原失败仍未归因。

## 最终文件摘要

| 文件 | SHA-256 |
| --- | --- |
| `.github/workflows/desktop-smoke.yml` | `3726364fa61ba307c05c0f635ed880fb7195990c19c77aaa6aa799763d0f160b` |
| `apps/desktop/scripts/windows-collaboration.mjs` | `4a12b8f95405ad5319c99bebe828f661cf48ff2808c1055361a12cb1cb909a80` |
| `apps/desktop/scripts/windows-collaboration-encrypted-diagnostic.mjs` | `3875dd1a27f94c5f1b5a1d2900918ecace706918618f09671b78cc9247b29619` |
| `apps/desktop/scripts/windows-collaboration-config.node-test.mjs` | `1b983e8ecd874bc240045489b0816b380855769475bc66813ab3c2795f4d7386` |

仅新增本review报告；原始实施和测试由实现者维护。正式新run后解密只在parent私有文件域进行，公开结论应继续避免引用原私密字符串。
