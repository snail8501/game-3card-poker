package db

import (
	_ "embed"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	"log"
	"os"
	"path"

	_ "github.com/mattn/go-sqlite3"
)

func NewGameDB() *gorm.DB {

	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	homeFullDir := path.Join(homeDir, ".games")
	if errs := os.MkdirAll(homeFullDir, 0700); errs != nil {
		panic(errs)
	}

	// fixes error "database is locked", caused by concurrent access from deal goroutines to a single sqlite3 db connection
	db, err := gorm.Open(sqlite.Open(path.Join(homeFullDir, "3card.poker.db?cache=shared")), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
		},
	})
	if err != nil {
		panic(err)
	}

	if err = db.AutoMigrate(User{}, UserHistory{}); err != nil {
		log.Println("AutoMigrate error: ", err)
	}

	return db
}
