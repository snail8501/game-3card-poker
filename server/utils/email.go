package utils

import (
	"fmt"
	"game-3-card-poker/server/config"
	"github.com/jordan-wright/email"
	"math/rand"
	"net/smtp"
	"strings"
	"time"
)

// GenValidateCode Randomly generate numeric strings
func GenValidateCode(width int) string {
	numeric := [10]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	r := len(numeric)
	rand.Seed(time.Now().UnixNano())

	var sb strings.Builder
	for i := 0; i < width; i++ {
		fmt.Fprintf(&sb, "%d", numeric[rand.Intn(r)])
	}
	return sb.String()
}

// SendEmail send email
func SendEmail(auth smtp.Auth, config config.SMTPConfiguration, emailAddress string, msg string) error {
	e := email.NewEmail()
	//设置发送方的邮箱
	e.From = config.Sender
	// 设置接收方的邮箱
	e.To = []string{emailAddress}
	//设置主题
	e.Subject = config.Subject
	//设置文件发送的内容
	e.Text = []byte(msg)
	//设置服务器相关的配置
	err := e.Send(fmt.Sprintf("%s:%d", config.Host, config.Port), auth)
	return err
}
