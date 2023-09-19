package src

import (
	"context"
	"encoding/json"
	"errors"
	"game-3-card-poker/server/config"
	"game-3-card-poker/server/constant"
	"game-3-card-poker/server/db"
	"game-3-card-poker/server/response"
	"game-3-card-poker/server/service"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"strings"
	"time"
)

type ReceiveMsg struct {
	Type      int   `json:"type"`      // 消息类型
	CurrRound int   `json:"currRound"` // 当前第几局
	BetChips  int64 `json:"betChips"`  // 下注筹码(跟注/加注)
	CompareId int64 `json:"compareId"` // 比牌的用户ID
	IsAutoBet bool  `json:"isAutoBet"` // 是否配置自动下注
}

type CreateGameReq struct {
	Minimum     int   `json:"minimum"`     // 最低人数
	LowBetChips int64 `json:"lowBetChips"` // 最低下注筹码
	TopBetChips int64 `json:"topBetChips"` // 封顶下注筹码
	TotalRounds int   `json:"totalRounds"` // 总游戏局数
}

func handlerSocketConnection(c *config.ServerConfig, w http.ResponseWriter, r *http.Request) {
	// 完成和Client HTTP >>> WebSocket的协议升级
	conn, err := c.WebSocket.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer conn.Close()

	// Error send message
	errMsgFunc := func(errMsg error) {
		errMessage := service.ErrorMessage{ErrorMsg: errMsg.Error()}
		conn.WriteMessage(websocket.TextMessage, errMessage.ToJsonStr(constant.EVENT_ERROR))
	}

	// Header json string to db.User
	user := db.User{}
	if errs := user.JsonStrToUser(r.Header.Get(constant.HeaderCustomUser)); errs != nil {
		defer func() {
			time.Sleep(time.Second)
			conn.Close()
		}()

		errMsgFunc(constant.UserNotExistError)
		return
	}

	token := conn.RemoteAddr().String()
	gameId := r.Header.Get(constant.HeaderCurrentGameId)

	// Register game Room
	Game, err := c.Game.ConnOnline(gameId, user.ID, token, conn)
	if err == nil {
		// UserJoinRoom 加入游戏
		if errs := Game.UserJoinRoom(user, false, nil, nil); errs != nil {
			// Error send message
			errMsgFunc(errs)
			return
		}
	}

	for {
		// Receive client message
		_, msg, errs := conn.ReadMessage()
		if errs != nil {
			// Client connection close
			c.Game.ConnOffline(gameId, user.ID, token)
			break
		}

		if msg != nil && len(msg) > 0 {
			var receiveMsg ReceiveMsg
			errors := json.Unmarshal(msg, &receiveMsg)
			if errors != nil {
				log.Println("receive message parse json error, ", errors)
				continue
			}

			var handlerErr error
			switch receiveMsg.Type {
			case constant.POKER_READY:
				// 0、开始游戏
				handlerErr = handlerUserJoinRoom(Game, user)
				break
			case constant.POKER_START:
				// 1、开始游戏->仅庄家操作
				handlerErr = handlerStartGame(Game, user.ID)
				break
			case constant.POKER_LOOK_CARD:
				// 2、看牌
				handlerErr = handlerLookCardGame(Game, user.ID, receiveMsg)
				break
			case constant.POKER_GIVE_UP:
				// 3、弃牌
				handlerErr = handlerGiveUpGame(Game, user.ID, receiveMsg)
				break
			case constant.POKER_BET:
				// 4、跟注/加注
				handlerErr = handlerBettingGame(Game, user.ID, false, receiveMsg)
				break
			case constant.POKER_COMPARE:
				// 5、下注比牌
				handlerErr = handlerBettingGame(Game, user.ID, true, receiveMsg)
				break
			case constant.POKER_AUTOBET:
				// 6、自动下注
				handlerErr = handlerAutoBetGame(Game, user.ID, receiveMsg)
				break
			}

			// Error send message
			if handlerErr != nil {
				errMsgFunc(handlerErr)
			}
		}
	}
}

func handlerUserJoinRoom(game *service.Game, user db.User) error {
	// 用户已准备好开始
	return game.UserJoinRoom(user, true, nil, func(gameRoom *service.GameRoom) error {
		dbUser, err := game.UserService.GetById(user.ID)
		if err != nil {
			log.Print("Query user error", err)
			return constant.UserNotExistError
		}

		// 用户筹码不足
		if dbUser.Balance < gameRoom.LowBetChips {
			return constant.UserNotEnoughBetError
		}

		return nil
	})
}

func handlerStartGame(game *service.Game, userId int64) error {
	// 设置游戏开始
	return game.StartGame(userId, func(gameRoom *service.GameRoom, joinUsers map[int64]*service.JoinUser, next func(map[int64]service.UserPoker) error) error {

		userIds := make([]int64, 0)
		for id := range joinUsers {
			userIds = append(userIds, id)
		}

		// 自动为每个用户生成3张扑克牌-提交上链-回滚不了
		cardPoker := service.CardPoker{}
		cardPoker.InitShufflePoker()
		cardPoker.CutCardPoker()
		userPokers := cardPoker.LicenseCardPoker(userIds)
		// todo 合约
		// game.SaveUserPoker(gameRoom, userPokers)
		// 整体放入同一个事物中
		return game.UserService.DeductAnteBetting(gameRoom.GameId, gameRoom.CurrRound, userIds, gameRoom.LowBetChips, func(balanceMap map[int64]int64) error {
			totalBetChips := int64(0)
			for id, betChips := range balanceMap {
				totalBetChips += betChips
				joinUsers[id].TotalBetChips += betChips

				// 下注筹码记录
				gameRoom.BetChips = append(gameRoom.BetChips, betChips)
			}

			gameRoom.TotalBetChips = totalBetChips
			gameRoom.ExposedBetChips = gameRoom.LowBetChips
			gameRoom.ConcealedBetChips = gameRoom.LowBetChips
			return next(userPokers)
		})
	})
}

func handlerLookCardGame(game *service.Game, userId int64, receiveMsg ReceiveMsg) error {
	// 设置链已查看状态
	return game.UserLookCard(userId, receiveMsg.CurrRound, func(gameRoom *service.GameRoom) (string, error) {
		// 获取链上3张牌值
		userPoker, err := game.GetUserPokerCache(context.Background(), gameRoom, userId)
		if err != nil {
			return "", err
		}
		return userPoker.ToString(), nil
	})
}

func handlerGiveUpGame(game *service.Game, userId int64, receiveMsg ReceiveMsg) error {
	return game.UserGiveUpCard(userId, receiveMsg.CurrRound, nil)
}

// handlerBettingGame (仅跟注/加注)或者(下注并与其他人Pk)
func handlerBettingGame(game *service.Game, userId int64, isPkCompare bool, receiveMsg ReceiveMsg) error {
	return game.UserBetting(userId, receiveMsg.CompareId, receiveMsg.CurrRound, receiveMsg.BetChips, nil, func(gameRoom *service.GameRoom, joinUser *service.JoinUser, callUpdateFunc func(bool, *service.UserPoker) error) error {

		isPkSuccess := false
		pkSuccPoker := &service.UserPoker{}

		// 获取userId,compareId两个用户的3张牌值,并比大小
		if isPkCompare {
			ctx := context.Background()
			userPoker, err := game.GetUserPokerCache(ctx, gameRoom, joinUser.UserId)
			if err != nil {
				return err
			}

			pkPoker, err := game.GetUserPokerCache(ctx, gameRoom, receiveMsg.CompareId)
			if err != nil {
				return err
			}

			// 两个游戏用户的3张牌值比大小
			isPkSuccess = userPoker.UserPokerPK(pkPoker)
			if isPkSuccess {
				// PK成功
				pkSuccPoker = pkPoker
			} else {
				// PK失败
				pkSuccPoker = userPoker
			}
		}

		// 整体放入同一个事物中
		// 扣除用户的跟注/加注筹码-操作数据库
		return game.UserService.DeductRaiseBetting(gameRoom.GameId, gameRoom.CurrRound, joinUser.UserId, receiveMsg.BetChips, func(betChips int64) error {
			// 记录全局下注最大值
			if joinUser.IsLookCard {
				// 明牌下注筹码
				gameRoom.ExposedBetChips = betChips
			} else {
				// 隐藏下注筹码
				gameRoom.ConcealedBetChips = betChips
			}

			// 游戏过程中PK记录(每局结束时，所有玩家只能看见自己比过或跟自己比过的玩家的手牌)
			if isPkCompare {
				gameRoom.Records = game.GetGamePkCompareRecord(gameRoom.Records, []int64{userId, receiveMsg.CompareId})
			}

			// 下注筹码记录
			gameRoom.BetChips = append(gameRoom.BetChips, betChips)

			joinUser.TotalBetChips += betChips
			gameRoom.TotalBetChips += betChips
			return callUpdateFunc(isPkSuccess, pkSuccPoker)
		})
	})
}

// handlerCompareGame 自动下注
func handlerAutoBetGame(game *service.Game, userId int64, receiveMsg ReceiveMsg) error {
	return game.UserSetAutoBetting(userId, receiveMsg.IsAutoBet, receiveMsg.CurrRound)
}

// handlerCreateGame 创建一个游戏房间
func handlerCreateGame(c *config.ServerConfig, w http.ResponseWriter, r *http.Request) {
	bodyJSON := CreateGameReq{}
	if err := ParseBody(r.Body, &bodyJSON); err != nil {
		response.ParamError(w)
		return
	}

	user := db.User{}
	if err := user.JsonStrToUser(r.Header.Get(constant.HeaderCustomUser)); err != nil {
		response.ParamError(w)
		return
	}

	gameRoom := &service.GameRoom{
		GameId:        strings.ReplaceAll(uuid.New().String(), "-", ""),
		JoinUsers:     make(map[int64]int, 0),
		Records:       make(map[int64][]int64, 0),
		BetChips:      make([]int64, 0),
		Minimum:       bodyJSON.Minimum,
		State:         constant.GAME_WAIT,
		TotalRounds:   bodyJSON.TotalRounds,
		CurrRound:     1,
		CurrLocation:  0,
		CurrTimeStamp: 0,
		CurrBetChips:  0,
		CurrBankerId:  user.ID,
		TotalBetChips: 0,
		LowBetChips:   bodyJSON.LowBetChips,
		TopBetChips:   bodyJSON.TopBetChips,
		CreateUser:    user.ID,
		CreateAt:      time.Now(),
	}

	conns, errs := c.Game.GetGame(gameRoom.GameId, true)
	if errs != nil && !errors.Is(errs, constant.CacheGetInfoError) && !errors.Is(errs, constant.GamePareError) {
		log.Println("CreateGame error:", errs)
		response.SystemError(w)
		return
	}

	// 创建游戏房间
	if errs = conns.CreateGames(gameRoom, user, nil); errs != nil {
		log.Println("CreateGame error:", errs)
		response.SystemError(w)
		return
	}

	response.SuccessWithData(gameRoom.GameId, w)
}
