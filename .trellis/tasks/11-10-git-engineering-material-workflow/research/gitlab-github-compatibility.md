# GitLab / GitHub：需求分支与人工评审兼容性

查阅日期：2026-11-11；2026-11-12 复核合并接口并更新 D7。范围为“创建或更新需求文件分支 → PR/MR → 产品人工批准 → 系统受控自动合并 → Multica 展示结果”。以下来自官方文档；未访问用户实例、读取凭证或执行任何远程写操作。当前在线文档不证明某个旧自建版本已具备同一能力。

产品决策 D5–D7：审核界面已确定在 Multica，产品人员仅需 Multica 账号；系统同步可追溯审核记录，仓库原生审批仍须独立满足。D7 于 2026-11-12 调整为产品批准后由预先授权的系统自动合并；正常路径无需另一人逐份操作。若目标仓要求额外原生人工审批，该条件仍须满足；完整无人接手路径取决于部署时的仓库策略，系统不得自行修改保护规则。

## 1. GitLab 的 Git 写权限不等于 API 写权限

**事实：** 对 PAT、project token、group token，`read_api` 是 API 只读；`write_repository` 是所属范围内 Git-over-HTTP 的 pull/push；`api` 是所属范围内 API 读写。GitLab 将角色和 scope 分开配置，scope 不替代项目角色、分支保护等授权。

**建议：** 创建/修改 MR 的 API 路径不能只配置 `read_api` 或 `write_repository`。若用 Git push 写文件并调用 API 管理 MR，需要分别满足 Git 和 API 权限；若全程通过仓库提交 API 写文件，不应机械地再索取 Git 传输权限。PAT 的 `api` 包含 Git-over-HTTP 能力，但当前统一 scope 文档没有把该条款同样赋予 project/group token，不应推定三者完全等价。

来源：[Access token scopes](https://docs.gitlab.com/security/tokens/access_token_scopes/)、[Merge requests API](https://docs.gitlab.com/api/merge_requests/)。

## 2. 优先收窄 token 资源范围，但创建能力依部署而定

**事实：** 当前文档中，project token 限于项目，group token 面向群组范围；GitLab.com 的二者需要 Premium/Ultimate，自建 GitLab 与 Dedicated 则在各许可下可用。创建 project token 需要项目 Maintainer/Owner；创建 group token 需要群组 Owner。管理员策略、角色、到期设置仍可能约束可用性。PAT 随用户访问范围生效，不是天然项目隔离。

**建议：** 单项目试点优先核验 project token，不能假设用户必须购买 GitLab.com 付费套餐，也不能把用户 PAT 当作唯一接入方式。连接器需支持自建 base URL、API 地址、CA/网络可达性和 token 生命周期；具体认证方式待目标实例核验。

来源：[Project access tokens](https://docs.gitlab.com/user/project/settings/project_access_tokens/)、[Group access tokens](https://docs.gitlab.com/user/group/settings/group_access_tokens/)、[Personal access tokens](https://docs.gitlab.com/user/profile/personal_access_tokens/)。

## 3. GitLab 的人工评审流程和强制审批规则必须分开

**事实：** 基本 MR API 覆盖 Free/Premium/Ultimate 和 Self-Managed。Free 可记录批准，但这些批准不阻止未批准合并；强制人数、指定审批者等 required approval rules 属于 Premium/Ultimate。当前 approvals API 的部分基础端点覆盖 Free，其他规则端点注明付费层级。

另一个兼容陷阱是 `/approvals` 的 `approved`：当前 EE（含无许可证）的无规则 MR 可以在没有任何 `approved_by` 时返回 true；CE 则以至少一次批准为 true。因此这个布尔值不能单独解释为“人已经批准”。

**建议：** 只有部署能力与策略核验通过，才显示“平台强制审批门禁”。D5 已选 Multica 内审核，需保留其人工决定，并核实如何满足 GitLab 原生审核要求；业务批准不能自动替代原生门禁。Multica 自己不继续执行，不能保证拥有直接合并权限的人无法从 GitLab 绕过它。

来源：[Merge request approvals](https://docs.gitlab.com/user/project/merge_requests/approvals/)、[Merge request approvals API](https://docs.gitlab.com/api/merge_request_approvals/)。

## 4. GitLab approve 与 merge 都可校验明确的 HEAD SHA

**事实：** `POST .../approve` 的 `sha` 若不等于 MR HEAD，返回 409；`PUT .../merge` 的 `sha` 若不等于来源分支 HEAD，也返回 409。当前 merge 文档还注明：要求请求必须提供 SHA 的设置在 GitLab 19.2 引入；`merge_when_pipeline_succeeds` 自 17.11 废弃，改用 `auto_merge`。这些版本变化不能套用到所有自建实例。

**建议：** 记录被审阅的 commit SHA，合并后另记录 merge/squash 结果 SHA。D6 采用 Multica 业务审核，不要求产品审核人绑定 Git 身份调用原生 approve API，也不能由发布机器人自批替代。按修订后的 D7，有效产品批准可创建系统合并意图；实际调用前核对授权、完整变更和原生门禁，必须传入获批 SHA 并处理冲突，不能悄悄改用新 HEAD 重试。采用延迟或排队合并前需核验版本变化、批准撤回与取消语义；否则由系统等待条件满足后重新核对并调用版本受限的合并，不把入队当作完成。

来源：[Approve merge request](https://docs.gitlab.com/api/merge_request_approvals/#approve-merge-request)、[Merge a merge request](https://docs.gitlab.com/api/merge_requests/#merge-a-merge-request)。

## 5. GitHub App 的写分支、管理 PR、合并权限有不同归属

| 操作 | 当前 REST / GitHub App 权限 |
|---|---|
| 创建或更新分支引用 | `Contents: write`；涉及工作流的部分情况另需 `Workflows: write` |
| 创建或更新 PR | `Pull requests: write` |
| 提交 PR review | `Pull requests: write` |
| 合并 PR（传统 REST merge 端点） | `Contents: write` |
| 读取 PR、读取 reviews | 对应 `Pull requests: read` |

**事实：** installation token 可进一步限定安装已授权的仓库及权限，不能超出安装授权；有效期为一小时。

**建议：** 普通需求文件不应引入工作流改写权限。也不能以“只给 Contents 写权限”为理由声称技术上阻止了机器人合并；合并权限还须由目标分支规则和应用动作范围约束。凭证应归连接器管理，不放进工程文件或模型上下文。

来源：[Git references API](https://docs.github.com/en/rest/git/refs)、[Pull requests API](https://docs.github.com/en/rest/pulls/pulls)、[Reviews API](https://docs.github.com/en/rest/pulls/reviews)、[Authenticate as an installation](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/authenticating-as-a-github-app-installation)。

## 6. GitHub 的 review 版本关联与 merge 条件检查不能混为一谈

**事实：** 创建 PR 使用 `head` 分支与 `base` 分支；更新同一来源分支上的提交会改变被评审内容。review 的 `commit_id` 指向评审提交，不传时默认为最近提交；这不是 GitLab approve 那样的“必须等于当前 HEAD，否则 409”的等价契约。传统 REST merge 端点则明确支持 `sha`，HEAD 不匹配时返回 409。

**建议：** 需要保存 review 的版本关系，并在显示“当前已审”前重新核验当前 HEAD 及政策。不可把历史 review 的 APPROVED 状态直接推广到新提交。GitHub 当前文档推荐异步 merge API，且传统端点不支持 merge queue；系统合并适配需按目标仓规则选择实际支持的端点，核验预期 HEAD、队列、新提交与取消行为，不把传统端点的条件保证直接套用到其他 API。未经证明能绑定当前获批内容的延迟合并方式不得启用。

来源：[Create a review](https://docs.github.com/en/rest/pulls/reviews#create-a-review-for-a-pull-request)、[Create / merge pull request](https://docs.github.com/en/rest/pulls/pulls)。

## 7. 两平台都可人工审核，但强制门禁不是统一的免费基线

**事实：** GitHub 分支保护可以要求审批、使新变更后的旧批准失效、要求最新 push 由其他人批准；这些配置须实际启用。其免费套餐覆盖公开仓库，私有仓库的保护能力依付费计划而定。GitLab 的强制审批与版本限制见第 3、4 项。

**候选兼容契约：** 连接器返回托管平台、实例、仓库稳定 ID、PR/MR 标识与链接、来源/目标分支、当前 HEAD、审批人及评审版本、原生规则是否可读取/满足、合并事实与结果 SHA、观察时间。无法确认规则或版本时显示“未知/待核验”，不得压缩成一个两平台通用的 `approved=true`。

这是设计建议，尚未实现或在目标部署验证。人工同意必须保留实际人的身份；机器人成功调用 approve API 不能被展示为人工批准。系统执行合并后，Multica 通过响应、Webhook 与补偿读取核对实际合并事实；超时先查远端结果再决定重试，实际合并身份与产品审核人分别保存。两平台各自的事件顺序与重投保证仍须按适配器验证，本次不对 GitLab 作交付顺序保证。

来源：[GitHub protected branches](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-protected-branches/about-protected-branches)、[GitLab required approvals](https://docs.gitlab.com/user/project/merge_requests/approvals/#required-approvals)。

## 尚未核验的部署条件

用户尚未提供 GitLab 实例版本、CE/EE、许可、私有网络条件、分支保护、审批设置、token 策略，以及 GitHub 是否为 Enterprise Server/其版本。不能据本报告给出“全部自建版本通用”或“已完成接入”的结论。下一步应做只读能力探测，形成部署矩阵，再决定启用哪些门禁；此次没有请求或接触任何 secret，也没有预设用户必须升级套餐。
