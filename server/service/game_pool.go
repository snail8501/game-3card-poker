package service

import (
	"context"
	"fmt"
	"game-3-card-poker/server/daley"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"sync"
)

// GamePool key is gameID
type GamePool struct {
	Conns map[string]*Game

	Mutex       sync.Mutex
	RedisClient *redis.Client
	DelayQueue  *daley.DelayQueue
	UserService *UserService
}

func (c GamePool) GetGame(gameId string, isNotExistAdd bool) (*Game, error) {
	conns := c.Conns[gameId]
	if conns == nil && isNotExistAdd {
		c.Mutex.Lock()
		defer c.Mutex.Unlock()

		// 初始化
		conns = &Game{
			GameId:      gameId,
			RedisClient: c.RedisClient,
			DelayQueue:  c.DelayQueue,
			UserService: c.UserService,
			Clients:     make(map[int64]map[string]*websocket.Conn, 0),
		}
		c.Conns[gameId] = conns
	}

	if conns == nil {
		return nil, fmt.Errorf("game is not exist error")
	}

	_, err := conns.GetGameRoom(context.Background())
	return conns, err
}

func (c GamePool) ConnOnline(gameId string, userId int64, token string, conn *websocket.Conn) (*Game, error) {
	conns, err := c.GetGame(gameId, true)
	if err != nil {
		return nil, err
	}

	if conns.Clients[userId] == nil {
		c.Mutex.Lock()
		defer c.Mutex.Unlock()

		conns.Clients[userId] = map[string]*websocket.Conn{token: conn}
	} else if conns.Clients[userId][token] == nil {
		c.Mutex.Lock()
		defer c.Mutex.Unlock()

		conns.Clients[userId][token] = conn
	}
	return c.Conns[gameId], nil
}

func (c GamePool) ConnOffline(gameId string, userId int64, token string) {
	conns, _ := c.GetGame(gameId, false)
	if conns != nil && conns.Clients != nil && conns.Clients[userId] != nil {
		c.Mutex.Lock()
		defer c.Mutex.Unlock()

		delete(conns.Clients[userId], token)
	}
}

func (c GamePool) CheckGameAvailability(gameId string) error {
	_, err := c.GetGame(gameId, true)
	if err != nil {
		return err
	}
	return nil

}
