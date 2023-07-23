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
		ID:       11,
		HeadPic:  "wwwwww",
		Balance:  1001,
		Address:  "Address",
		CreateAt: time.Time{},
		UpdateAt: time.Time{},
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
	user, err := userDB.GetByAddress("317911613@qq.com")
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(user)
}
