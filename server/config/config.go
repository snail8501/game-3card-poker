package config

import (
	"flag"
	"fmt"
	"game-3-card-poker/server/daley"
	"game-3-card-poker/server/service"
	"github.com/go-crypt/crypt/algorithm"
	"github.com/go-crypt/crypt/algorithm/argon2"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"path"
)

type ServerConfig struct {
	Config      Configuration
	Hash        algorithm.Hash
	SmtpAuth    smtp.Auth
	Game        *service.GamePool
	WebSocket   websocket.Upgrader
	RedisClient *redis.Client
	UserService *service.UserService
}

func NewConfiguration() Configuration {

	// Get the current working directory
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	// 定义命令行参数
	var configDir string
	flag.StringVar(&configDir, "config", dir, "config yml dir")

	// 解析命令行参数
	flag.Parse()

	viper.SetConfigType("yaml")
	viper.SetConfigFile(path.Join(configDir, "configuration.yml"))

	//读取配置文件
	if errs := viper.ReadInConfig(); errs != nil {
		panic(err)
	}

	//将配置文件读到结构体中
	var config Configuration
	if err = viper.Unmarshal(&config); err != nil {
		panic(err)
	}

	return config
}

func NewServerConfig(config Configuration, webSocket websocket.Upgrader, smtpAuth smtp.Auth, hasher *argon2.Hasher, redisClient *redis.Client, userService *service.UserService) *ServerConfig {

	connects := &service.GamePool{
		RedisClient: redisClient,
		UserService: userService,
		Conns:       make(map[string]*service.Game, 0),
	}

	// DelayQueue init
	connects.DelayQueue = daley.NewQueue("delay-queue", redisClient, func(message, idStr string) bool {
		if connects.Conns != nil {
			delayMsg := service.DelayMsg{}
			if err := delayMsg.ToDelayMsg(message); err != nil {
				log.Println("delay-queue error:", err)
			}

			Game, ok := connects.Conns[delayMsg.GameId]
			if ok {
				return Game.DelayCallback(delayMsg)
			}
		}
		return false
	})

	// 延迟队列初始化
	go func() {
		// start consume
		done := connects.DelayQueue.StartConsume()
		<-done
	}()

	return &ServerConfig{
		Game:        connects,
		Config:      config,
		Hash:        hasher,
		SmtpAuth:    smtpAuth,
		WebSocket:   webSocket,
		RedisClient: redisClient,
		UserService: userService,
	}
}

func NewArgon2Password(config Configuration) *argon2.Hasher {
	hash, err := argon2.New(
		argon2.WithVariantName(config.Argon2.Variant),
		argon2.WithT(config.Argon2.Iterations),
		argon2.WithM(uint32(config.Argon2.Memory)),
		argon2.WithP(config.Argon2.Parallelism),
		argon2.WithK(config.Argon2.KeyLength),
		argon2.WithS(config.Argon2.SaltLength),
	)

	if err != nil {
		panic(err)
	}
	return hash
}

func NewRedisClient(config Configuration) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", config.Redis.Host, config.Redis.Port),
		Password: config.Redis.Password,
		DB:       config.Redis.DatabaseIndex,
	})
}

func NewWebSocket() websocket.Upgrader {
	return websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			// allow Game only from example.com
			return true
		},
	}
}

func NewEmailSmtpAuth(c Configuration) smtp.Auth {
	return smtp.PlainAuth(c.Smtp.Identifier, c.Smtp.Username, c.Smtp.Password, c.Smtp.Host)
}

// NewLogger the documentation of the In and Out types for ways around this restriction.
func NewLogger() *log.Logger {
	logger := log.New(os.Stdout, "" /* prefix */, 0 /* flags */)
	logger.Print("Executing NewLogger.")
	return logger
}
