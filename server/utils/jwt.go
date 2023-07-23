package utils

import (
	"errors"
	"github.com/golang-jwt/jwt/v5"
	"log"
	"time"
)

type Claims struct {
	jwt.RegisteredClaims
	Address string `json:"address"`
}

const SECRET = "testKey"

// CreateJWT 生成JWT
func CreateJWT(address string) (string, error) {
	key := []byte(SECRET)
	claims := Claims{
		Address: address,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "3-card-game",                                      // 签发人
			IssuedAt:  jwt.NewNumericDate(time.Now()),                     // 签发时间
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 24)), // 过期时间
			NotBefore: jwt.NewNumericDate(time.Now()),                     // 生效时间
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(key)
	if err != nil {
		log.Println("create JWT error: ", err)
		return "", err
	}
	return tokenString, nil
}

// Secret 返回秘钥
func Secret() jwt.Keyfunc {
	return func(token *jwt.Token) (interface{}, error) {
		return []byte(SECRET), nil
	}
}

// ParseJWT 解析JWT
func ParseJWT(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, Secret())
	if err != nil {
		log.Println("parse JWT error: ", err)
		return nil, err
	}
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("couldn't parse this token")
}
