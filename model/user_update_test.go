package model

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupUserUpdateTestState(t *testing.T) {
	t.Helper()
	truncateTables(t)
	require.NoError(t, DB.Exec("DELETE FROM users").Error)

	oldRedisEnabled := common.RedisEnabled
	oldBatchUpdateEnabled := common.BatchUpdateEnabled
	common.RedisEnabled = false
	common.BatchUpdateEnabled = false
	t.Cleanup(func() {
		common.RedisEnabled = oldRedisEnabled
		common.BatchUpdateEnabled = oldBatchUpdateEnabled
	})
}

func TestUserUpdateDoesNotOverwriteAccountingFields(t *testing.T) {
	setupUserUpdateTestState(t)

	user := User{
		Id:              1,
		Username:        "quota-race-user",
		Password:        "password",
		DisplayName:     "before",
		Status:          common.UserStatusEnabled,
		Quota:           1000,
		UsedQuota:       20,
		RequestCount:    3,
		AffCount:        2,
		AffQuota:        300,
		AffHistoryQuota: 500,
	}
	require.NoError(t, DB.Create(&user).Error)

	staleUser, err := GetUserById(user.Id, true)
	require.NoError(t, err)

	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).Updates(map[string]interface{}{
		"quota":         gorm.Expr("quota - ?", 400),
		"used_quota":    gorm.Expr("used_quota + ?", 400),
		"request_count": gorm.Expr("request_count + ?", 1),
		"aff_count":     gorm.Expr("aff_count + ?", 1),
		"aff_quota":     gorm.Expr("aff_quota - ?", 100),
		"aff_history":   gorm.Expr("aff_history + ?", 200),
	}).Error)

	staleUser.DisplayName = "after"
	require.NoError(t, staleUser.Update(false))

	var got User
	require.NoError(t, DB.First(&got, user.Id).Error)
	assert.Equal(t, "after", got.DisplayName)
	assert.Equal(t, 600, got.Quota)
	assert.Equal(t, 420, got.UsedQuota)
	assert.Equal(t, 4, got.RequestCount)
	assert.Equal(t, 3, got.AffCount)
	assert.Equal(t, 200, got.AffQuota)
	assert.Equal(t, 700, got.AffHistoryQuota)
}

func TestUpdateUserSettingOnlyUpdatesSetting(t *testing.T) {
	setupUserUpdateTestState(t)

	user := User{
		Id:           2,
		Username:     "setting-user",
		Password:     "password",
		Status:       common.UserStatusEnabled,
		Quota:        1000,
		UsedQuota:    20,
		RequestCount: 3,
	}
	require.NoError(t, DB.Create(&user).Error)

	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).Updates(map[string]interface{}{
		"quota":         gorm.Expr("quota - ?", 250),
		"used_quota":    gorm.Expr("used_quota + ?", 250),
		"request_count": gorm.Expr("request_count + ?", 1),
	}).Error)

	require.NoError(t, UpdateUserSetting(user.Id, dto.UserSetting{Language: "zh"}))

	var got User
	require.NoError(t, DB.First(&got, user.Id).Error)
	assert.Equal(t, 750, got.Quota)
	assert.Equal(t, 270, got.UsedQuota)
	assert.Equal(t, 4, got.RequestCount)
	assert.Equal(t, "zh", got.GetSetting().Language)
}

func TestUpdateUserAccessTokenOnlyUpdatesAccessToken(t *testing.T) {
	setupUserUpdateTestState(t)

	oldToken := "old-access-token"
	user := User{
		Id:           3,
		Username:     "access-token-user",
		Password:     "password",
		DisplayName:  "before",
		AccessToken:  &oldToken,
		Status:       common.UserStatusEnabled,
		Quota:        1000,
		UsedQuota:    20,
		RequestCount: 3,
		AffQuota:     300,
	}
	require.NoError(t, DB.Create(&user).Error)

	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).Updates(map[string]interface{}{
		"display_name":  "concurrent-update",
		"quota":         gorm.Expr("quota - ?", 250),
		"used_quota":    gorm.Expr("used_quota + ?", 250),
		"request_count": gorm.Expr("request_count + ?", 1),
		"aff_quota":     gorm.Expr("aff_quota - ?", 50),
	}).Error)

	newToken := "new-access-token"
	require.NoError(t, UpdateUserAccessToken(user.Id, newToken))

	var got User
	require.NoError(t, DB.First(&got, user.Id).Error)
	assert.Equal(t, newToken, got.GetAccessToken())
	assert.Equal(t, "concurrent-update", got.DisplayName)
	assert.Equal(t, 750, got.Quota)
	assert.Equal(t, 270, got.UsedQuota)
	assert.Equal(t, 4, got.RequestCount)
	assert.Equal(t, 250, got.AffQuota)
}

func TestInviteRewardDoesNotOverwriteUserAccounting(t *testing.T) {
	setupUserUpdateTestState(t)

	oldReward := common.QuotaForInviter
	common.QuotaForInviter = 250
	t.Cleanup(func() {
		common.QuotaForInviter = oldReward
	})

	user := User{
		Id:              4,
		Username:        "inviter",
		Password:        "password",
		Status:          common.UserStatusEnabled,
		Quota:           1000,
		UsedQuota:       20,
		RequestCount:    3,
		AffCount:        2,
		AffQuota:        300,
		AffHistoryQuota: 500,
	}
	require.NoError(t, DB.Create(&user).Error)
	require.NoError(t, inviteUser(user.Id))

	var got User
	require.NoError(t, DB.First(&got, user.Id).Error)
	assert.Equal(t, 1000, got.Quota)
	assert.Equal(t, 20, got.UsedQuota)
	assert.Equal(t, 3, got.RequestCount)
	assert.Equal(t, 3, got.AffCount)
	assert.Equal(t, 550, got.AffQuota)
	assert.Equal(t, 750, got.AffHistoryQuota)
}

func TestInviteRewardRejectsAffiliateQuotaOverflow(t *testing.T) {
	setupUserUpdateTestState(t)

	oldReward := common.QuotaForInviter
	common.QuotaForInviter = 250
	t.Cleanup(func() {
		common.QuotaForInviter = oldReward
	})

	user := User{
		Id:              5,
		Username:        "overflow-inviter",
		Password:        "password",
		Status:          common.UserStatusEnabled,
		AffCount:        2,
		AffQuota:        common.MaxWalletQuota - 100,
		AffHistoryQuota: common.MaxWalletQuota - 100,
	}
	require.NoError(t, DB.Create(&user).Error)

	err := inviteUser(user.Id)
	require.ErrorIs(t, err, ErrUserQuotaLimitExceeded)

	var got User
	require.NoError(t, DB.First(&got, user.Id).Error)
	assert.Equal(t, 2, got.AffCount)
	assert.Equal(t, common.MaxWalletQuota-100, got.AffQuota)
	assert.Equal(t, common.MaxWalletQuota-100, got.AffHistoryQuota)
}

func TestTransferAffQuotaRejectsWalletOverflow(t *testing.T) {
	setupUserUpdateTestState(t)

	transferQuota := int(common.QuotaPerUnit)
	user := User{
		Id:       6,
		Username: "affiliate-transfer-overflow",
		Password: "password",
		Status:   common.UserStatusEnabled,
		Quota:    common.MaxWalletQuota - 100,
		AffQuota: transferQuota,
	}
	require.NoError(t, DB.Create(&user).Error)

	err := user.TransferAffQuotaToQuota(transferQuota)
	require.ErrorIs(t, err, ErrUserQuotaLimitExceeded)

	var got User
	require.NoError(t, DB.First(&got, user.Id).Error)
	assert.Equal(t, common.MaxWalletQuota-100, got.Quota)
	assert.Equal(t, transferQuota, got.AffQuota)
}

func TestEnsureEmailAvailableRejectsExistingEmailCaseInsensitive(t *testing.T) {
	setupUserUpdateTestState(t)

	require.NoError(t, DB.Create(&User{
		Username: "existing",
		Password: "old-password",
		Email:    "Taken@Example.com",
		Status:   common.UserStatusEnabled,
	}).Error)

	err := EnsureEmailAvailable(" taken@example.COM ", 0)
	require.ErrorIs(t, err, ErrEmailAlreadyTaken)

	user, err := GetUniqueUserByEmail("TAKEN@example.com")
	require.NoError(t, err)
	assert.Equal(t, "existing", user.Username)

	require.NoError(t, EnsureEmailAvailable("taken@example.com", user.Id))
}

func TestInsertRejectsDuplicateEmailWithoutUniqueIndex(t *testing.T) {
	setupUserUpdateTestState(t)

	require.NoError(t, DB.Create(&User{
		Username: "existing",
		Password: "old-password",
		Email:    "taken@example.com",
		Status:   common.UserStatusEnabled,
	}).Error)

	user := &User{
		Username: "oauth-user",
		Email:    "TAKEN@example.com",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}

	err := user.Insert(0)
	require.ErrorIs(t, err, ErrEmailAlreadyTaken)

	var count int64
	require.NoError(t, DB.Model(&User{}).Where("username = ?", "oauth-user").Count(&count).Error)
	assert.Zero(t, count)
}

func TestInsertKeepsBlankPasswordForPasswordlessUser(t *testing.T) {
	setupUserUpdateTestState(t)

	user := &User{
		Username: "passwordless-user",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}

	require.NoError(t, user.Insert(0))

	var stored User
	require.NoError(t, DB.Where("username = ?", user.Username).First(&stored).Error)
	assert.Empty(t, stored.Password)
}

func TestValidateAndFillRejectsPasswordlessUser(t *testing.T) {
	setupUserUpdateTestState(t)

	require.NoError(t, DB.Create(&User{
		Username: "passwordless-user",
		Password: "",
		Status:   common.UserStatusEnabled,
	}).Error)

	loginUser := User{
		Username: "passwordless-user",
		Password: "NewPassword123",
	}
	err := loginUser.ValidateAndFill()
	require.ErrorIs(t, err, ErrInvalidCredentials)

	var stored User
	require.NoError(t, DB.Where("username = ?", "passwordless-user").First(&stored).Error)
	assert.Empty(t, stored.Password)
}

func TestResetUserPasswordByEmailRequiresSingleActiveMatch(t *testing.T) {
	setupUserUpdateTestState(t)

	require.NoError(t, DB.Create(&User{
		Username: "duplicate-1",
		Password: "old-1",
		Email:    "legacy@example.com",
		AffCode:  "dupe1",
		Status:   common.UserStatusEnabled,
	}).Error)
	require.NoError(t, DB.Create(&User{
		Username: "duplicate-2",
		Password: "old-2",
		Email:    "LEGACY@example.com",
		AffCode:  "dupe2",
		Status:   common.UserStatusEnabled,
	}).Error)

	err := ResetUserPasswordByEmail("legacy@example.com", "NewPassword123")
	require.ErrorIs(t, err, ErrEmailAmbiguous)

	var duplicates []User
	require.NoError(t, DB.Where("LOWER(email) = ?", "legacy@example.com").Order("username asc").Find(&duplicates).Error)
	require.Len(t, duplicates, 2)
	assert.Equal(t, "old-1", duplicates[0].Password)
	assert.Equal(t, "old-2", duplicates[1].Password)

	require.NoError(t, DB.Create(&User{
		Username: "unique",
		Password: "old",
		Email:    "unique@example.com",
		AffCode:  "unique",
		Status:   common.UserStatusEnabled,
	}).Error)

	require.NoError(t, ResetUserPasswordByEmail("UNIQUE@example.com", "NewPassword123"))

	var unique User
	require.NoError(t, DB.Where("username = ?", "unique").First(&unique).Error)
	assert.True(t, common.ValidatePasswordAndHash("NewPassword123", unique.Password))

	err = ResetUserPasswordByEmail("missing@example.com", "NewPassword123")
	require.True(t, errors.Is(err, ErrEmailNotFound))
}
