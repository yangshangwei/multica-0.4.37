# 构建临时目录隔离修正

边界：只调整任务私有完整检查 launcher 的 `MULTICA_DEV_TMPDIR`，产品与测试源码不变。第二轮231a08原始5失败、22个包库及未跟踪`.cursor/mcp.json`完整保留；不恢复或清理该工作树。

同一源码的5个失败用例仅改变TMPDIR到Git树外，race实际5/5通过，见execenv-temp-diagnosis.json。完整execenv race正在同条件复验。

下一轮前由统筹在`/private/tmp`新建唯一0700任务目录，写入所有权记录，核对不存在Git祖先；加载实际运行环境后再核对，保留目录与失败日志。只由launcher设置显式DEV_TMPDIR供check脚本使用，不变更操作员默认TMPDIR或共享卷权限，也不设置Git开关把错误隐藏。default suite/fixturehelper/assertions保持原样。修正结果经独立只读核对后再开始新候选完整check。
