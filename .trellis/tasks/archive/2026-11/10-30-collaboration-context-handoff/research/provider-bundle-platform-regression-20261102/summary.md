# Bundle 快照夹具回归修复

根因已由单变量诊断确认：原测试在任务 TMPDIR 下6pass/2fail，只有 TMPDIR 改为标准 Unix 临时根后8pass。`openBundleSource` 支持 Linux/Darwin；它正确拒绝 `/Volumes/artisan` 的非root组可写祖先。未把该结果解释为 Darwin 不支持，也未修改共享卷权限。

代码 `91113cbeb` 仅改一个测试文件：Linux/Darwin 的源夹具自行拥有并规范化安全临时目录，原安全矩阵保留，新增组可写父目录拒绝；其他平台明确断言拒绝且无输出文件，不加skip。生产文件零改动。

同一旧任务 TMPDIR 下全部 ProviderBundle 用例 normal29/29、race29/29通过，0skip。私有 Go overlay 只禁用祖先权限检查时，新增父目录回归按预期失败，证明没有掩盖权限边界。Linuxarm64和Windowsamd64编译通过。统筹以非root用户、共享可写TMPDIR、无网络的缓存Linuxarm64容器实际运行29项通过；仅证明bundle文件矩阵，不是完整Linux containment/安装验收。Windows无原生证据。统筹完整collaborationrun race仍在执行。

所有命令、环境、原始退出码、具名计数、源文件与日志哈希见配套JSON。原全量失败仍为失败；独立复核、本地合流、重新绑定及完整最终检查仍由统筹推进。未操作数据库、服务、真实Provider或用户安装的AgentCLI。
