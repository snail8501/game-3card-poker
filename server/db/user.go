package db

import (
	"encoding/json"
	"gorm.io/gorm"
	"time"
)

type User struct {
	ID         int64     `json:"id" gorm:"primaryKey;autoIncrement;not null"`
	Email      string    `json:"email" gorm:"not null"`
	Password   string    `json:"-" gorm:"not null"`
	Nickname   string    `json:"nickname" gorm:"not null"`
	HeadPic    string    `json:"headPic"`
	Balance    int64     `json:"balance"`
	ViewKey    string    `json:"-"`
	PrivateKey string    `json:"-"`
	Address    string    `json:"-"`
	Record     string    `json:"-"`
	CreateAt   time.Time `json:"-" gorm:"autoCreateTime:milli; not null; default:(datetime('now', 'localtime'))"`
	UpdateAt   time.Time `json:"-" gorm:"not null; default:(datetime('now', 'localtime'))"`
}

func (u *User) UserToJsonStr() string {
	marshal, _ := json.Marshal(u)
	return string(marshal)
}

func (u *User) JsonStrToUser(jsonStr string) error {
	return json.Unmarshal([]byte(jsonStr), &u)
}

type UserDB struct {
	db *gorm.DB
}

func NewUserDB(db *gorm.DB) *UserDB {
	return &UserDB{db: db}
}

func (u *UserDB) CreateUser(user User) (User, error) {
	result := u.db.Model(&User{}).Create(&user)
	return user, result.Error
}

func (u *UserDB) QueryById(id int64) (User, error) {
	var user User
	result := u.db.Find(&user, id)
	return user, result.Error
}

func (u *UserDB) QueryByEmail(email string) (User, error) {
	var user User
	result := u.db.Find(&user, "email = ?", email)
	return user, result.Error
}

func (u *UserDB) QueryByNickname(nickname string) (User, error) {
	var user User
	result := u.db.Find(&user, "nickname = ?", nickname)
	return user, result.Error
}

func (u *UserDB) Update(user User) (User, error) {
	tx := u.db.Model(&user).Updates(&user)
	return user, tx.Error
}

func (u *UserDB) GetListByUserIds(userIds []int64) ([]*User, error) {
	var users []*User
	tx := u.db.Where("id IN ?", userIds).Find(&users)
	return users, tx.Error
}

func (u *UserDB) BettingTransaction(next func(tx *gorm.DB) error) error {
	return u.db.Transaction(func(tx *gorm.DB) error {
		return next(tx)
	})
}
