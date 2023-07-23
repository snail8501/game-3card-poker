package src

import (
	"context"
	"fmt"
	"game-3-card-poker/server/config"
	"game-3-card-poker/server/constant"
	"game-3-card-poker/server/db"
	"game-3-card-poker/server/response"
	"game-3-card-poker/server/service"
	"game-3-card-poker/server/utils"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

type TempUser struct {
	db.User
	Token string `json:"token"`
}

func canReceive(c *config.ServerConfig, address string) bool {
	user, err := c.UserService.GetByAddress(address)
	if err != nil {
		return false
	}
	if user == (db.User{}) {
		return false
	}
	// 金额大于等于1000不允许领取
	if user.Balance >= c.Config.User.ReceiveLimit {
		return false
	}
	ctx := context.Background()
	count, _ := c.RedisClient.Get(ctx, "receive:"+address).Int()
	// 领取次数大于等于3次不允许领取
	if count >= c.Config.User.ReceiveCount {
		return false
	}
	return true
}

// handlerReceiveCoin 领取金币
func handlerReceiveCoin(c *config.ServerConfig, w http.ResponseWriter, r *http.Request) {
	var receive service.ReceiveReq
	if err := ParseBody(r.Body, &receive); err != nil {
		response.ParamError(w)
		return
	}

	user := db.User{}
	if err := user.JsonStrToUser(r.Header.Get(constant.HeaderCustomUser)); err != nil {
		log.Println("json to user error: ", err)
	}

	// 金额大于等于1000不允许领取
	if user.Balance >= c.Config.User.ReceiveLimit {
		response.Fail(constant.Code10013, constant.BalanceThan1000, w)
		return
	}

	ctx := context.Background()
	count, _ := c.RedisClient.Get(ctx, "receive:"+user.Address).Int()
	// 领取次数大于等于3次不允许领取
	if count >= c.Config.User.ReceiveCount {
		response.Fail(constant.Code10009, constant.PushThanThree, w)
		return
	}

	if !receive.Receive {
		response.SuccessWithData(c.Config.User.ReceiveAmount, w)
		return
	}

	err := c.UserService.ReceiveCoin(c.Config.User.ReceiveAmount, user)
	if err != nil {
		response.SystemError(w)
		return
	}

	// 获取当前时间
	now := time.Now()
	// 获取目标时间
	target := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, 1)
	// 计算时间差
	duration := target.Sub(now)
	c.RedisClient.Set(ctx, "receive:"+user.Address, count+1, duration)
	response.Success(w)
}

func handlerGetSign(c *config.ServerConfig, w http.ResponseWriter, r *http.Request) {
	var bodyJson service.UserReq
	if err := ParseBody(r.Body, &bodyJson); err != nil {
		response.ParamError(w)
		return
	}
	response.SuccessWithData(fmt.Sprintf("%s%s", c.Config.User.SignatureMessage, bodyJson.Address), w)
	return
}

func handlerVerifySign(c *config.ServerConfig, w http.ResponseWriter, r *http.Request) {
	var verify service.Verify
	if err := ParseBody(r.Body, &verify); err != nil {
		response.ParamError(w)
		return
	}

	if success, _ := signatureVerify(c, verify); !success {
		response.Fail(constant.Code10007, constant.LoginFailed, w)
		return
	}

	// 设置随机数种子
	rand.Seed(time.Now().Unix())
	// 生成0-9之间的随机数
	randHeadPic := c.Config.User.DefaultHeadPic[rand.Intn(10)]
	user, err := c.UserService.SignatureVerify(verify.Address, c.Config.User.DefaultBalance, randHeadPic)
	if err != nil {
		response.SystemError(w)
		return
	}

	token, err := utils.CreateJWT(user.Address)
	if err != nil {
		log.Println("create JWT error:", err)
		response.SystemError(w)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    token,
		Path:     "/",
		Domain:   "",
		HttpOnly: false,
	})

	response.SuccessWithData(TempUser{user, token}, w)
}

func signatureVerify(c *config.ServerConfig, verify service.Verify) (bool, error) {
	var client = http.Client{}
	url := fmt.Sprintf("%s?address=%s&message=%s&signature=%s", c.Config.User.SignatureVerifyHost, verify.Address, fmt.Sprintf("%s%s", c.Config.User.SignatureMessage, verify.Address), verify.Signature)
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Println("http.NewRequest error:", err)
		return false, err
	}

	resp, err := client.Do(request)
	if err != nil {
		log.Println("Request verify url error:", err)
		return false, err
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("Failed to read HTTP response:", err)
		return false, err
	}

	if success := strings.TrimSpace(string(body)); strings.EqualFold(success, "true") {
		return true, nil
	}
	return false, nil
}

func handlerUpdateHeadPic(c *config.ServerConfig, w http.ResponseWriter, r *http.Request) {
	var jsonBody service.UpdateHeadPic
	if err := ParseBody(r.Body, &jsonBody); err != nil {
		response.ParamError(w)
		return
	}

	user := db.User{}
	if err := user.JsonStrToUser(r.Header.Get(constant.HeaderCustomUser)); err != nil {
		log.Println("json to user error: ", err)
	}

	err := c.UserService.UpdateHeadPic(db.User{Address: user.Address, HeadPic: jsonBody.HeadPic})
	if err != nil {
		response.SystemError(w)
		return
	}
	response.Success(w)
}

func handlerGetUser(c *config.ServerConfig, w http.ResponseWriter, r *http.Request) {
	user := db.User{}
	if err := user.JsonStrToUser(r.Header.Get(constant.HeaderCustomUser)); err != nil {
		log.Println("json to user error: ", err)
	}

	user, err := c.UserService.GetByAddress(user.Address)
	if err != nil {
		response.SystemError(w)
		return
	}
	response.SuccessWithData(user, w)
}

func handlerHeadList(c *config.ServerConfig, w http.ResponseWriter, r *http.Request) {
	response.SuccessWithData(c.Config.User.DefaultHeadPic, w)
	return
}

func handlerHistoryList(c *config.ServerConfig, w http.ResponseWriter, r *http.Request) {
	var jsonBody service.RequestHistory
	if err := ParseBody(r.Body, &jsonBody); err != nil {
		response.ParamError(w)
		return
	}

	list := c.UserService.GetHisotryRecordList(jsonBody.GameId)
	response.SuccessWithData(list, w)
	return
}
