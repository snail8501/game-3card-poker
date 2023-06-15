package model

import "time"

type Game struct {
	ID         string    `json:"id" gorm:"primaryKey"`
	LowBet     int       `json:"lowBet" gorm:"not null"`
	TopBet     int       `json:"topBet" gorm:"not null"`
	Status     int       `json:"status" gorm:"not null; default:1"`
	CreateUser string    `json:"createUser" gorm:"not null"`
	CreateTime time.Time `json:"createTime" gorm:"autoCreateTime; not null"`
	UpdateTime time.Time `json:"updateTime" gorm:"autoCreateTime; not null"`
}

type CreateGameReq struct {
	LowBet     int    `json:"lowBet" binding:"required"`
	TopBet     int    `json:"topBet" binding:"required"`
	CreateUser string `json:"createUser" binding:"required"`
}
