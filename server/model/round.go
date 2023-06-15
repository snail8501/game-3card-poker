package model

import "time"

type Round struct {
	ID         int       `json:"id" gorm:"primaryKey; autoIncrement"`
	GameID     string    `json:"gameID"`
	Winner     string    `json:"winner"`
	WinAmount  int       `json:"winAmount"`
	CreateTime time.Time `json:"createTime" gorm:"autoCreateTime not null"`
	UpdateTime time.Time `json:"updateTime" gorm:"autoUpdateTime not null"`
}
