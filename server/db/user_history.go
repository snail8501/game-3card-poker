package db

import (
	"gorm.io/gorm"
	"time"
)

type UserHistory struct {
	ID            int64     `json:"id" gorm:"primaryKey;autoIncrement;not null"`
	UserId        int64     `json:"userId"`
	GameId        string    `json:"gameId"`
	RoundID       int       `json:"roundID"`
	Status        int       `json:"status"`
	Amount        int64     `json:"amount"`
	BalanceBefore int64     `json:"balanceBefore"`
	CreateAt      time.Time `json:"createTime" gorm:"autoCreateTime:milli; not null; default:(datetime('now', 'localtime'))"`
}

type UserHistoryDB struct {
	db *gorm.DB
}

func NewUserHistoryDB(db *gorm.DB) *UserHistoryDB {
	return &UserHistoryDB{db: db}
}

func (u *UserHistoryDB) CreateUserHistory(userHistory UserHistory) (UserHistory, error) {
	result := u.db.Model(&UserHistory{}).Create(&userHistory)
	return userHistory, result.Error
}

func (u *UserHistoryDB) List(userId string) ([]UserHistoryDB, error) {
	historys := make([]UserHistoryDB, 0, 16)
	result := u.db.Model(&UserHistory{}).Select(&historys).Where("userId = ?", userId)
	return historys, result.Error
}
