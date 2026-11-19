# F06 completion preview 接续 r03 独立检查

**结论：PASS_SCOPED_GREEN_R03_F06。** 未发现本增量的待修产品或测试问题。接受范围仅为旧 green-r03 产品/helper 加唯一新增 F06 测试的 4,955 文件冻结图；不代表整个 ROOT、新 r05 产品、完整 2d、S2 或 A01–A12 通过。

## Findings (fixed)

无 F06 产品、helper 或测试修改。检查仅新增本 MD/JSON 报告和仓库外证据。私有清理统计 r01 原数保留，r02 将不可变 M1 evaluation/policy 历史与瞬时夹具分开，未删除审计数据。

## Findings (not fixed)

本增量无已证实的未修 finding，blocking findings 为 0。完整 stage-enabled 完成、最终产品整合、P2、四个 Web/Desktop 场景及完整 S2 验收仍由主会话继续，不能用本检查替代。

## Verification

| 检查 | 实际结果 |
| --- | --- |
| normal | 112 PASS / 0 FAIL / 0 SKIP；10 顶层、102 子记录、95 叶记录 |
| race | 同一 112 项全部 PASS；缺名、额外名、重复终结记录均为 0 |
| Lint | 新测试 gofmt -l 输出为空；go vet ./cmd/server exit 0 |
| TypeCheck | Go go test -c ./cmd/server exit 0；未执行 TestMain。此 Go 测试增量不适用 TypeScript 检查 |
| 源身份 | before / after-normal / after-race / final 全 4,955 文件 bytes、SHA256、mode 相同；4,954 个原文件不变 |
| 配置/数据库 | 两份指定 review_router 配置 SHA 相同；835 ledger、末 864、bare hex64；最终其他连接 0、stage 临时事实 0 |

normal/race 独立执行同一库存，不能相加为 224 个用例；父记录也不是新增业务用例。再次独立解析原始 Go JSON，112 个 run 与 112 个终结记录一一对应，源码提取的 10 个顶层名称完全匹配。每次执行的 11 组 READONLY/AUTHORITY_ORDER 前后 SHA 均相同。

实际工具链为 **go1.27.0 darwin/arm64**，PostgreSQL 为 17.11；没有 Go1.26/Linux CI 结论。编译 binary SHA256 为 66badc1501111f381852125941f866c42553982760b3839a02d546a32fa3471c，与实施冻结 binary 相同。

## 精确源与执行证据

- 冻结源：/Users/artisan/.multica/acceptance/pay241-20261115-continuation/f06-tests/resume-r03/source。
- 唯一新增测试：server/cmd/server/collaboration_stage_completion_preview_integration_test.go；54,561 bytes、811 lines；SHA256 5c83a327636c4f0a9795ceed599e1f3dfef06ab6918bf40e5dff7ac8b81490ec。
- 原 green-r03 manifest SHA256：3e4bc0270a1e675ebb0a9cae4e479853393c6d796266d049f4e07f1728a3e7b3。
- 原实施 MD SHA256：e75e3c0fa3cc5bf2879db7af00fa7a6dcd1c51d2da1d5373c8b963af1d645b8e；JSON SHA256：938bdbdff2a6716081e0c2c5f6f3c3de8f52770deade0277432cc8164d5d2ca7。
- 原始 34 个证据引用逐一核对 SHA/bytes，全部匹配；原草稿和 r01/r02/r03 失败不变。
- 私有独立证据：/Users/artisan/.multica/acceptance/pay241-20261115-continuation/resume-r05/check-f06。完整 argv、时间、退出码、全部命名结果、四份源清单、八份数据库审计及原始证据核对在伴随 JSON 中。

| 原始独立日志 | SHA256 |
| --- | --- |
| logs/normal-r01.jsonl | dbe2cc296784eae237910e78f98982333e6e03437e7e840bfd4721ce1e249796 |
| logs/race-r01.jsonl | 5de3e3b732fc042edfae9e3b9c1031a1dfc9be62c3d4e9fa20c6458beb658d53 |
| final-source.json | 8174a8bc535f198ef47e551da782fff11f8de149aec5b6135c0249844eb759cc |

测试仅通过既有 run.mjs，分别使用 resume-r05/configs/review_router.json 和 review_router_race.json；实际 Go argv 为 -C <上述冻结源>/server test ./cmd/server -run '^TestCollaborationStageCompletionPreviewRouter' -count=1 -json -timeout=180s，race 另加 -race。两个执行串行，未创建、迁移、切换或删除数据库。HOME/TMP 四路径均在 Git 外，git-root probe exit 128。静态检查没有数据库环境。

## W17 与 Q05–Q07 审阅依据

| 范围 | 具体代码与断言 |
| --- | --- |
| W17 / Q06 | 新测试 95–178：四个实际 root/R38 candidates/preview 路由；普通 child 实际经 M1 完成后仍可作为 root 来源；root/child 数字 revision、直接 parent、同 workspace/project 和实际 tar 字节。138 行逐项比较 target/project/evidence/服务器 consumer。self draft 读取前后不绑定 |
| Q05 读取子集 | 180–219 的 issuer 与 initiator epochs 7/9、有限 issuer credential horizon；249–281 的真实 router 签名 JWT/task/AgentActor/伪造头拒绝；538–597 的 R38；771–811 在实际 Auth 后改变 password session、platform role 或唯一 organization，旧权限不能恢复私有证据 |
| Q06 零读写 | 55–91 的 root/source 快照复用既有 completionHintSnapshot，覆盖 evaluation、授权/terminal、status/submission、attachment/reference、queue、event/operation、stage 等。142/173/240/420/762 等在成功和拒绝后比较；fixture、明确撤权和 M1 控制写单独归因 |
| Q07 strict/pagination | 283–378：严格 object、duplicate/unknown/case alias/trailing JSON、有效 64 KiB 与超限、合法跨路由 variant 拒绝、query/limit 校验；427–536：两个真实 packaging、1/50 分页、root/source/viewer/revision cursor 绑定、旧路由与结构化对象拒绝、变更及缓存撤权 |
| Q07 锁与空页 | 599–769：list/empty/cursor/preview × 两个提交顺序共八叶例。wrapper 转发全部 SQL，Begin/Commit 必须实际命中一次且两 backend 不同；read-first 删除取得 55P03，读 commit 后删除成功；revocation-first 在 Auth 后/首数据查询前实际删除，当前与缓存请求只得 no-store 最小 404。717 行保存最初 reader PID，后续请求不改其归因 |

实际产品路径来自同一冻结图：handler/collaboration_completion_preview.go:29 的五秒借用事务与最终 credential/commit，service/collaboration_completion_preview.go:61/123/176 的 root/source/M1 observer；router.go:1703/1824 的真实路由。新测试复用 testutil Insert/InsertNoID/QueryRow/Call/Want/JSON；原始 malformed 字符串直接转发，Scan 失败会终止测试。没有伪造主体上下文、SQL 重写、产品测试路由或新依赖。

## 原红例与最小夹具修正

原 36,878-byte、SHA 0cbb17ece0ebc01bd5eb54adc236ecd02488906c7b7e755ec8493a5b463c875e 草稿的 wrong_current_organization_is_not_R38 明确 INSERT internal:false；保留红日志记录 organization_internal_check / SQLSTATE 23514，未到 HTTP。465 CHECK(internal) 和 467 UNIQUE organization(internal) 不允许第二合法组织。

新版 569–596 行只改 organization_workspace 关联到新 canonical UUID；GET /api/admin/me 仍证明原真实 admin 与当前合法单一 org，两个 R38 路由均断言 404/no-store/完整最小 body/零业务写。它证明关联错配 fail-closed，**不证明第二真实 tenant**。

r03 两个 cursor 红叶来自第二次 packaging 后误用旧单目录 archive helper，原失败保留。冻结新版先用原 helper 验证完整 tar/entry/bytes/SHA/EOF，再按原 execution ID 读取其字节；原 helper 和 4,954 文件均未改。

## 数据保留与清理

每个独立库的 M1 authorization/terminal 从审计的 0/0 变为 6/6，来自三个显式 child+root 完成控制；不是 preview 生成授权。两个库分别保留 33 evaluation、72 M1 policy revision，与 33 fixture 的明确 evaluation、每 fixture 两次 policy 修订及三组额外 observe/off 调用一致。逐请求快照仍证明这些行未由读取增加或修改。

最终 24 类瞬时主体/成员/角色/issue/session/task/approval/artifact/admin 等表均为 0，stage 临时事实为 0，其他数据库连接为 0；ledger、配置及 gate 前后不变。私有 final-transient-cleanup.json 原 counts 保留；r02 纠正将两类不可变 M1 历史纳入统一 empty 谓词的分类，没有清除历史。

## 范围与后续

此增量没有 stage-enabled all-writer 正例；stage 不完整三种 M1 模式的非授权读取与原 stage-off M1 实际授权/完成是不同证据。继承的 200-row hidden scan 未在本切片重测；没有新 P2、独立验收 API/browser/Provider、真实模型/支付/远端 CI/部署调用。原全局 F2 迁移注册缺口未在此重跑或关闭。

本测试只新增回归，未建立新生产合同，无需修改全局 spec/template。主会话应把同 SHA 测试整合到最终 server candidate，再绑定最终源码与专用新验证环境；完整 W/Q、A09/A11/A12 和四场景继续。父 24 例仍为 4 PARTIAL、20 NOT_RUN。ROOT freeze 下本检查未运行或编辑 ROOT 产品/全局 docs/spec/tests。
