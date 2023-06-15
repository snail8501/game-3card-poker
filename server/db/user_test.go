package db

import (
	"fmt"
	"testing"
	"time"
)

func TestUserDB_CreateUser(t *testing.T) {

	db := NewGameDB()
	userDB := NewUserDB(db)

	user := User{
		ID:         11,
		Email:      "317911613@qq.com",
		Password:   "1234567",
		Nickname:   "snail",
		HeadPic:    "wwwwww",
		Balance:    1001,
		ViewKey:    "ViewKey",
		PrivateKey: "PrivateKey",
		Address:    "Address",
		CreateTime: time.Time{},
		UpdateTime: time.Time{},
	}
	createUser, err := userDB.CreateUser(user)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(createUser)
}

func TestUserDB_QueryUser(t *testing.T) {
	db := NewGameDB()
	userDB := NewUserDB(db)
	user, err := userDB.QueryByEmail("317911613@qq.com")
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(user)
}
