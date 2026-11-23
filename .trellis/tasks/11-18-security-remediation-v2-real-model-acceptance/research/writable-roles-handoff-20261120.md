# C1/D1可写角色边界与Git交接准备

本记录是无模型环境/工具验证，未执行真实角色工作，不生成预期补丁，不修改冻结的P1wrapper/spool/managedrequirements。新脚本 `probe_writable_roles.py` 直接运行相同固定LinuxCodex镜像原生app-server，仅initialize与command/exec，无thread/turn/model/账号。

## 候选部署矩阵

| 角色 | code目录 | tests目录 | AGENTS/配置/requirements | Git元数据 |
|---|---|---|---|---|
| 普通Contributor | RW | RW | 单独RO挂载 | 显式writable_roots当前repo/.git |
| Implementer | RW | verifier tests整目录RO嵌套挂载 | 单独RO | 显式当前repo/.git |
| Verifier | repo整体RO，代码不可写 | tests单独RW | 单独RO | .git单独RW+显式writable_roots |
| Reviewer/QA | repo整体RO | RO | RO | 不授予额外.git写根 |

最终r5四个角色全部通过：按表写入/拒绝符合预期，Contributor/Implementer/Verifier通过真实gitadd/commit，Reviewer提交失败；同容器livepeer的控制访问200，而native普通shell与显式networkAccess=true均不可达；dangerFullAccess被不可改要求拒绝。不存在同伴role挂载，未注入真实凭据。输出保存在私有 `writable-role-r5`，摘要及逐角色hash见同名JSON。

原失败保留：r2误向Reviewer授予.git写根，使bwrap尝试在RO.git下创建.git失败；r4移除Contributor/Implementer.git写根，文件编辑可用但gitindex.lock被拒。最终只给三种需要提交的角色精确.git写根，Reviewer不授予。不能用文件能写推断gitcommit能执行。

此矩阵还没有装入正在执行P1的冻结wrapper；C1/D1前需单独扩展wrapper的受管rolepolicy，新增AGENTSRO、ImplementertestsRO、VerifiercodeRO/testsRW/gitRW挂载，并冻结新入口源码和独立准入。不得在P1真实启动期间直接改livewrapper。

## 交接工具与验证

`git_role_handoff.py` 从平台确定的actor/task对应role副本导出实际HEAD的Gitbundle；只接受完整base/headhash、干净已提交目录、祖先关系、无merge提交、允许路径范围、常规blob文件，保护的复现测试必须保持sha256。manifest保留base/head/tree、bundlehash、实际Git作者与提交者原记录；作者字符串不是平台身份授权来源。

接收者须在同一base、干净目录。先把bundle导入临时stageclone，检查真实commit/tree、全量changedpaths、允许路径、protectedtesthash和无symlink/submodule，再对接收者fast-forward到原始commit。不会cherry-pick重写作者，不补写代码、不修改模型输出，不把协调者转移记录当成员交付。

六个测试通过：原commit和作者逐字保留；保护测试变动拒绝；越权路径与symlink拒绝；bundle字节篡改拒绝；manifest伪造changedpaths拒绝；接收baseline漂移拒绝。仅合成提交，未实现漏洞修复。导入工具当前要求协调者已经freeze/reviewmanifest，并在独享停止的角色目录交接；不支持活动模型并发修改、任意git配置/filter、自动merge冲突修复。真实交接前须锁住各角色run、审查所接收Git对象及repo配置，复现测试/hash与平台评论引用一并记录。


## 暂停时安全检查点

父任务在供应商连续403限制后要求停止扩展。`codex_code_container.py`仅创建了冻结P1wrapper的逐字副本、尚未实现候选role_mode策略，已核对相同后删除，避免把空实现当成可运行C1/D1入口。冻结P1wrapper/spool没有因本次C1/D1工作改变。

实际平台接线建议已与harness核对：local_directory的worktree模式产生指向共享.git目录的文件，不适合仅挂当前role目录。应使用每角色独立完整Gitclone（.git目录）作为projectlocal_directory的in_place路径，协调者通过真实人类API在串行领取前切换到预期roleclone并核对claim身份；leader仍使用平台生成的scratch目录。必须另验resourceupdate不产生非预期任务，且不能靠人为直接改claim响应伪造归属。Verifier第二次重跑应使用verifier_recheck只读模式，不授予代码/测试写入。

上述真实codewrapper和projectresource接线尚未实施、未准入、未真实调用。已完成的候选原生role挂载探针和Git交接工具仍是准备证据，不能宣称C1/D1模型验收通过。
