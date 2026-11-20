# F06 completion preview 接续 r03

此切片在精确旧 green-r03 基线上通过，未关闭 S2 2d 或 A01–A12。复用原草稿，新增测试已可供主会话与最终产品候选整合后独立复跑。

## 文件与源码身份

- 唯一产品树新增：`server/cmd/server/collaboration_stage_completion_preview_integration_test.go`，54,561 字节、811 行，SHA256 `5c83a327636c4f0a9795ceed599e1f3dfef06ab6918bf40e5dff7ac8b81490ec`。
- 隔离源：`/Users/artisan/.multica/acceptance/pay241-20261115-continuation/f06-tests/resume-r03/source`；4,955 文件中 4,954 个旧产品/helper/DDL 文件未改，前后完整清单字节及模式一致。
- 原草稿 SHA256 `0cbb17ece0ebc01bd5eb54adc236ecd02488906c7b7e755ec8493a5b463c875e`、r01/r02 及所有新失败日志保留。没有 Git 提交/分支、新依赖、迁移或独立 API/browser/model 启动。
- 伴随 JSON 保存完整 argv、raw/metadata 哈希、全部 112 项 normal/race 结果、数据库及源清单。私有证据根：`/Users/artisan/.multica/acceptance/pay241-20261115-continuation/f06-tests/resume-r03`。

## 最终验证

| 检查 | 结果 | 原始证据 |
| --- | --- | --- |
| normal | 112 PASS / 0 FAIL / 0 SKIP；10 顶层、102 子记录、95 叶记录 | `logs/normal-r03-final-v2.jsonl` |
| race | 同一 112 项全部通过；10 顶层、102 子记录、95 叶记录 | `logs/race-r03-final.jsonl` |
| vet | exit 0 | `logs/vet-r03-final.jsonl` |
| 编译 | `go test -c` exit 0，生成私有测试 binary；没有执行 TestMain | `logs/compile-r03-final.jsonl` |
| gofmt | exit 0，输出为空 | `logs/gofmt-r03-final.jsonl` |
| 源/数据库身份 | 4,955 文件零漂移；两个许可库各 835 ledger、末 864、bare hex64、其他连接 0、stage facts 0 | `logs/final-r03-v2-{before,after}-source.json` 与 `*-database.json` |

使用 `run.mjs` 与分别匹配的 `router.json`/`router_race.json`，仅运行 `go -C <隔离源>/server test ./cmd/server -run '^TestCollaborationStageCompletionPreviewRouter' -count=1 -json -timeout=180s [-race]`。实际工具链为 `go version go1.27.0 darwin/arm64`；未宣称 CI Go1.26/Linux 验证。normal/race 是同一库存的独立执行，不能相加为 224 个用例；父记录也不是新增业务用例。

## 原失败与最小修正

原 r01 为 70 PASS/8 FAIL/0 SKIP（4 个失败顶层；5 个失败终端叶记录）：外部发起人及无 source 成员管理员已正确被 403 拒绝，但旧断言要求 404；consumer 撤权实际 404，旧断言要求 403；第二次真实 packaging 后调用只接受单 execution directory 的 archive helper 失败。原 r02 的作者已分别更正状态包络和指定 archive 路径，未修改产品。

原 r02 为 86 PASS/2 FAIL/0 SKIP；唯一失败叶例 `wrong_current_organization_is_not_R38` 在 `organization internal:false` INSERT 上触发 23514，还未到 HTTP。465 规定 CHECK(internal)，467 对 internal 全列唯一，不能构造第二个合法组织。本轮先以原 SHA 精确复现该红例（0 PASS/2 FAIL，父+叶），再改名为 `workspace_organization_link_mismatch`：只改变无 FK 的 workspace 关联为新 canonical UUID，GET `/api/admin/me` 仍证明原真实 admin/current 唯一组织；两个 R38 路由保持 404、最小包络及零写。它只证明缺失/错配关联 fail-closed，不证明第二真实 tenant 的隔离。

r03 新边界首跑的 76 PASS/3 FAIL 保留；两个红叶是新 cursor 夹具在第二个 archive 之后误用旧单目录 helper。修正仅为第二次 packaging 前验证原 tar、之后按原 execution ID 比对其字节。后续两个 cursor order 均绿。首次全量 normal 的 112 通过也保留；最终 v2 只冻结首个读事务 PID，使后续拒绝读取不改写证据归因，再完整 normal/race 复跑。

## W17 与 Q05–Q07 的范围

| 项目 | 已验证的旧读取增量 | 边界 |
| --- | --- | --- |
| W17 | 四个真实 root/R38 candidates/preview 路由、done 普通 child、数字 root/child revision、stage 不完整三 M1 模式返回 200 非授权 | 最终 2d 产品源重跑待主会话 |
| Q05 读取子集 | 当前 issuer/consumer 分离、外部 initiator 私有证据拒绝、issuer 到期上限、consumer epochs、password/role/当前唯一 org 中途失效 | 原 M1 stage-off 正路径是兼容控制，未证明 stage-enabled all-writer 完成；无第二真实 tenant |
| Q06 | 11 个明确 SHA 前后相等见证；authorization/evaluation/submission/status/attachment-link/queue 与新增 stage facts 零读取写入；self draft 不预绑定 | 独立授权/完成控制和测试 setup 本来会写入，不是 preview 效果 |
| Q07 | strict JSON/64KiB、分页/cursor、empty 隐私、旧路由/对象区分；list/empty/cursor/preview 两个撤权顺序 | 不把继承的 200-row hidden scan 算本切片额外验证 |

八个竞争用例均使用实际 router auth 和两个不同 PostgreSQL backend。read-first 在真正 Commit 前让 canonical DeleteMember 取得 55P03，读 commit 后删除成功，下一原请求只得 404；revocation-first 在 Auth 后/读事务首数据查询前实际删除，当前与缓存请求都为 404，无私有页或 binding。没有 SQL 重写、测试专用公网路由或伪造主体上下文。

只读断言针对每次候选/preview；整套测试另有明确 M1 授权与完成控制。normal 库从既有 12/12 授权/terminal 到 24/24，来自两次全量各六条独立 M1 控制；race 从 0/0 到 6/6。全部历史保留，stage 临时事实清空，不能把这些库级计数误写成读操作生成授权。

## 后续

无已证实的本切片产品 finding。P2 保持 false，所有账户/PR 数据明确为 synthetic，实际 tar 与真实本地 auth/路由不等于远端 CI、模型、支付或部署。主会话需将本测试与最终服务端 candidate 整合交独立 checker，按同一库存重跑；完整 W/Q、stage-enabled 完成、A09/A11/A12 与四个 Web/Desktop 场景继续开放。父 24 例状态不变。
