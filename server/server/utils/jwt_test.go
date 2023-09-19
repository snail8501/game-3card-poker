package utils

import (
	"fmt"
	"testing"
)

func TestParseJWT(t *testing.T) {
	claim, err := ParseJWT("eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiIzLWNhcmQtZ2FtZSIsImV4cCI6MTY4NDk4MzY5OSwibmJmIjoxNjg0OTgwMDk5LCJpYXQiOjE2ODQ5ODAwOTksInVzZXJuYW1lIjoiMTIzNEAxNjMuY29tIn0.4cgu88P1FrpUYOMiCuOabNyqWxm6k5lCx2ocibc08Eg")
	if err != nil {
		fmt.Println("错误:", err)
		return
	}
	fmt.Println(claim.Email)
}
