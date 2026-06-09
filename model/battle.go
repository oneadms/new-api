package model

import (
	"errors"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"

	"github.com/bytedance/gopkg/util/gopool"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrBattleQuotaInsufficient  = errors.New("battle quota insufficient")
	ErrBattleQuotaLimitExceeded = errors.New("battle quota limit exceeded")
	ErrBattleUserNotFound       = errors.New("battle user not found")
)

type BattleRecord struct {
	Id         int    `json:"id"`
	CreatedAt  int64  `json:"created_at" gorm:"bigint;index:idx_battle_records_created_at"`
	RoomId     string `json:"room_id" gorm:"size:64;index:idx_battle_records_room_id"`
	EventId    string `json:"event_id" gorm:"size:80;uniqueIndex:idx_battle_records_event_id"`
	FromUserId int    `json:"from_user_id" gorm:"index:idx_battle_records_from_user_id"`
	ToUserId   int    `json:"to_user_id" gorm:"index:idx_battle_records_to_user_id"`
	Quota      int    `json:"quota"`
	Reason     string `json:"reason" gorm:"size:64"`
}

type BattleQuotaUsage struct {
	Lost int `json:"lost"`
	Won  int `json:"won"`
}

type BattleQuotaLimit struct {
	Since int64
	Max   int
}

type BattleQuotaMutationParams struct {
	RoomId     string
	EventId    string
	UserId     int
	Quota      int
	Reason     string
	UsageLimit *BattleQuotaLimit
}

func DebitBattleQuota(params BattleQuotaMutationParams) (*BattleRecord, error) {
	if params.UserId <= 0 {
		return nil, errors.New("battle debit user id is invalid")
	}
	if params.Quota <= 0 {
		return nil, errors.New("battle debit quota must be positive")
	}
	if params.EventId == "" {
		return nil, errors.New("battle debit event id is required")
	}
	if err := validateBattleQuotaLimit(params.UsageLimit); err != nil {
		return nil, err
	}

	record := &BattleRecord{
		CreatedAt:  common.GetTimestamp(),
		RoomId:     params.RoomId,
		EventId:    params.EventId,
		FromUserId: params.UserId,
		Quota:      params.Quota,
		Reason:     params.Reason,
	}

	err := DB.Transaction(func(tx *gorm.DB) error {
		user, err := lockBattleUser(tx, params.UserId)
		if err != nil {
			return err
		}
		if user.Quota < params.Quota {
			return ErrBattleQuotaInsufficient
		}
		if err := enforceBattleQuotaLimit(tx, "from_user_id", params.UserId, params.Quota, params.UsageLimit); err != nil {
			return err
		}
		if err := tx.Create(record).Error; err != nil {
			return err
		}
		result := tx.Model(&User{}).
			Where("id = ? AND quota >= ?", params.UserId, params.Quota).
			Update("quota", gorm.Expr("quota - ?", params.Quota))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrBattleQuotaInsufficient
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	gopool.Go(func() {
		if err := cacheDecrUserQuota(params.UserId, int64(params.Quota)); err != nil {
			common.SysLog("failed to decrease battle user quota cache: " + err.Error())
		}
	})
	RecordLog(params.UserId, LogTypeSystem, fmt.Sprintf("多人枪战掉落额度 %s", logger.LogQuota(params.Quota)))
	return record, nil
}

func CreditBattleQuota(params BattleQuotaMutationParams) (*BattleRecord, error) {
	if params.UserId <= 0 {
		return nil, errors.New("battle credit user id is invalid")
	}
	if params.Quota <= 0 {
		return nil, errors.New("battle credit quota must be positive")
	}
	if params.EventId == "" {
		return nil, errors.New("battle credit event id is required")
	}
	if err := validateBattleQuotaLimit(params.UsageLimit); err != nil {
		return nil, err
	}

	record := &BattleRecord{
		CreatedAt: common.GetTimestamp(),
		RoomId:    params.RoomId,
		EventId:   params.EventId,
		ToUserId:  params.UserId,
		Quota:     params.Quota,
		Reason:    params.Reason,
	}

	err := DB.Transaction(func(tx *gorm.DB) error {
		if _, err := lockBattleUser(tx, params.UserId); err != nil {
			return err
		}
		if err := enforceBattleQuotaLimit(tx, "to_user_id", params.UserId, params.Quota, params.UsageLimit); err != nil {
			return err
		}
		if err := tx.Create(record).Error; err != nil {
			return err
		}
		result := tx.Model(&User{}).
			Where("id = ?", params.UserId).
			Update("quota", gorm.Expr("quota + ?", params.Quota))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrBattleUserNotFound
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	gopool.Go(func() {
		if err := cacheIncrUserQuota(params.UserId, int64(params.Quota)); err != nil {
			common.SysLog("failed to increase battle user quota cache: " + err.Error())
		}
	})
	RecordLog(params.UserId, LogTypeSystem, fmt.Sprintf("多人枪战拾取额度 %s", logger.LogQuota(params.Quota)))
	return record, nil
}

func validateBattleQuotaLimit(limit *BattleQuotaLimit) error {
	if limit == nil {
		return nil
	}
	if limit.Since < 0 {
		return errors.New("battle quota limit start is invalid")
	}
	if limit.Max < 0 {
		return errors.New("battle quota limit must not be negative")
	}
	return nil
}

func lockBattleUser(tx *gorm.DB, userId int) (*User, error) {
	var user User
	query := tx.Select("id", "quota").Where("id = ?", userId)
	if !common.UsingSQLite {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := query.First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrBattleUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

func enforceBattleQuotaLimit(tx *gorm.DB, userColumn string, userId int, quota int, limit *BattleQuotaLimit) error {
	if limit == nil {
		return nil
	}
	used, err := getBattleQuotaTotal(tx, userColumn, userId, limit.Since)
	if err != nil {
		return err
	}
	if used+int64(quota) > int64(limit.Max) {
		return ErrBattleQuotaLimitExceeded
	}
	return nil
}

func getBattleQuotaTotal(db *gorm.DB, userColumn string, userId int, since int64) (int64, error) {
	var total int64
	err := db.Model(&BattleRecord{}).
		Where(userColumn+" = ? AND created_at >= ?", userId, since).
		Select("COALESCE(SUM(quota), 0)").
		Scan(&total).Error
	return total, err
}

func GetBattleQuotaUsageSince(userId int, since int64) (BattleQuotaUsage, error) {
	var usage BattleQuotaUsage
	if userId <= 0 {
		return usage, nil
	}

	lost, err := getBattleQuotaTotal(DB, "from_user_id", userId, since)
	if err != nil {
		return usage, err
	}

	won, err := getBattleQuotaTotal(DB, "to_user_id", userId, since)
	if err != nil {
		return usage, err
	}

	usage.Lost = int(lost)
	usage.Won = int(won)
	return usage, nil
}

func BattleDailyUsageStart() int64 {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
}

func GetBattleDailyQuotaUsage(userId int) (BattleQuotaUsage, error) {
	return GetBattleQuotaUsageSince(userId, BattleDailyUsageStart())
}
