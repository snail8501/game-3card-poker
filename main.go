package main

import (
	"context"
	"embed"
	"game-3-card-poker/server"
	"game-3-card-poker/server/config"
	db2 "game-3-card-poker/server/db"
	"game-3-card-poker/server/service"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"io"
	"net/http"
)

//go:embed static/*
var staticFiles embed.FS

func main() {

	app := fx.New(
		fx.Provide(
			http.NewServeMux,
			config.NewConfiguration,
			db2.NewGameDB,
			db2.NewUserDB,
			db2.NewUserHistoryDB,
			config.NewRedisClient,
			config.NewEmailSmtpAuth,
			config.NewArgon2Password,
			config.NewLogger,
			config.NewWebSocket,
			service.NewUserService,
			config.NewServerConfig),
		fx.Invoke(src.NewHTTPServer, src.NewServeMux, NewTestStaticFile),

		// This is optional. With this, you can control where Fx logs
		// its events. In this case, we're using a NopLogger to keep
		// our test silent. Normally, you'll want to use an
		// fxevent.ZapLogger or an fxevent.ConsoleLogger.
		fx.WithLogger(
			func() fxevent.Logger {
				return fxevent.NopLogger
			},
		),
	)

	// 启动应用程序
	if err := app.Start(context.Background()); err != nil {
		panic(err)
	}

	// 等待应用程序关闭
	<-app.Done()
}

// NewTestStaticFile 测试静态文件
func NewTestStaticFile(mux *http.ServeMux) {

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		data, _ := staticFiles.ReadFile("static/test.html")
		if len(data) > 0 {
			io.WriteString(w, string(data))
		}
	})
}
