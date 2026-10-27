package model

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOptionPrimaryKeyMigrationPreservesAmbiguousSettings(t *testing.T) {
	for _, dialect := range []struct{ name, env string }{{"sqlite", ""}, {"mysql", "TEST_MYSQL_DSN"}, {"postgres", "TEST_POSTGRES_DSN"}} {
		t.Run(dialect.name, func(t *testing.T) {
			dsn := "local"
			if dialect.env != "" {
				dsn = os.Getenv(dialect.env)
				if dsn == "" {
					t.Skip("disposable test DSN not configured")
				}
			}
			previousPath := common.SQLitePath
			common.SQLitePath = filepath.Join(t.TempDir(), "options.db")
			t.Cleanup(func() { common.SQLitePath = previousPath })
			t.Setenv("OPTION_MIGRATION_TEST_DSN", dsn)
			db, _, err := chooseDB("OPTION_MIGRATION_TEST_DSN", false)
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			t.Cleanup(func() { _ = sqlDB.Close() })
			require.False(t, db.Migrator().HasTable("options"), "requires an isolated database without options")
			type legacyOption struct {
				Key   string `gorm:"size:191"`
				Value string `gorm:"type:text"`
			}
			require.NoError(t, db.Table("options").Migrator().CreateTable(&legacyOption{}))
			t.Cleanup(func() { _ = db.Migrator().DropTable("options") })
			original := []legacyOption{{Key: "ActualModelRatio", Value: `{"model":0.5}`}, {Key: "ActualModelRatio", Value: `{"model":2}`}}
			require.NoError(t, db.Table("options").Create(&original).Error)
			err = migrateOptionPrimaryKey(db)
			require.ErrorContains(t, err, "conflicting duplicate values")
			var remaining []legacyOption
			require.NoError(t, db.Table("options").Find(&remaining).Error)
			assert.ElementsMatch(t, original, remaining, "do not guess which price was intended")
			assert.False(t, db.Migrator().HasTable(optionPrimaryKeyTmpTable))
		})
	}
}
