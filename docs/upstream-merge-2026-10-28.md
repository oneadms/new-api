# 官方上游合并及 PostgreSQL 升级说明

本次工作于 2026-10-27 开始，2026-10-28 完成。目标是合并 QuantumNous/new-api 官方 `main`，保留当前分支的业务功能与已有用户数据。未连接、修改或迁移线上数据库。

## 代码范围与版本

- 合并前本地提交：`dd9087206a868b89d05e35fba8d754efe27e75be`。
- 官方上游：`https://github.com/QuantumNous/new-api.git`，本次拉取的 `main` 为 `34003e9c2eed2c20e617f583fdfe54bf1bf42237`。
- 上游相对共同祖先增加 481 个提交，本地增加 67 个提交；采用保留双方历史的 merge。
- 本地备份分支：`codex/backup-before-upstream-20261027`。生产回滚还需要旧镜像、原配置和数据库备份，备份分支不能替代数据备份。
- 编译工具链：根模块 Go 1.26.10；前端本次使用 Bun 1.4.4。独立模块以各自 `go.mod` 为准。

保留了现有充值奖励、抽奖、分组卡、订阅分组限制、实际计费倍率、对战、质量检测及公开状态历史、SSE 追踪、Codex2API 策略签名和 K3s 渲染器部署。订阅余额购买继续维持当前分支的关闭状态。

## 数据结构与兼容措施

| 变化 | 存量数据处理 |
| --- | --- |
| 用户累计额度字段 | `users.quota`、`used_quota`、`aff_quota`、`aff_history` 保持 bigint；旧 integer 字段在主节点启动时先扩容，再执行上游检查。不清零余额，不收窄为 int32。 |
| GORM 与驱动 | GORM 1.25.12、PostgreSQL 驱动 1.5.9、MySQL 驱动 1.5.7、glebarez/sqlite 1.11.0，作为同一组合验证。保留上游数据库迁移适配器。 |
| PostgreSQL 唯一约束 | 保留上游 token key、预填充分组等约束迁移；迁移后的重复数据插入仍被拒绝，不能用删索引绕过启动错误。 |
| options 主键 | 对没有唯一键的旧表检查重复项。同名且值相同的记录按上游方案保留旧表备份后归并；同名但值不同则停止升级，不任意选择计费或认证配置。错误会向启动流程返回。 |
| 会话与认证 | 保留会话哈希字段兼容处理、认证版本初始化；策略封禁接入上游认证版本与缓存失效机制，保留用户账务数据。新增个人访问令牌、登录加密密钥等上游表。 |
| API token | 新增自动分组配置和额度查询开关；旧 API key 保留。旧令牌的额度查询开关默认关闭，需按需显式开启。 |
| 日志和扩展 | 引入上游审计日志、任务插件等表；保留本地奖励、分组卡、检测、追踪和策略记录表。消费日志的旧内容仍可读取，管理字段不向普通用户暴露。 |
| 计费配置 | 保留 `ActualModelRatio`，包括显式 0；预扣、文本/音频结算及价格编辑均使用该值。已有实际倍率不会被新增内置计费表达式静默覆盖；显式配置表达式模式仍按该模式执行。 |

迁移由 `model/main.go` 的主节点启动流程执行。**不要设置 `SKIP_64BIT_QUOTA_SCHEMA_CHECK=true` 来绕过结构问题。** 从节点应在主节点迁移成功后启动。

旧面板管理访问令牌的兼容期是首次升级启动后 30 天，截止时间持久化在 `options.LegacyAccessTokenRetireAt`，重启不会重置。使用旧令牌的管理脚本应在期限内切换至具有适当权限的 `nap_` 个人访问令牌。这与调用模型的 `sk-` API key 不同，后者不会因此自动失效。

## 上线前：使用真实备份在隔离环境演练

本次自动化演练使用合并前代码生成 500 个模拟用户及关联记录。线上实际 PostgreSQL 版本、历史脏数据、扩展和自定义索引尚未读取；正式发布前，应在生产备份副本上复核。

以下 `PGSERVICE` 是 PostgreSQL 客户端连接配置名示例；需要先配置到正确的服务器和数据库。密码放在受限权限的连接配置中。备份目录应有足够空间和访问限制。

1. 记录当前镜像 digest、应用提交、PostgreSQL 版本、主库/独立日志库连接配置、部署副本数及密钥配置。保留原 `SESSION_SECRET`、`CRYPTO_SECRET` 等密钥。
2. 先制作演练备份；正式切换时停止流量、后台任务及所有旧实例的写入，再制作一份最终一致备份。若日志库独立配置，也单独备份。

```sh
umask 077
export UPGRADE_BACKUP_DIR='/secure-backups/new-api-2026-10-28'
mkdir -p "$UPGRADE_BACKUP_DIR"
PGSERVICE=newapi-production psql -X -v ON_ERROR_STOP=1 -c 'SELECT version();'
PGSERVICE=newapi-production pg_dump --format=custom --file="$UPGRADE_BACKUP_DIR/main.dump"
pg_restore --list "$UPGRADE_BACKUP_DIR/main.dump" > "$UPGRADE_BACKUP_DIR/main.contents.txt"
PGSERVICE=newapi-production psql -X --csv -v ON_ERROR_STOP=1 -c \
  'SELECT id, quota, used_quota, aff_quota, aff_history FROM users ORDER BY id' \
  > "$UPGRADE_BACKUP_DIR/user-balances-before.csv"
```

`pg_restore --list` 只检查归档可读；实际恢复成功才证明备份可用。独立日志库执行同样的 `pg_dump`，使用不同归档名称。

3. 在独立测试 PostgreSQL 实例创建空库，将备份恢复到该库。以下服务名必须指向演练实例，不能指向生产库。

```sh
PGSERVICE=newapi-rehearsal-admin createdb newapi_rehearsal
PGSERVICE=newapi-rehearsal pg_restore --exit-on-error --single-transaction \
  --no-owner --no-privileges "$UPGRADE_BACKUP_DIR/main.dump"
```

4. 新程序连接恢复后的演练库，设置 `NODE_TYPE=master`，保持数据库检查开启。演练实例使用独立 Redis，隔离公网入口和出站网络，不接入真实支付回调、邮件任务或上游请求。检查主库和日志库迁移结果，再重启一次，确认没有重复结构修改。
5. 在无业务写入时导出升级后的用户余额，与 `user-balances-before.csv` 逐行比较；核对用户数、token 数、充值订单、订阅用量、奖励与分组卡、渠道配置和日志记录。检查旧 API key、用户登录/刷新、订阅限制与实际倍率配置；在受控测试渠道验证一次扣费与失败退款。

PostgreSQL 可先执行以下只读检查。发现重复配置或未知约束时，先在副本分析并确定业务上正确的处理方式，再安排生产修复。

```sql
SELECT "key", count(*) FROM options GROUP BY "key" HAVING count(*) > 1;
SELECT column_name, data_type FROM information_schema.columns
WHERE table_schema = current_schema() AND table_name = 'users'
  AND column_name IN ('quota', 'used_quota', 'aff_quota', 'aff_history');
SELECT to_timestamp(value::bigint) AS legacy_management_token_deadline
FROM options WHERE "key" = 'LegacyAccessTokenRetireAt';
```

## 正式切换与回滚

1. 演练通过后安排维护窗口。停止旧实例、相关消费者和定时任务，等待在途扣费/充值处理结束，完成最终数据库备份与余额快照。
2. **先只启动一个新版本主节点**，观察迁移完成和健康检查。不要让新旧版本同时迁移、写入同一个数据库。
3. 对比最终快照，确认余额、订单、订阅与本地功能数据一致；启动其余新版本实例，再恢复流量。保留迁移日志和首批扣费/充值核验记录。
4. 如果切换前失败，保持维护状态，分析错误。数据库可能已有部分 DDL 完成，不能仅换回旧镜像就假定恢复。
5. 需要回滚时，停止新版本写入，把升级前备份恢复到一个**新的空数据库**，验证后让旧镜像和原配置指向恢复库；独立日志库同步考虑。保留失败后的原库，便于排查。
6. 如果新版本已接受充值、扣费或新用户注册，先保存当前库并核对新增交易，再决定补账或修复方案。直接还原旧备份会丢失切换后的数据，不能无差别覆盖。

## 可复现的模拟存量升级测试

`scripts/test-upstream-upgrade.sh` 将指定旧提交导出到临时目录，在**空的本地临时数据库**建立旧结构和 500 用户 fixture，再用当前代码迁移两次。默认旧提交就是本次合并前提交。不要把这个测试脚本用于真实数据库或生产备份副本；真实副本按上面的应用启动流程演练。

```sh
# 预先创建空的本地 PostgreSQL 测试库；端口与凭据为示例。
UPGRADE_REHEARSAL_DSN='postgres://newapi_test:newapi_test@127.0.0.1:55432/newapi_upgrade?sslmode=disable' \
  ./scripts/test-upstream-upgrade.sh

# 另建一个空库，覆盖更早的 integer 钱包字段升级。
UPGRADE_REHEARSAL_INT32=1 \
UPGRADE_REHEARSAL_DSN='postgres://newapi_test:newapi_test@127.0.0.1:55432/newapi_upgrade_int32?sslmode=disable' \
  ./scripts/test-upstream-upgrade.sh

# SQLite 使用尚不存在的临时文件。
UPGRADE_REHEARSAL_DSN=local \
UPGRADE_REHEARSAL_SQLITE_PATH=/tmp/new-api-upgrade-rehearsal.db \
  ./scripts/test-upstream-upgrade.sh

# 在另一个空库直接验证新建数据库；同样执行重复迁移检查。
UPGRADE_REHEARSAL_MODE=fresh UPGRADE_REHEARSAL_DSN='本地空库 DSN' \
  go test ./model -run '^TestUpstreamDatabaseUpgradeRehearsal$' -count=1 -v
```

MySQL 使用标准 Go MySQL DSN，例如 `root:newapi_test@tcp(127.0.0.1:53306)/newapi_upgrade?charset=utf8mb4&parseTime=True&loc=Local`，以同样方式运行。

## 本次验证结果

| 验证项 | 结果 |
| --- | --- |
| PostgreSQL 16.15（aarch64 / Alpine） | 新库、合并前代码生成的存量库、integer 钱包字段扩容均通过；迁移两次，保留全部被核验旧列的数据；唯一约束继续生效。 |
| MySQL 8.0.46 | 同样通过新库、存量库、int 钱包扩容与重复迁移检查。 |
| SQLite 3.50.4 | 新库和存量库重复迁移通过。 |
| 500 用户数据检查 | 核验余额（含超过 int32 的值）、API token、充值订单、订阅/会话/渠道、奖励、分组卡、对战、检测、追踪、任务、日志与既有配置。第二次启动无 DDL。 |
| 独立日志数据库 | SQLite、MySQL、PostgreSQL 的扣费、结算、退款与审计记录测试通过；MySQL/PostgreSQL 日志库新建和旧结构升级测试通过。 |
| 后端 | `go test ./...` 通过；策略日志修复后的 `go test ./service` 通过；`relaykit/` 和 `tokenkit/` 内分别运行 `GOWORK=off go build ./...` 通过。 |
| 前端 | Bun 1.4.4 下 typecheck、生产构建通过；232 个测试文件、2,531 项测试通过，随后纯格式修复的主题测试再次通过。 |
| Lint | 上游原样代码 154 个 error；合并后 151 个，未新增 error，本次手动整合的文件无 lint error。全仓 lint 因上游已有问题仍不通过。 |
| 格式/国际化 | 手动整合的前端文件按项目格式化并保留版权头；Go 修改经过 gofmt；七语言 locale 同步完成；`git diff --check` 通过。 |

数据库专项测试命令如下，DSN 均指向临时测试实例。各组测试顺序执行，避免共享表冲突：

```sh
# 设置 TEST_MYSQL_DSN / TEST_POSTGRES_DSN，独立日志库另设 TEST_*_LOG_DSN。
go test ./model -run 'Migration|Migrate|Prefill|TokenKey|OptionPrimary|ChannelJSON|Scanner|Value' -count=1 -v
go test ./model -run 'DatabaseMatrix|DatabaseRoundTrip' -count=1 -v
go test ./controller -run 'DatabaseMatrix|DatabaseRoundTrip|DeleteRedemptionBatch' -count=1 -v

# 审计测试另设 AUDIT_MYSQL_DSN / AUDIT_POSTGRES_DSN。
go test ./controller -run 'AuditDatabaseMatrix|LogExportDatabaseMatrix|IndependentAuditLogStores' -count=1 -v

# 结算测试另设 TEST_FIXED_MYSQL_DSN / TEST_FIXED_POSTGRES_DSN，
# 以及 TEST_FIXED_MYSQL_LOG_DSN / TEST_FIXED_POSTGRES_LOG_DSN。
go test ./service -run '^TestFixedPriceBillingDatabaseMatrix$' -count=1 -v
```

上述验证没有覆盖 ClickHouse、所有最低支持版本或线上真实数据库；生产应使用与线上相同版本和实际备份继续演练。认证相关适配参考 [OWASP ASVS 5.0.0](https://owasp.org/www-project-application-security-verification-standard/) 的会话终止控制 `v5.0.0-7.4.1`、`v5.0.0-7.4.2`，以及 [Authentication](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html) / [Session Management](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html) Cheat Sheet，验证了策略封禁的认证版本失效、API token 停用、账务保留与策略拒绝不触发渠道禁用；这不是全项目 ASVS 认证声明。
