package model

import "time"

type PushRecord struct {
	ID         int       `json:"id" gorm:"primaryKey; autoIncrement"`
	User       int       `json:"user" gorm:"not null"`
	Amount     int       `json:"amount" gorm:"not null"`
	CreateTime time.Time `json:"createTime" gorm:"autoCreateTime; not null"`
	UpdateTime time.Time `json:"updateTime" gorm:"autoUpdateTime; not null"`
}

type PushRecordReq struct {
	User   int `json:"user"`
	Amount int `json:"amount"`
}
