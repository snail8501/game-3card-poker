package db

import (
	"fmt"
	"testing"
	"time"
)

func TestInitSQLite3(t *testing.T) {

	unix := time.Now().Unix()

	time.Sleep(20 * time.Second)

	diffTimeStamp := time.Now().Unix() - unix
	fmt.Println(diffTimeStamp)
}
