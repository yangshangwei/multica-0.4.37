# 交付准备执行边界

沿用父任务发布门与2026-11-01续作授权，不创建新任务。分支`codex/collaboration-delivery-20261101`从主集成`822c15785`创建，工作树`/Volumes/artisan/code/2026/mc-collaboration-delivery-20261101`。准备可以提前，最终候选绑定与安装结果必须等runtime、知识和RK1合流。

## 最小行为缺口与所有权

- Windows原生检查器只有旧Host必需清单，不能显式拒绝遗漏或跳过新checkpoint/seal/重启恢复用例。修改`scripts/check-collaboration-native-tests.mjs`及其测试，增加发布门G1列出的四个实际用例；现有三系统作业继续复用。
- 安装版协作E2E只支持macOS native或Linux Docker；Windows已安装Main无法走同一协作旅程。优先扩展`e2e/fixtures/collaboration-installed-main.ts/.mjs`和`e2e/collaboration-host-installed.spec.ts`的平台路径、元数据、home隔离与进程结束约束，复用真实Main/preload/renderer和Host fixture。不得另写一套伪IPC成功脚本。先验证底层Host fixture是否支持Windows，再接入显式opt-in的Windows作业；不支持的原语必须具名拒绝，不能假报pass。
- `.github/workflows/desktop-smoke.yml`已有安装/业务/更新流程，但未包含协作。接入候选版的fixture旅程并保存源码SHA、CLI/ASAR/EXE哈希、原生runtime、具名用例和报告；上传只含脱敏报告，私有状态不上传。运行外部CI需另核授权。
- `docs/offline-upgrade.zh-CN.md`仍宣称0.6.6与570上限且缺知识开关。当前已修订为候选说明与已合入760上限，保留认证方式，补默认false和升级顺序；待审计索引合流后按实际迁移上限再更新。打包时两份指南必须逐字节一致。

不新增依赖，不改认证/授权或生产默认值，不发布tag、推送、触发外部CI或部署。Windows/arm64、真实账号、签名及离线信任各自保持未验收。

## 验证与独立复核

1. 检查器先加入遗漏/skip的失败回归，再实现并跑Node测试。
2. Windows平台路径和安装元数据用本地fake process/文件验证；真实平台结论只能由原生runner给出。Fake executable不得转调用户安装Agent CLI。
3. 当前源码Windows交叉build/vet/`go test -c`与macOS同过滤器计时，使用agent CLI guard；不以`go test -run '^$'`编译检查。
4. 指南配置值逐项对照`.env.example`、Compose和Go解析，文档diff检查；包完成后比较两份指南和源文件哈希。
5. 分支独立check通过后，按runtime→知识→RK1→交付顺序由统筹本地合入，最终同候选make check/门控Web/Electron/RK1 fake；此处准备不关闭平台发布门。

## 外部输入待核对

现有授权覆盖本地准备、提交与独立复核通过后的本地合入。Windows原生runner或外部CI、公开推送、K1–K3/E0b、安装版真实账号旅程、AC43两环境不同Provider账号和额度须单独落实。待本地方案可审阅后集中询问，不以未授权的调用或发布验证准备代码。
