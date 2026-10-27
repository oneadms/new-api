package model

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

// This fixture is also compiled against the deployed revision by
// scripts/test-upstream-upgrade.sh. It deliberately uses only its model API.
// The explicit rehearsal DSN must name a disposable, local database.
func TestUpstreamDatabaseUpgradeRehearsal(t *testing.T) {
	dsn := os.Getenv("UPGRADE_REHEARSAL_DSN")
	if dsn == "" {
		t.Skip("disposable upgrade database not configured")
	}
	mode := os.Getenv("UPGRADE_REHEARSAL_MODE")
	require.Contains(t, []string{"seed", "upgrade", "fresh"}, mode)
	require.True(t, dsn == "local" || strings.Contains(dsn, "127.0.0.1"), "rehearsal only accepts local databases")
	oldDB, oldLogDB := DB, LOG_DB
	oldMainType, oldLogType := common.MainDatabaseType(), common.LogDatabaseType()
	oldPath := common.SQLitePath
	t.Cleanup(func() {
		DB, LOG_DB = oldDB, oldLogDB
		common.SetDatabaseTypes(oldMainType, oldLogType)
		common.SQLitePath = oldPath
		initCol()
	})
	if dsn == "local" {
		common.SQLitePath = os.Getenv("UPGRADE_REHEARSAL_SQLITE_PATH")
		require.Contains(t, filepath.Base(common.SQLitePath), "new-api-upgrade")
	}
	db, dbType, err := chooseDB("UPGRADE_REHEARSAL_DSN", false)
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()
	DB, LOG_DB = db, db
	common.SetDatabaseTypes(dbType, dbType)
	initCol()
	if mode == "seed" || mode == "fresh" {
		require.False(t, db.Migrator().HasTable(&User{}), "seed/fresh requires an empty disposable database")
		require.NoError(t, migrateDB())
		require.NoError(t, migrateLOGDB())
		seedUpgradeRehearsal(t)
		if mode == "seed" && os.Getenv("UPGRADE_REHEARSAL_INT32") == "1" {
			for _, column := range []string{"quota", "used_quota", "aff_quota", "aff_history"} {
				switch db.Dialector.Name() {
				case "postgres":
					require.NoError(t, db.Exec(fmt.Sprintf(`ALTER TABLE users ALTER COLUMN "%s" TYPE integer USING "%s"::integer`, column, column)).Error)
				case "mysql":
					require.NoError(t, db.Exec("ALTER TABLE `users` MODIFY COLUMN `"+column+"` int DEFAULT 0").Error)
				}
			}
		}
	} else {
		var marker Option
		require.NoError(t, db.Where(commonKeyCol+" = ?", "__upgrade_rehearsal_fixture").First(&marker).Error)
		require.Equal(t, "500", marker.Value)
	}
	tables := []string{"users", "tokens", "top_ups", "subscription_plans", "user_subscriptions", "user_sessions", "channels", "recharge_reward_events", "user_group_passes", "battle_records", "quality_probe_runs", "sse_traces", "tasks", "logs"}
	columns := make(map[string][]string)
	before := make(map[string][]string)
	for _, table := range tables {
		columnTypes, err := db.Table(table).Migrator().ColumnTypes(table)
		require.NoError(t, err)
		for _, column := range columnTypes {
			columns[table] = append(columns[table], column.Name())
		}
		before[table] = upgradeTableRows(t, db, table, columns[table])
	}
	var options []Option
	require.NoError(t, db.Find(&options).Error)
	if mode == "seed" {
		t.Log("seeded deployed-version schema with 500 users and related records")
		return
	}
	var migrationLog bytes.Buffer
	DB = db.Session(&gorm.Session{Logger: logger.New(log.New(&migrationLog, "", 0), logger.Config{LogLevel: logger.Info})})
	LOG_DB = DB
	for pass := 1; pass <= 2; pass++ {
		migrationLog.Reset()
		require.NoError(t, migrateDB(), "migration pass %d", pass)
		require.NoError(t, migrateLOGDB(), "log migration pass %d", pass)
		if pass == 2 {
			assert.NotRegexp(t, regexp.MustCompile(`(?i)(ALTER TABLE|CREATE TABLE|DROP TABLE|RENAME TABLE|CREATE (UNIQUE )?INDEX|DROP INDEX)`), migrationLog.String(), "second startup must not rewrite schema")
		}
		for _, table := range tables {
			assert.Equal(t, before[table], upgradeTableRows(t, db, table, columns[table]), "%s data after pass %d", table, pass)
		}
		for _, option := range options {
			var current Option
			require.NoError(t, db.Where(commonKeyCol+" = ?", option.Key).First(&current).Error)
			assert.Equal(t, option.Value, current.Value, "option %s", option.Key)
		}
	}
	for _, entry := range []struct{ table, column string }{{"users", "username"}, {"tokens", "key"}} {
		var row map[string]any
		require.NoError(t, db.Table(entry.table).Take(&row).Error)
		delete(row, "id")
		err := db.Transaction(func(tx *gorm.DB) error { return tx.Table(entry.table).Create(&row).Error })
		assert.Error(t, err, "%s.%s uniqueness must survive", entry.table, entry.column)
	}
	t.Logf("%s: migrated twice; all original columns, 500 users, related records and uniqueness preserved", db.Dialector.Name())
}

func seedUpgradeRehearsal(t *testing.T) {
	t.Helper()
	const now int64 = 1793059200
	users := make([]User, 500)
	for i := range users {
		n := i + 1
		users[i] = User{Username: fmt.Sprintf("upgrade-user-%03d", n), Password: "test-only-password-hash", Email: fmt.Sprintf("user%d@example.invalid", n), AffCode: fmt.Sprintf("aff%04d", n), Group: "default", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Quota: 3_000_000_000 + n, UsedQuota: 3000 + n, AffQuota: 100 + n, AffHistoryQuota: 200 + n, RequestCount: n}
		access := fmt.Sprintf("%032d", n)
		users[i].AccessToken = &access
		if os.Getenv("UPGRADE_REHEARSAL_INT32") == "1" {
			users[i].Quota = 1_000_000 + n
		}
	}
	require.NoError(t, DB.CreateInBatches(&users, 50).Error)
	for _, user := range users {
		require.NoError(t, DB.Create(&Token{UserId: user.Id, Key: fmt.Sprintf("%048d", user.Id), Name: "existing API key", Group: "default", Status: common.TokenStatusEnabled, RemainQuota: 900000, UsedQuota: 1000, ExpiredTime: -1}).Error)
		require.NoError(t, DB.Create(&TopUp{UserId: user.Id, Amount: 10, Money: 10, TradeNo: fmt.Sprintf("upgrade-order-%d", user.Id), PaymentMethod: "epay", Status: "success", CreateTime: now, CompleteTime: now + 1}).Error)
	}
	for _, row := range []any{
		&Option{Key: "__upgrade_rehearsal_fixture", Value: "500"},
		&Option{Key: "ActualModelRatio", Value: `{"existing-model":0.5}`},
		&Option{Key: "ModelRatio", Value: `{"existing-model":1}`},
		&Option{Key: "SubscriptionDisabledGroups", Value: `["vip"]`},
		&Channel{Name: "existing channel", Type: 1, Status: common.ChannelStatusEnabled, Key: "test-only-channel-key", Group: "default", Models: "existing-model", Setting: common.GetPointer(`{"hide_upstream_error":true}`)},
		&SubscriptionPlan{Title: "existing plan", PriceAmount: 9.99, Currency: "USD", DurationUnit: "month", DurationValue: 1},
		&UserSubscription{UserId: users[0].Id, PlanId: 1, AmountTotal: 1_000_000, AmountUsed: 12345, StartTime: now, EndTime: now + 86400*30, Status: "active"},
		&UserSession{SID: "upgrade-session", UserID: users[0].Id, UserAuthVersion: 1, Version: 1, Status: "active", RefreshHash: strings.Repeat("a", 64), PreviousRefreshHash: strings.Repeat("b", 64), LoginMethod: "password", CreatedAt: now, LastActiveAt: now, ExpiresAt: now + 86400},
		&BattleRecord{RoomId: "upgrade-room", EventId: "upgrade-event", FromUserId: users[0].Id, ToUserId: users[1].Id, Quota: 10, Reason: "existing"},
		&QualityProbeRun{PublicID: "upgrade-quality", BatchID: 1, ChannelID: 1, Model: "existing-model", Status: "success", CreatedAt: now},
		&SSETrace{RequestId: "upgrade-trace", FilePath: "test-fixture-not-a-real-file", CreatedAt: now, ExpiresAt: now + 86400},
		&Task{TaskID: "upgrade-task", UserId: users[0].Id, Quota: 120, Status: TaskStatusSuccess},
		&Log{UserId: users[0].Id, Type: LogTypeConsume, Quota: 150, ModelName: "existing-model", Other: `{"actual_model_ratio":0.5,"model_ratio":1}`, CreatedAt: now},
	} {
		require.NoError(t, DB.Create(row).Error)
	}
	require.NoError(t, DB.Create(&RechargeRewardEvent{UserId: users[0].Id, TopUpId: 1, RechargeQuota: 100, GrantedCards: 1, LotteryDraws: 2, UsedDraws: 1, ConfigVersion: 1, CreatedAt: now}).Error)
	require.NoError(t, DB.Create(&UserGroupPass{UserId: users[0].Id, TemplateId: "upgrade-card", Name: "Existing pass", GroupName: "vip", DurationMinutes: 60, Status: "active", ExpiresAt: now + 86400, ActivatedAt: now, ActiveUntil: now + 3600, SourceType: "recharge", SourceId: "upgrade-order-1", CreatedAt: now}).Error)
}

func upgradeTableRows(t *testing.T, db *gorm.DB, table string, columns []string) []string {
	t.Helper()
	var rows []map[string]any
	selected := make([]clause.Column, 0, len(columns))
	for _, column := range columns {
		selected = append(selected, clause.Column{Name: column})
	}
	require.NoError(t, db.Table(table).Clauses(clause.Select{Columns: selected}).Find(&rows).Error)
	result := make([]string, 0, len(rows))
	for _, row := range rows {
		for key, value := range row {
			if b, ok := value.([]byte); ok {
				row[key] = string(b)
			}
		}
		encoded, err := common.Marshal(row)
		require.NoError(t, err)
		result = append(result, string(encoded))
	}
	slices.Sort(result)
	return result
}
