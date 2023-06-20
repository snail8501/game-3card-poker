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
	"github.com/go-crypt/crypt"
	"github.com/go-crypt/crypt/algorithm/argon2"
	"log"
	"net/http"
	"strings"
	"time"
)

const (
	KeyVerifyCode = "verify-code:"
)

type TempUser struct {
	db.User
	Token string `json:"token"`
}

// handlerSendVerifyCode 验证码校验
func handlerSendVerifyCode(c *config.ServerConfig, w http.ResponseWriter, r *http.Request) {
	bodyJSON := service.SendCodeReq{}
	if err := ParseBody(r.Body, &bodyJSON); err != nil {
		response.ParamError(w)
		return
	}

	err := c.UserService.CheckUser(service.RegisterReq{
		Email: bodyJSON.Email,
	})

	if err != nil {
		if err.Error() == constant.EmailExist {
			response.Fail(constant.Code10002, constant.EmailExist, w)
			return
		}

		if err.Error() == constant.NicknameExist {
			response.Fail(constant.Code10005, constant.NicknameExist, w)
			return
		}
		response.SystemError(w)
		return
	}

	code := utils.GenValidateCode(6)
	msg := fmt.Sprintf("验证码：%s", code)
	c.RedisClient.SetEx(context.Background(), KeyVerifyCode+bodyJSON.Email, code, time.Minute*5)
	err = utils.SendEmail(c.SmtpAuth, c.Config.Smtp, bodyJSON.Email, msg)
	if err != nil {
		response.Fail(constant.Code10011, constant.VerifyCodeSendError, w)
		return
	}
	response.Success(w)
}

func handlerUserRegister(c *config.ServerConfig, w http.ResponseWriter, r *http.Request) {
	bodyJSON := service.RegisterReq{}
	if err := ParseBody(r.Body, &bodyJSON); err != nil {
		response.ParamError(w)
		return
	}

	value, err := c.RedisClient.Get(context.Background(), KeyVerifyCode+bodyJSON.Email).Result()
	if err != nil || !strings.EqualFold(value, bodyJSON.VerifyCode) {
		log.Println("redis error:", err)
		response.Fail(constant.Code10004, constant.VerifyCodeError, w)
		return
	}

	hashed, err := c.Hash.Hash(bodyJSON.Password)
	if err != nil {
		log.Println("redis error:", err)
		response.Fail(constant.Code10004, constant.VerifyCodeError, w)
		return
	}

	bodyJSON.Password = hashed.Encode()
	if _, errs := c.UserService.Register(bodyJSON); errs != nil {
		if errs.Error() == constant.EmailExist {
			response.Fail(constant.Code10002, constant.EmailExist, w)
			return
		}
		if errs.Error() == constant.NicknameExist {
			response.Fail(constant.Code10005, constant.NicknameExist, w)
			return
		}

		log.Println("register error:", err)
		response.SystemError(w)
		return
	}
	response.Success(w)
}

func handlerUserLogin(c *config.ServerConfig, w http.ResponseWriter, r *http.Request) {
	bodyJSON := service.LoginBody{}
	if err := ParseBody(r.Body, &bodyJSON); err != nil {
		response.ParamError(w)
		return
	}

	user, err := c.UserService.GetByEmail(bodyJSON.Email)
	if err != nil {
		log.Println("login GetByEmail error:", err)
		response.SystemError(w)
		return
	}

	// database password to argon2 algorithm
	match, err := func(encodePassword, loginPassword string) (match bool, err error) {
		decoder := crypt.NewDecoder()
		if err = argon2.RegisterDecoderArgon2id(decoder); err != nil {
			return false, err
		}

		digest, errs := decoder.Decode(encodePassword)
		if errs != nil {
			return false, errs
		}

		if match, err = digest.MatchAdvanced(loginPassword); err != nil {
			return false, err
		}

		return match, nil
	}(user.Password, bodyJSON.Password)

	if err != nil || !match {
		response.Fail(constant.Code10007, constant.LoginFailed, w)
		return
	}

	token, err := utils.CreateJWT(db.User{
		Email:    bodyJSON.Email,
		UpdateAt: user.UpdateAt,
	})

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

func handlerUserLogout(c *config.ServerConfig, w http.ResponseWriter, r *http.Request) {
	if err := c.RedisClient.Del(context.Background(), "token:"+r.Header.Get(constant.HeaderCustomToken)); err != nil {
		log.Println("create JWT error:", err)
	}

	email := r.Header.Get(constant.HeaderCustomUser)
	if email == "" {
		response.Fail(constant.Code10012, constant.UserNotLogin, w)
		return
	}
	response.Success(w)
}

func handlerChangePassword(c *config.ServerConfig, w http.ResponseWriter, r *http.Request) {
	var bodyJSON service.LoginBody
	if err := ParseBody(r.Body, &bodyJSON); err != nil {
		response.ParamError(w)
		return
	}

	strToken := r.Header.Get(constant.HeaderCustomToken)
	email, err := c.RedisClient.Get(context.Background(), "token:"+strToken).Result()
	if err != nil || len(email) == 0 {
		response.Fail(constant.Code10012, constant.UserNotLogin, w)
		return
	}

	user, err := c.UserService.GetByEmail(bodyJSON.Email)
	if user == (db.User{}) || err != nil {
		log.Println("get user by email error:", err)
		response.Fail(constant.Code10010, constant.UserNotExist, w)
		return
	}

	hashed, err := c.Hash.Hash(bodyJSON.Password)
	if err != nil {
		response.Fail(constant.Code10007, constant.LoginFailed, w)
		return
	}

	user.Password = hashed.Encode()
	if _, err = c.UserService.ChangePassword(user); err != nil {
		response.SystemError(w)
		return
	}

	if errs := c.RedisClient.Del(context.Background(), "token:"+strToken); errs != nil {
		response.SystemError(w)
		return
	}
	response.Success(w)
}
