package src

import (
	"context"
	"encoding/json"
	"fmt"
	"game-3-card-poker/server/config"
	"game-3-card-poker/server/constant"
	"game-3-card-poker/server/model"
	"game-3-card-poker/server/response"
	"game-3-card-poker/server/utils"
	"github.com/asaskevich/govalidator"
	"github.com/gorilla/websocket"
	"github.com/rs/cors"
	"go.uber.org/fx"
	"io"
	"log"
	"net"
	"net/http"
	"time"
)

func NewHTTPServer(lifecycle fx.Lifecycle, mux *http.ServeMux, c config.Configuration) {
	// 配置允许跨域请求
	options := cors.New(cors.Options{
		AllowCredentials: true,
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type", "Content-Length", "Accept-Encoding", "Authorization", "X-CSRF-Token"},
	})

	srv := &http.Server{Addr: fmt.Sprintf(":%d", c.Server.Port), Handler: options.Handler(mux)}
	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			ln, err := net.Listen("tcp", srv.Addr)
			if err != nil {
				return err
			}

			fmt.Println("Starting HTTP server at", srv.Addr)
			go srv.Serve(ln)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return srv.Shutdown(ctx)
		},
	})
}

// RequireAuth 拦截器验证用户是否登录
func RequireAuth(next RequestHandler) RequestHandler {
	return func(c *config.ServerConfig, w http.ResponseWriter, r *http.Request) {
		// 获取 URL 中的查询字符串参数
		queryParams := r.URL.Query()
		strToken := queryParams.Get("token")
		if len(strToken) <= 0 {
			tokenCookie, err := r.Cookie("token")
			if err != nil {
				response.Fail(constant.Code10012, constant.UserNotLogin, w)
				return
			}

			strToken = tokenCookie.Value
		}

		claims, err := utils.ParseJWT(strToken)
		if err != nil {
			response.Fail(constant.Code10012, constant.UserNotLogin, w)
			return
		}

		user, err := c.UserService.GetByEmail(claims.Email)
		if err != nil {
			response.Fail(constant.Code10012, constant.UserNotLogin, w)
			return
		}

		if !claims.LastModifyTime.Equal(user.UpdateAt) {
			response.Fail(constant.Code10012, constant.UserNotLogin, w)
			return
		}

		// 设置登录用户头信息
		r.Header.Set(constant.HeaderCustomUser, user.UserToJsonStr())
		r.Header.Set(constant.HeaderCustomToken, strToken)

		next(c, w, r)
	}
}

// RequireWebSocketAuth 拦截器验证用户是否登录
func RequireWebSocketAuth(next RequestHandler) RequestHandler {
	errorCallbackFunc := func(c *config.ServerConfig, w http.ResponseWriter, r *http.Request, response model.Response) {
		conn, err := c.WebSocket.Upgrade(w, r, nil)
		if err != nil {
			log.Print("upgrade:", err)
			return
		}

		defer func() {
			time.Sleep(time.Second)
			conn.Close()
		}()

		marshal, _ := json.Marshal(response)
		conn.WriteMessage(websocket.TextMessage, marshal)
	}

	return func(c *config.ServerConfig, w http.ResponseWriter, r *http.Request) {

		queryParams := r.URL.Query()
		strToken := queryParams.Get("token")
		if len(strToken) <= 0 {
			tokenCookie, err := r.Cookie("token")
			if err != nil {
				response.Fail(constant.Code10012, constant.UserNotLogin, w)
				return
			}

			strToken = tokenCookie.Value
		}

		// 校验jwt是否有效
		claims, err := utils.ParseJWT(strToken)
		if err != nil {
			errorCallbackFunc(c, w, r, model.Response{
				Code:    constant.Code10012,
				Message: constant.UserNotLogin,
			})
			return
		}

		user, err := c.UserService.GetByEmail(claims.Email)
		if err != nil {
			errorCallbackFunc(c, w, r, model.Response{
				Code:    constant.Code10012,
				Message: constant.UserNotLogin,
			})
			return
		}

		if !claims.LastModifyTime.Equal(user.UpdateAt) {
			errorCallbackFunc(c, w, r, model.Response{
				Code:    constant.Code10012,
				Message: constant.UserNotLogin,
			})
			return
		}

		// 获取 URL 中的查询字符串参数
		query2Params := r.URL.Query()
		strGameId := query2Params.Get("gameId")

		// 检查游戏GameId连接是否有效
		if errs := c.Game.CheckGameAvailability(strGameId); errs != nil {
			errorCallbackFunc(c, w, r, model.Response{
				Code:    constant.Code20001,
				Message: constant.GameNotExist,
			})
			return
		}

		// 设置登录用户头信息
		r.Header.Set(constant.HeaderCustomUser, user.UserToJsonStr())
		r.Header.Set(constant.HeaderCurrentGameId, strGameId)
		next(c, w, r)
	}
}

func NewServeMux(mux *http.ServeMux, c *config.ServerConfig) {

	middlewareAPI := NewBridgeBuilder(c).Build()
	middlewareAuth := NewBridgeBuilder(c).WithPostMiddlewares(RequireAuth).Build()
	middlewareSocketAuth := NewBridgeBuilder(c).WithPostMiddlewares(RequireWebSocketAuth).Build()

	mux.HandleFunc("/ws", middlewareSocketAuth(handlerSocketConnection))
	mux.HandleFunc("/api/game/create", middlewareAuth(handlerCreateGame))

	mux.HandleFunc("/api/user/login", middlewareAPI(handlerUserLogin))
	mux.HandleFunc("/api/user/logout", middlewareAuth(handlerUserLogout))
	mux.HandleFunc("/api/user/sendVerifyCode", middlewareAPI(handlerSendVerifyCode))
	mux.HandleFunc("/api/user/register", middlewareAPI(handlerUserRegister))
	mux.HandleFunc("/api/user/changePassword", middlewareAuth(handlerChangePassword))
}

// ParseBody parse the request body into the type of value.
func ParseBody(r io.Reader, value any) error {
	body, err := io.ReadAll(r)
	if err != nil {
		fmt.Printf("read body err, %v\n", err)
		return err
	}

	err = json.Unmarshal(body, &value)
	if err != nil {
		return fmt.Errorf("unable to parse body: %w", err)
	}

	valid, err := govalidator.ValidateStruct(value)

	if err != nil {
		return fmt.Errorf("unable to validate body: %w", err)
	}

	if !valid {
		return fmt.Errorf("Body is not valid")
	}

	return nil
}
