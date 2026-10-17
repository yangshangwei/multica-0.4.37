# 测试结果

被测 main@5049e0f6a，328个唯一既有用例全部执行，322通过、6失败、0未执行。测试任务完成；产品验收未全绿。

- 产品体验：确认密码不匹配时错误提示使首次点击提交丢失（1项）；375px英文/中文智能体推荐挤压编辑区（2项）。
- 测试维护：收件箱3项仍查找旧显式role；仅修正定位的临时诊断副本3/3通过，原测试保持失败。
- 27项专属Electron全部通过；额外4项原生故障恢复检查通过；内网Nginx/Chromium14组通过。
- 全仓943个TS测试文件/11377项通过；lint/typecheck15任务成功，31条warning；Go70个含测试包race通过、vet通过。
- 所有任务服务、隔离数据库、临时worktree已清理；未改产品代码/正式测试，未创建提交。
- 共享目录由另一工作流从main切到feat/capability-market；保留其修改和生产构建生成的next-env.d.ts，不回切分支。被测生产构建及native输出来源仍是main@5049e0f6a。

完整报告：/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/main-full-e2e-20261016/REPORT.md
逐项结果：/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/main-full-e2e-20261016/all-328-results.csv
独立审阅：/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/main-full-e2e-20261016/recent-features/final-count-review.md
清理证明：/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/main-full-e2e-20261016/cleanup-verification.json

## Spec review

本轮没有新增产品契约。测试发现与夹具边界保存在完整报告：隐式原生button不能用显式role属性定位；Electron注入必须验证实际response与Query错误status，status0不能证明403权限清除。无需改动并行分支的全局spec。
