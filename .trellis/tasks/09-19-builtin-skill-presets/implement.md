# 执行计划：内网可投放的 Skill 模板库

## 前置

- 分支：从 main 起一个 `feat/builtin-skill-presets`（勿直接改 main）。
- 无数据库迁移。改动集中在 `server/internal/service` + `server/internal/handler` + compose/docs + 少量前端测试。

## 有序清单

### 1. 后端：配置注入
- [ ] 1.1 `TaskService` 加 `SkillTemplateDir string` 字段。
- [ ] 1.2 在 server 装配处（读 `MULTICA_PLUGIN_DIR` 的同一初始化代码附近）从 `os.Getenv("MULTICA_SKILL_TEMPLATE_DIR")` 读取并 `TrimSpace` 注入。
- 验证：`空 → SkillTemplates() 只返回 embed`（单测）。

### 2. 后端：目录扫描 + 合并
- [ ] 2.1 新增 `server/internal/service/skill_template_dir.go`：`scanSkillTemplateDir(dir string, embedNames map[string]struct{}) []RoleSkillTemplate`。
  - 一级子目录；名称过 skill 校验 + 保留字；拒绝 `/ \ .`-前缀（对齐 `plugin_package.go:86`）。
  - 读 `<name>/SKILL.md`，`ParseSkillFrontmatter` 取 description；目录名为权威 name。
  - 同名 embed → 跳过 + `slog.Warn`（D4）。
  - 递归收集附属文件为 `AgentSkillFileData`，套用大小/数量上限（复用 import 常量或等价）。
  - 稳定按 name 排序。
- [ ] 2.2 `TaskService.SkillTemplates()` = `RoleSkillTemplates()` + `scanSkillTemplateDir(...)`，embed 在前。
- 验证：畸形条目跳过、其余正常（单测覆盖每类畸形）。

### 3. 后端：handler 切换来源
- [ ] 3.1 `ListSkillTemplates` 从 `service.RoleSkillTemplates()` 改调 `h.TaskService.SkillTemplates()`。
  - 挂载条目 `Version` 取约定值（0）。响应结构不变。
- 验证：`skill_template_test.go` 扩展——挂载临时目录，断言列表含挂载条目、含 embed、冲突时 embed 赢。

### 4. 前端：测试锁定（无生产代码改动）
- [ ] 4.1 面板测试：给一个不在 `BUILTIN_ROLE_SKILL_NAMES` 的模板，断言用 raw description 渲染 + 可搜索。
- [ ] 4.2 zod malformed-response 测试：畸形 templates 响应不让添加流程崩（AC7）。

### 5. 部署 + 文档
- [ ] 5.1 `docker-compose.selfhost.yml` 加只读挂载：`${SKILL_TEMPLATE_DIRECTORY:-./skill-templates}:/app/data/skill-templates:ro`，并让 backend 环境把 `MULTICA_SKILL_TEMPLATE_DIR=/app/data/skill-templates`（对齐 `CHANGELOG_DIRECTORY:64` 写法）。
- [ ] 5.2 SELF_HOSTING / offline-bundle 说明补「投放 skill 模板」小节：目录格式 `<name>/SKILL.md`、只读挂载、丢文件即生效、冲突规则。

## 验证命令

```bash
make test                                   # Go：service + handler
pnpm --filter @multica/views test           # 面板 + zod 测试
pnpm typecheck
pnpm lint
```

聚焦跑：
```bash
(cd server && go test ./internal/service/ ./internal/handler/ -run 'SkillTemplate' -count=1)
```

## 风险文件 / 回滚点

- `skill_template_dir.go`（新文件）——路径安全是重点，评审务必看逃逸/软链/大文件三类。
- `ListSkillTemplates`（改来源）——回滚只需改回 `service.RoleSkillTemplates()`。
- compose 挂载行——回滚删行即可，无数据残留。

## start 前检查

- [ ] embed-only 路径与现状字节级一致（未配置环境变量）。
- [ ] 三类畸形条目单测就位。
- [ ] 冲突（embed 赢）单测就位。
- [ ] 前端 malformed-response 测试就位。
