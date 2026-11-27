# 物料仓初始化设计

状态：沿用用户已审阅的完整目录方案及后续明确开发请求。独立目录内核可实施；共享运行时接线依赖尚未落地，禁止将内核完成标为端到端完成。

## 1. 作用域与已有实现

本任务创建项目/仓库根级初始化与维护流程，不伪造普通需求。现有 git_material_job 绑定 material_id/revision_id；materialgit 只允许 1–3 个 requirements/<id>/{prd,design,plan}.md。保留此约束，不为基础规则扩大普通需求权限。运行时依赖设计见 research/dependency-*.md，版本目录依据见 research/prior-product-release-design.md。

## 2. 独立目录内核

新增 server/internal/materialrepo：仅做确定性目录生成、检查、候选清单与固定提交核对；不持有 Git token、不调用远端、不调度任务、不写数据库。Go embed.FS 嵌入官方 schema/template，现有 YAML/JSON 标准库解析，无新增依赖。

输入与结果合同由该包自身类型集中定义：
- Binding：稳定 system_id、root_path。仓库实例/ID、分支、运行时和权限由上层冻结。
- Snapshot：完整固定 commit 的树条目和所需文件正文/摘要；明确 complete 标记，未知树不能推定不存在。常规文件 mode=100644，目录040000，拒绝符号链接/子模块/越界和前缀碰撞。
- Catalog：20 个必须基础文件（README、AGENTS、material.yaml；6个schema；11个template），排序、每个文件摘要及总清单摘要固定，不放随机时间或本次自身commit。
- Inspection：preserved/missing/conflict/compatible，完整保留未知无关业务文件；明确哪些现存规则需维护审核，不靠文件名认定兼容。
- Candidate：仅缺失文件的 create 动作，绑定检查时commit、catalog摘要和保留文件摘要。存在冲突则没有可执行补齐候选。
- PublicationProof：独立初始化允许文件集合、source/base、exact head与全部 changed paths，不能夹带业务文件；精确创建和文件内容都核对。
- VerifyMerged：读取实际合并commit，核对所有目标文件以及检查时记录的受保护现存文件，只有完整一致才返回可供上层采用的 proof；纯函数本身没有批准 authority。

README 已存在时按原字节保留，不把任意现存 README 等价为产品生成说明。已存在 AGENTS/schema/template/material.yaml 仅已知兼容内容或后续独立审核的固定采用清单可接受；未知/定制冲突保持原样并阻塞，不自动追加或覆盖。内核首个版本先落实精确内容兼容，维护采用合同通过上层明确评审后再扩展，不能用 skipped 状态假装 ready。

## 3. 初始化文件与业务目录

首次生成入口、schema_version=1/bundle_version、目录规则、schema与模板。目录约定包含 modules、requirements、issues、releases、contracts、specs、decisions、knowledge。releases模板含版本定义/需求和修复范围、版本测试、实际验收、发布基线。不要创建示例业务版本、空需求/issue、虚构模块或 .gitkeep。现有 uat/ 及真实正文不迁移。

模板应明确 PRD、设计、任务、测试、问题、修复、产品版本与固定Git修订的区别；元数据不含状态/负责人/token。schema具体字段采用固定 schema_version，限制非法路径/未定义字段，引用自身同次提交只使用路径+摘要；发布基线引用已存在提交。

## 4. 项目/根级生命周期（依赖接线）

unconfigured -> checking -> needs_review -> publishing -> in_review -> merging -> verifying -> ready。
分支：blocked、changes_requested、unknown。操作运行 queued/running/failed 与领域阶段分别返回，未知结果保留原操作。检查发现完全兼容的仓库时，可经当前绑定的独立读回证明直接 ready，不产生假PR或假审核。

保存首次有效绑定后只入队一次 inspect。首次缺文件检查生成冻结候选、运行时发布初始化PR；有维护权限的人类批准当前 revision/head 后才调度 merge。合并后独立读回字节+完整路径并检查当前权限/绑定，才保存 ready证明。

重新检查同一intent复用记录；重试先观察远端已存在branch/PR/merge。配置的仓库身份、root、base或runtime改变使旧尝试失效，不能自动迁移旧批准；历史ready证明仍可读，但必须重新检查新的binding。运行时删除保留操作和审核历史。

完全空仓缺目标分支时进入 base_branch_missing，显示先建立默认分支的明确原因；不在后台绕过用户要求的PR审核直接建立默认分支。空仓首次提交的单独授权流程是后续兼容项，当前用户测试仓已具备main。

## 5. 独立权限与业务门槛

沿用配置维护者：当前工作空间 owner/admin 且具有项目访问权限可配置、审核初始化/维护。不复用普通需求 reviewer_ids；普通PRD审核资格不自动授予AGENTS/schema写入许可。机器凭据不能批准。

普通物料可以保存 ready=false 草稿，但 ready=true发布、后台发布领取、批准后合并和新方案启动必须检查当前已验证初始化证明。未初始化返回 material_repository_not_ready；旧项目不被自动改写或标ready。已有基线历史保留，已启动执行不被粗暴删除；未启动的后续动作按新门槛阻塞并给出修复入口。

ready证明至少绑定仓库身份/目标根、检查提交、bundle/catalog摘要和实际文件摘要。普通目录提交不得改变受保护规则；检测到未经维护接受的规则变更时应撤销当前可发布资格并重查。不能把旧ready布尔永久缓存为权限。

## 6. 运行时依赖合同

复用所选运行时任务/尝试/daemon独立只读回执，不另建队列。必须给前置 operation 增加显式目标 discriminator：repository_initialization 与 material_revision；初始化绑定 project/root/generation/candidate，不要求虚构material_id。

阶段 inspect_repository、publish_initialization、sync_initialization_decision、merge_initialization、verify_initialization 使用同一运行时合同。daemon只读观察与模型Git写入分离，结果从当前task/claim/runtime的可信回执消费。初始化allowlist最多catalog文件数，与普通PRD allowlist完全分离。

前置尚无可编译的operation/receipt代码，因此当前只能完成独立内核、模板、测试和审阅材料。持久化、队列、HTTP和UI接线在依赖代码进入本分支后实施；不提交永远排队的伪接口。

## 7. 存储/API/界面接线方案

存储建议使用 project_material_repository（binding/generation/state/ready proof）、material_repository_revision（immutable candidate）、material_repository_decision（human/head/config）、复用前置operation/attempt/receipt/request。索引单独concurrently迁移、无外键；迁移编号在前置SQL落地后分配。

API复用 /api/projects/{id}/materials 下 /initialization 读取；/initialization/check、/decisions、/retry 为version+request_id写入。响应独立 initialization capability；未支持的旧客户端不得显示ready，未知枚举拒绝版本相关操作。

共享 core/materials 初始化schema/queries，views项目物料页提供状态卡、初始化完整变更、维护人审核、受阻原因、原请求恢复；设置页保存binding后导航回初始化卡。正文草稿仍可保存，发布复选框在未ready时提示“请先完成仓库初始化”。Web/Desktop共用，无新增应用路由。

## 8. 独立维护候选合同

新增 POST /initialization/maintenance，输入仅 request_id、expected_version、expected_settings_version、服务器公告的 target_bundle_version。由当前人类 owner/admin 发起；不能上传任意文件或客户端自定allowlist。新目标须当前二进制支持，schema不兼容升级另行迁移设计，不自动尝试。

维护修订 kind=maintain，冻结当前已采用proof、旧bundle/catalog摘要、新bundle/catalog摘要、独立runtime当前读取的base commit、每个明确修改文件的before/after内容摘要以及完整保留文件集合。维护只能变更catalog内的精确路径；README若已保留为用户内容继续保留，定制规则无已采用固定摘要时阻塞。没有变化不建PR。候选必须重新经维护者审核并绑定新HEAD；旧初始化批准不能用于升级。

初始化Candidate是create-only；维护候选使用独立MaintenanceChangeSet结构和校验入口，不新增overwrite开关。runtime发布修改必须带当前base与before摘要，竞态导致旧内容不符就阻塞并重查。新proof只有实际合并+完整文件验证后替换当前proof，旧proof及决定不可变。

兼容旧bundle仍受支持且实际规则未变化时，提出维护候选不撤销原publish_allowed；未知/不兼容或实际内容漂移立刻使publish_allowed=false。此策略由服务端明确返回，UI不从maintenance状态推断。升级失败保留当前受支持的proof，不能把新bundle误标已采用。

此维护业务API/持久化随完整runtime依赖接线；当前独立内核只实现初始化和严格内容核验，不以其完成替代维护功能。

## 9. 兼容、恢复与测试

- 同仓同根重试不重复建评审，冲突不覆盖，目标分支前进后重新核对而非强推。
- 不自动采纳仓库内AGENTS为执行权限；schema/template内容是数据/受审规则。
- 纯内核测试用完整固定snapshot；集成使用独占数据库、fake runtime与隔离Git fixture，真实仓库另行核验且保留数据。
- 只有目录内核通过不代表初始化状态、runtime调用、人工批准或业务门槛完成。
