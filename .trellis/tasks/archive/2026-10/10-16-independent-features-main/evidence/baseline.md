# main 基线

main7906c6e93，Node22，锁定依赖离线安装成功。三个目标测试文件实测：23失败、6通过。失败包含ApiError mock和listitem/交互时序问题，与批准的14642c42f修复范围一致。命令：pnpm -C packages/views exec vitest run inbox/components/inbox-context-menu.test.tsx issues/components/issues-header.search-filter.test.tsx issues/components/issues-page.test.tsx --maxWorkers=2。

原始日志：/tmp/multica-independent-main-baseline.log。只读计划审阅：批准，无阻塞。
