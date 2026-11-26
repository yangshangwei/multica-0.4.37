# V0.8.8 交付文档与材料预审

日期：2026-11-26。范围为final-docs三份准备稿、dist/release/v0.8.8当前SQL/配置清单、公开Windows摘要、下载服务同步tar及Windows CLI zip。**本轮为partial-preparation预审，不是正式Release放行；正式Linux包/Linux CLI/唯一CI feed/最终总校验及FINAL-VERIFICATION尚待完成，不将明确待填项当缺陷。**

## 发现与处置

1. 已解决：CONFIG-CHANGES开头目标SHA曾错误保留5a48预览SHA，与最终CONFIG-INVENTORY及统一d2c不一致。已报告主会话，privateaudit和dist两份均已修正d2c9240eaf9f768446faba93cac625bd63e9cf53；原预览验证SHA保持不动。
2. 建议精确化RELEASE-NOTES自动化修复：限定run_only在解析智能体后、事务前读取runtime时被删除，记录target_unavailable/skipped且不入队；其他数据库错误保留。原“按实际可执行条件处理/避免错误调度”容易泛化到运行中删除或offline状态，已交parent修正，未代改。
3. 建议README第4节明确“在发布机完整仓库根目录执行”。前文在/opt/multica部署目录，后文脚本依赖完整仓库apps/Node工具；材料包本身不具备collect依赖。只需明确cwd，不改命令。

除此之外未发现当前准备稿的实质事实错误。状态顶部与RELEASE-STATUS清楚禁止未完成目录部署；Windows已有PASS与Linux旧预览/正式待填分开，目标Release URL标为未确认发布。版本/标签/源SHA/目标仓库/架构、旧Windows RC基线、未签名/差分未证/knowledgeOFF/客户内网范围均准确。

## SQL与配置证据

独立检查SQL-INVENTORY：1900源码项=877up+877down+146query，完整版本唯一、up/down配对；1900行SQL-SHA256SUMS逐行与manifest一致。逐一读取d2c Git对象核对全部1900文件bytes/SHA，不只是计数或最大889。基线1270项，新增630=566迁移文件+64queries，改写18queries，删除0。完整up词典序与清单相同。

SQL-AUDIT准确说明条件迁移记账、371跳过后trgm保留、876 runner提交文件不等于DO内部所有分支；预览/源码链路与最终构件待验区分。历史四组/209 concurrent索引结论与既有研究一致，本轮未重复数据库或运行测试。

CONFIG-INVENTORY的24个源文件hash全部独立匹配d2c Git对象，变化/传递表与既有比较相符。配置文档要求真实.env/override顺序、JWT/加密密钥/DB/部署ID/URL/卷/桌面身份保留，不把模板中的新增key等同容器已传入；最终实际容器复验标pending。

## 归档及现有交付字节

下载服务tar共10个普通文件：8个repo文件+同步README+manifest；所有路径均在唯一预期根，无绝对/穿越/符号链接/设备/AppleDouble、真实.env/updates.env/download-config.json或管理员凭据。8个源码文件逐字等于d2c Git对象，manifest完整source一致。README准确限定已有站点页面/手册同步，不含Nginx镜像或Node依赖，不宣称新站点完整离线包，明确保留管理员配置/业务地址/历史安装器。

Windows CLI zip仅multica.exe/LICENSE/NOTICE/README.txt，无异常路径；二进制SHA43578ef72a867b3ff86d649a6f3d9b43cf6a86fa5d532ab7523ea7f916b8dffe等于已原生验收安装器内CLI。README明确同源d2c、server先升级、受控替换/原身份保留、upstream自更新限制及未签名。三个现有.sha256 sidecar均对实际对应zip/tar/exe重算通过。

总README中的最终SHA256SUMS.txt、正式server/CLI文件尚不存在属于显式待填交付门槛，不计为当前错误；最后必须生成完整总清单并复验。两份通用升级手册保持冻结源原字节，不能为了最终报告修改包内源手册而破坏同源。

## Windows最终汇总复核

再次读取desktop_delivery最新private windows-verification.json：所有evidence路径文件SHA、4个tool脚本hash、prepared-windows六文件长度/SHA重算一致。公开dist/windows-verification.json指向其精确summary SHA，公开4个构件hash、native/payload/download范围与此前独立完整审查一致，未包含本机绝对路径、测试账号/身份或加密私有取证内容。public status=passed仅为Windows模块，不覆盖目录总体准备状态。

原生scope基线0.5.2-rc.11.1、5lifecycle/9business/2完整OFF协作、full-download fallback、unsigned等事实均保留；原13次失败run仍列失败，不回写。ASAR CRLF仅2locale+LICENSE/NOTICE四份文本的源比较，原raw hashes和内部完整性未规范化；HTTP staging5文件仍与源码逐字一致，初始失败保留。

## 验证方式与待办

本轮只读文档、源码Git对象、归档成员、既有审计及文件摘要。未重复runtime/DB/Windows/浏览器验证，未写产品或修改打包文件；仅新增本报告。普通exec读取遇工具transport误判后保留所有状态，改用只读Node filesystem/child-process通路，无运行资源清理。

最终放行前仍需当前main/live漏洞与Release CI结果、唯一CI三feed文件、正式Linux完整包与Linux CLI、正式两模式/PG/schema/故障/恢复/登录验收、全附件线上本地摘要、最终SHA256SUMS与FINAL-VERIFICATION。当前已准备材料无需因这些明确待办被伪标失败，也不能提前标正式通过。

## 准备稿当前摘要

| 文件 | SHA-256 |
| --- | --- |
| `README-v0.8.8.zh-CN.md` | `da9a90b78ff17fbc8edd4c3ebbac66c5fa3deb1a5204146f4bd60a87e1845da4` |
| `RELEASE-NOTES-v0.8.8.zh-CN.md` | `c8af0051ef795b8b5b22c2888093836b2b7447272e3a95f6511aa47f74a2efb9` |
| `BUILD-VERIFICATION-v0.8.8.zh-CN.md` | `8b01699d5f81a75b4b4cd3a1f149ee090bd0c0b3ca0873440ab3a80ec0afc337` |


## 最后准确性修订复核

主会话已落实两条建议：版本说明明确run_only解析智能体后、事务前runtime删除→target_unavailable/skipped/no enqueue，普通DB读取错误保留；README第4节明确在发布机完整仓库根目录执行。独立读回确认。**当前交付准备稿与已准备材料预审PASS，无剩余已知文案阻断；最终发布门槛仍待后续正式构件/CI/总清单验收。**

修订后的文档SHA：

- README-v0.8.8.zh-CN.md: da9a90b78ff17fbc8edd4c3ebbac66c5fa3deb1a5204146f4bd60a87e1845da4

- RELEASE-NOTES-v0.8.8.zh-CN.md: c8af0051ef795b8b5b22c2888093836b2b7447272e3a95f6511aa47f74a2efb9
