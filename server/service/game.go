package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"game-3-card-poker/server/constant"
	"game-3-card-poker/server/daley"
	"game-3-card-poker/server/db"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/jinzhu/copier"
	"github.com/redis/go-redis/v9"
	"log"
	"sort"
	"strings"
	"sync"
	"time"
)

// Game key is gameID
type Game struct {
	GameId  string
	Clients map[int64]map[string]*websocket.Conn

	Mutex       sync.Mutex
	RedisClient *redis.Client
	DelayQueue  *daley.DelayQueue
	UserService *UserService
}

// AutoBetDelayFunc 自动下注延迟队列
var AutoBetDelayFunc = func(c Game, gameRoom *GameRoom, joinUser *JoinUser) {
	if gameRoom.CurrLocation == joinUser.Location && joinUser.IsAutoBet {
		// 下注最低筹码
		lowBetChips, _ := c.GetCurrentLowBetChips(gameRoom, joinUser, nil)

		// 延迟1秒倒计时->(用户设置自动跟注)
		autBetMsg := DelayMsg{
			DelayType: constant.DELAY_AUTOBET,
			GameId:    gameRoom.GameId,
			UserId:    joinUser.UserId,
			CurrRound: gameRoom.CurrRound,
			Timestamp: gameRoom.SetLocationTime,
			BetChips:  lowBetChips,
		}
		c.DelayQueue.SendDelayMsg(autBetMsg.ToJsonStr(), time.Second, daley.WithRetryCount(5))
	}
}

// TimeOutGiveUpDelayFunc 超时用户自动放弃
var TimeOutGiveUpDelayFunc = func(c Game, gameRoom *GameRoom, joinUser *JoinUser) {
	if gameRoom.CurrLocation == joinUser.Location {
		// 延迟30秒倒计时->(超时用户自动放弃)
		delayMsg := DelayMsg{
			DelayType: constant.DELAY_GIVEUP,
			GameId:    gameRoom.GameId,
			UserId:    joinUser.UserId,
			CurrRound: gameRoom.CurrRound,
			Timestamp: gameRoom.SetLocationTime,
		}
		c.DelayQueue.SendDelayMsg(delayMsg.ToJsonStr(), CountdownSecond*time.Second, daley.WithRetryCount(5))
	}
}

// CheckAvailability 检查用户是否在当前游戏局中
func (c Game) CheckAvailability(ctx context.Context, userId int64) (*GameRoom, error) {
	gameRoom, err := c.GetGameRoom(ctx)
	if err != nil {
		return nil, err
	}

	// 游戏已结束
	if gameRoom.State == constant.GAME_ENDED {
		return gameRoom, constant.GameEndError
	}

	// 是否已加入
	value, ok := gameRoom.JoinUsers[userId]
	if !ok {
		return gameRoom, constant.GameNotInJoinError
	}

	// 不在当前游戏中(或者加入旁观者)
	if value != gameRoom.CurrRound {
		return gameRoom, constant.RoundError
	}
	return gameRoom, nil
}

// CheckPlaying 检查用户是否在当前游戏局中
func (c Game) CheckPlaying(ctx context.Context, userId int64, currRound int) (*GameRoom, error) {
	// 检查用户是否当前局游戏中
	gameRoom, err := c.CheckAvailability(ctx, userId)
	if err != nil {
		return gameRoom, err
	}

	// 主要是检查用户操作是否已过期(前端传currRound值)
	if gameRoom.CurrRound != currRound {
		// 操作已过期
		return gameRoom, constant.RoundNotCurrentError
	}

	// 当前用户是否游戏中状态
	joinUser := c.GetJoinUser(ctx, userId, gameRoom.CurrRound)
	if joinUser.State != constant.EVENT_PLAYING_USER {
		// 游戏状态不能操作-弃牌或者PK失败
		return gameRoom, constant.GameNotOperateError
	}
	return gameRoom, nil
}

// GetBroadcastMsg 获取广播消息
func (c Game) GetBroadcastMsg(ctx context.Context, gameRoom *GameRoom, eventMsg *EventMsg) ([]byte, error) {

	// 广播消息通知所有用户
	userIds := make([]int64, 0)
	joinUsers := make([]*JoinUser, 0)
	for userId := range gameRoom.JoinUsers {
		if user := c.GetJoinUser(ctx, userId, gameRoom.CurrRound); user != nil {
			userIds = append(userIds, user.UserId)
			joinUsers = append(joinUsers, user)
		}
	}

	// 从数据库中当前用户的账号筹码
	if userIds != nil && len(userIds) > 0 {
		userMap, err := c.UserService.GetUsersByIds(userIds)
		if err == nil && userMap != nil && len(userMap) > 0 {
			for index := range joinUsers {
				user := joinUsers[index]
				if dbUser, ok := userMap[user.UserId]; ok {
					user.AccountBetChips = dbUser.Balance
					user.HeadPic = dbUser.HeadPic
				}
			}
		}
	}

	// 升序
	sort.Slice(joinUsers, func(i, j int) bool { return joinUsers[i].Location < joinUsers[j].Location })

	var room GameRoom
	copier.Copy(&room, &gameRoom)
	if &room != nil {
		room.BetChips = make([]int64, 0)
		room.Records = make(map[int64][]int64, 0)
	}

	// 广播json字符串数组对象
	msgJsonByte, err := json.Marshal(&BroadcastMsg{
		Timestamp: time.Now().Unix(),
		Message:   Message{constant.EVENT_JOIN_USER, strings.ReplaceAll(uuid.New().String(), "-", "")},
		Event:     eventMsg,
		Room:      &room,
		Users:     joinUsers,
		BetChips:  gameRoom.BetChips,
	})
	return msgJsonByte, err
}

// BroadcastWinMsg 广播游戏状态->(每局结束时，所有玩家只能看见自己比过或跟自己比过的玩家的手牌)
func (c Game) BroadcastWinMsg(ctx context.Context, gameRoom *GameRoom, eventMsg *EventMsg) {
	if gameRoom == nil || c.Clients == nil || len(c.Clients) <= 0 {
		return
	}

	// 每局结束时，所有玩家只能看见自己比过或跟自己比过的玩家的手牌
	if gameRoom.Records != nil && len(gameRoom.Records) > 0 {
		for userId, records := range gameRoom.Records {
			cardList := make(map[int64]string, 0)
			userPoker, _ := c.GetUserPokerCache(context.Background(), gameRoom, userId)
			if userPoker != nil {
				cardList[userId] = userPoker.ToString()
			}

			if records != nil && len(records) > 0 {
				for index := range records {
					otherUserId := records[index]
					otherPoker, _ := c.GetUserPokerCache(context.Background(), gameRoom, otherUserId)
					if otherPoker != nil {
						cardList[otherUserId] = otherPoker.ToString()
					}
				}
			}

			// 游戏结束定向广播关联牌值
			message := CardEndMessage{CardList: cardList}
			c.SendMsgByUserId(ctx, gameRoom, userId, message.ToJsonStr(constant.EVENT_WIN_USER))
		}
	}

	// 广播消息
	c.BroadcastMsg(ctx, gameRoom, eventMsg)
}

// BroadcastMsg 广播游戏状态->所有在线用户
func (c Game) BroadcastMsg(ctx context.Context, gameRoom *GameRoom, eventMsg *EventMsg) {
	if gameRoom == nil || c.Clients == nil || len(c.Clients) <= 0 {
		return
	}

	// 广播消息
	msgJsonByte, _ := c.GetBroadcastMsg(ctx, gameRoom, eventMsg)
	for userId := range c.Clients {
		joinUser := c.GetJoinUser(ctx, userId, gameRoom.CurrRound)
		if joinUser == nil {
			continue
		}

		// 游戏等待中(没ready状态不主动推送消息)
		if gameRoom.State == constant.GAME_WAIT && gameRoom.CurrRound > 1 {
			if joinUser.State == constant.EVENT_JOIN_USER {
				// 上局加入过游戏
				lastJoinUser := c.GetJoinUser(ctx, userId, gameRoom.CurrRound-1)
				if lastJoinUser != nil && lastJoinUser.State > constant.EVENT_JOIN_USER {
					continue
				}
			}
		}

		clients := c.Clients[userId]
		for token := range clients {
			clients[token].WriteMessage(websocket.TextMessage, msgJsonByte)
		}
	}
}

// SendMsgByUserId 发送消息->指定用户
func (c Game) SendMsgByUserId(ctx context.Context, gameRoom *GameRoom, userId int64, msgJsonByte []byte) {

	if gameRoom == nil || c.Clients == nil || c.Clients[userId] == nil || len(c.Clients[userId]) <= 0 {
		return
	}

	joinUser := c.GetJoinUser(ctx, userId, gameRoom.CurrRound)
	if joinUser == nil {
		return
	}

	clients := c.Clients[userId]
	for token := range clients {
		clients[token].WriteMessage(websocket.TextMessage, msgJsonByte)
	}
}

// GetGamePkCompareRecord 游戏过程中PK记录(每局结束时，所有玩家只能看见自己比过或跟自己比过的玩家的手牌)
func (c Game) GetGamePkCompareRecord(records map[int64][]int64, userIds []int64) map[int64][]int64 {
	if records == nil {
		records = make(map[int64][]int64, 0)
	}

	addRecordFunc := func(userId int64) []int64 {
		userIdArr := records[userId]
		if userIdArr == nil {
			userIdArr = make([]int64, 0)
		}

		for index := range userIds {
			srcUserId := userIds[index]
			if userId == srcUserId {
				continue
			}

			if !func() bool {
				for indexArr := range userIdArr {
					if userIdArr[indexArr] == srcUserId {
						return true
					}
				}
				return false
			}() {
				userIdArr = append(userIdArr, srcUserId)
			}
		}
		return userIdArr
	}

	for addIndex := range userIds {
		addUserId := userIds[addIndex]
		records[addUserId] = addRecordFunc(addUserId)
	}
	return records
}

// CheckGameWinUser 最终赢家用户
func (c Game) CheckGameWinUser(ctx context.Context, gameRoom *GameRoom, joinUsers []*JoinUser) bool {

	// 游戏中玩家列表
	payingUsers := func() []*JoinUser {
		users := make([]*JoinUser, 0)
		for index := range joinUsers {
			user := joinUsers[index]
			if user.State == constant.EVENT_PLAYING_USER {
				users = append(users, user)
			}
		}
		return users
	}()

	var topRecordUserIdArr []int64

	getWinUserFunc := func() (bool, *JoinUser) {

		// 仅剩一个游戏玩家判定为赢家
		if len(payingUsers) == 1 {
			winJoinUser := payingUsers[0]
			gameRoom.Records[winJoinUser.UserId] = nil
			return true, winJoinUser
		}

		// 封顶全部开牌
		if gameRoom.TotalBetChips >= gameRoom.TopBetChips {
			joinUserMap := make(map[int64]*JoinUser, 0)
			pkUserPokers := make(map[int64]*UserPoker, 0)
			for index := range payingUsers {
				joinUser := payingUsers[index]
				joinUserMap[joinUser.UserId] = joinUser
				topRecordUserIdArr = append(topRecordUserIdArr, joinUser.UserId)

				// 获取用户底牌
				userPoker, err := c.GetUserPokerCache(ctx, gameRoom, joinUser.UserId)
				if err != nil {
					pkUserPokers[joinUser.UserId] = &UserPoker{}
					log.Printf("Get userId=%d poker cache error: %s", joinUser.UserId, err)
					continue
				}

				// 收集所有游戏中玩家的底牌
				pkUserPokers[joinUser.UserId] = userPoker
			}

			var pkUserId int64

			// map遍历比较获取最大牌游戏用户
			for userId, userPoker := range pkUserPokers {
				poker, ok := pkUserPokers[pkUserId]
				if !ok || userPoker.UserPokerPK(poker) {
					pkUserId = userId
				}
			}
			return true, joinUserMap[pkUserId]
		}

		return false, nil
	}

	// 判定是否检查到玩家判赢条件
	isWinGame, winJoinUser := getWinUserFunc()
	if !isWinGame {
		return false
	}

	// todo 最终赢家数据上链
	//go c.SaveRound(gameRoom, winJoinUser.UserId, gameRoom.TotalBetChips)

	// 整体放入同一个事物中
	// 总筹码提现到最终赢家账户-操作数据库
	records, err := c.UserService.UpateWinBetting(gameRoom.GameId, gameRoom.CurrRound, winJoinUser.UserId, gameRoom.TotalBetChips, func(betChips int64) error {
		winJoinUser.State = constant.EVENT_WIN_USER
		gameRoom.State = constant.GAME_ENDED

		// 更新游戏状态gameRoom,users
		users := make(map[int64]*JoinUser, 0)
		users[winJoinUser.UserId] = winJoinUser

		// 游戏过程中PK记录(每局结束时，所有玩家只能看见自己比过或跟自己比过的玩家的手牌)
		if topRecordUserIdArr != nil && len(topRecordUserIdArr) > 0 {
			gameRoom.Records = c.GetGamePkCompareRecord(gameRoom.Records, topRecordUserIdArr)
			for index := range payingUsers {
				payUser := payingUsers[index]
				if payUser.UserId != winJoinUser.UserId {
					// PK输家->用户状态
					payUser.State = constant.EVENT_COMPARE_LOSE_USER
					users[payUser.UserId] = payUser
				}
			}
		}

		return c.setBatchCache(ctx, gameRoom, users)
	})

	if err != nil {
		log.Println("Update win game user error", err)
		return true
	}

	//  检查游戏是否结束
	isGameOver := false
	if gameRoom.TotalRounds <= gameRoom.CurrRound {
		// 游戏结束
		isGameOver = true
		records = c.UserService.GetHisotryRecordList(gameRoom.GameId)
		if records != nil && len(records) > 0 {
			// 降序
			sort.Slice(records, func(i, j int) bool {
				return records[i].Amount > records[j].Amount
			})
		}
	}

	// 发送获胜者的广播消息->(每局结束时，所有玩家只能看见自己比过或跟自己比过的玩家的手牌)
	c.BroadcastWinMsg(ctx, gameRoom, &EventMsg{
		Type:            constant.EVENT_OVER,
		UserId:          winJoinUser.UserId,
		AnimationSecond: AnimationSecond,
		IsGameOver:      isGameOver,
		Records:         records,
	})

	// 下一局开始,获胜者成为新庄家
	if !isGameOver {
		// 默认自动加入下一局用户
		otherUsers := make([]*JoinUser, 0)
		for i := range joinUsers {
			user := joinUsers[i]
			// 其他非等待用户
			if user.State != constant.EVENT_JOIN_USER {
				otherUsers = append(otherUsers, user)
			}
		}

		// 延迟2秒执行
		go func() {
			timer := time.NewTimer(2 * time.Second)
			<-timer.C

			// 重置游戏设置
			gameRoom.CurrLocation = 0
			gameRoom.CurrTimeStamp = 0
			gameRoom.CurrBetChips = 0
			gameRoom.State = constant.GAME_WAIT
			gameRoom.CurrBankerId = winJoinUser.UserId
			gameRoom.CurrRound = gameRoom.CurrRound + 1
			gameRoom.ExposedBetChips = gameRoom.LowBetChips
			gameRoom.ConcealedBetChips = gameRoom.LowBetChips
			gameRoom.JoinUsers = make(map[int64]int, 0)
			gameRoom.Records = make(map[int64][]int64, 0)
			gameRoom.BetChips = make([]int64, 0)

			callFunc := func(room *GameRoom, joinUser map[int64]*JoinUser) {
				// 将当前获取赢家排第一位
				newUsers := make([]*JoinUser, 0)
				for index := range otherUsers {
					newUser := otherUsers[index]
					if newUser.UserId == winJoinUser.UserId {
						// 游戏赢家作为庄家排除在排序中
						continue
					}

					// 赢家排第一位,赢家之前通过+100000对应追加到尾部
					if newUser.Location < winJoinUser.Location {
						newUser.Location += 100000
					}

					newUser.State = constant.EVENT_JOIN_USER
					newUser.IsBanker = false
					newUser.IsLookCard = false
					newUser.TotalBetChips = 0
					newUser.IsAutoBet = false
					newUsers = append(newUsers, newUser)
				}

				// 升序
				sort.Slice(newUsers, func(i, j int) bool { return newUsers[i].Location < newUsers[j].Location })

				// 重新排序(Location=0表示庄家,其他从Location=+1开始)
				for index := range newUsers {
					newUser := newUsers[index]
					newUser.Location = index + 1
					joinUser[newUser.UserId] = newUser

					// 加入房间
					room.JoinUsers[newUser.UserId] = room.CurrRound
				}
			}

			c.CreateGames(gameRoom, db.User{
				ID:      winJoinUser.UserId,
				Address: winJoinUser.Address,
				HeadPic: winJoinUser.HeadPic,
			}, callFunc)
		}()
	}

	return true
}

// SetNextOperateUser 设置下个操作用户
func (c Game) SetNextOperateUser(ctx context.Context, gameRoom *GameRoom, operateLocation int) {

	// 确认是否游戏中
	if gameRoom.State != constant.GAME_PAYING {
		return
	}

	joinUsers := make([]*JoinUser, 0)
	for userId := range gameRoom.JoinUsers {
		if gameRoom.JoinUsers[userId] == gameRoom.CurrRound {
			joinUser := c.GetJoinUser(ctx, userId, gameRoom.CurrRound)
			if joinUser != nil {
				joinUsers = append(joinUsers, joinUser)
			}
		}
	}

	// 判定是否检查到玩家判赢条件
	if c.CheckGameWinUser(ctx, gameRoom, joinUsers) {
		return
	}

	// 校验操作用户与游戏中的位置是否匹配
	if gameRoom.CurrLocation != operateLocation {
		return
	}

	// 游戏中玩家列表
	payingUsers := func() []*JoinUser {
		users := make([]*JoinUser, 0)
		for index := range joinUsers {
			user := joinUsers[index]
			if user.State == constant.EVENT_PLAYING_USER {
				users = append(users, user)
			}
		}
		return users
	}()

	if len(payingUsers) <= 0 {
		return
	}

	// 升序
	sort.Slice(payingUsers, func(i, j int) bool { return payingUsers[i].Location < payingUsers[j].Location })

	location := payingUsers[0].Location
	locationUsers := make(map[int]*JoinUser, 0)
	for index := range payingUsers {
		user := payingUsers[index]
		locationUsers[user.Location] = user

		if user.Location > gameRoom.CurrLocation {
			location = user.Location
			break
		}
	}

	// 更新游戏房间信息
	gameRoom.CurrLocation = location
	gameRoom.CurrTimeStamp = time.Now().Unix()
	err := c.setGameRoomCache(context.Background(), gameRoom)
	if err != nil {
		log.Println(err)
		return
	}

	// 延迟1秒执行
	go func() {
		timer := time.NewTimer(time.Second)
		<-timer.C

		operateUser := locationUsers[location]

		// 用户已设置自动跟注
		if operateUser.IsAutoBet {
			// 自动下注延迟队列
			AutoBetDelayFunc(c, gameRoom, operateUser)
		} else {
			// 超时自动放弃
			TimeOutGiveUpDelayFunc(c, gameRoom, operateUser)
		}

		// 下注最低筹码
		lowBetChips, _ := c.GetCurrentLowBetChips(gameRoom, operateUser, nil)

		// 广播消息通知所有用户
		eventMsg := EventMsg{
			Type:            constant.EVENT_CURRENT_USER,
			UserId:          operateUser.UserId,
			Location:        operateUser.Location,
			TotalSecond:     CountdownSecond,
			CountdownSecond: CountdownSecond,
			BetChips:        lowBetChips,
			ListBetChips:    c.GetListBetChips(gameRoom, lowBetChips),
		}
		c.BroadcastMsg(ctx, gameRoom, &eventMsg)
	}()
}

func (c Game) GetListBetChips(gameRoom *GameRoom, currentBetChips int64) []int64 {

	listBetChips := make([]int64, 0)
	if gameRoom.LowBetChips < currentBetChips {
		listBetChips = append(listBetChips, gameRoom.LowBetChips)
		listBetChips = append(listBetChips, currentBetChips)
		listBetChips = append(listBetChips, int64(float64(currentBetChips)*1.5))
		listBetChips = append(listBetChips, currentBetChips*2)
		listBetChips = append(listBetChips, int64(float64(currentBetChips)*2.5))
		listBetChips = append(listBetChips, currentBetChips*3)
	} else {
		listBetChips = append(listBetChips, gameRoom.LowBetChips)
		listBetChips = append(listBetChips, gameRoom.LowBetChips+1)
		listBetChips = append(listBetChips, gameRoom.LowBetChips+2)
		listBetChips = append(listBetChips, gameRoom.LowBetChips+3)
		listBetChips = append(listBetChips, gameRoom.LowBetChips+4)
		listBetChips = append(listBetChips, gameRoom.LowBetChips+5)
	}
	return listBetChips
}

// CreateGames 创建游戏
func (c Game) CreateGames(gameRoom *GameRoom, user db.User, callFunc func(*GameRoom, map[int64]*JoinUser)) error {
	// 更新游戏房间信息
	c.setGameRoomCache(context.Background(), gameRoom)

	// UserJoinRoom 庄家默认加入游戏
	return c.UserJoinRoom(user, false, callFunc, nil)
}

// GetCurrentLowBetChips 获取当前投注最低筹码
func (c Game) GetCurrentLowBetChips(gameRoom *GameRoom, joinUser *JoinUser, checkFunc func(int64) error) (int64, error) {
	if joinUser.IsLookCard {
		// 已看牌
		exposedBetChips := gameRoom.ConcealedBetChips * 2
		if exposedBetChips < gameRoom.ExposedBetChips {
			exposedBetChips = gameRoom.ExposedBetChips
		}

		if checkFunc != nil {
			if err := checkFunc(exposedBetChips); err != nil {
				return exposedBetChips, err
			}
		}
		return exposedBetChips, nil
	} else {
		concealedBetChips := gameRoom.ExposedBetChips / 2
		if concealedBetChips < gameRoom.ConcealedBetChips {
			concealedBetChips = gameRoom.ConcealedBetChips
		}

		if checkFunc != nil {
			if err := checkFunc(concealedBetChips); err != nil {
				return concealedBetChips, err
			}
		}
		return concealedBetChips, nil
	}
}

// DelayCallback 延迟队列消息
func (c Game) DelayCallback(delayMsg DelayMsg) bool {

	switch delayMsg.DelayType {
	case constant.DELAY_AUTOBET:
		err := c.UserBetting(delayMsg.UserId, 0, delayMsg.CurrRound, delayMsg.BetChips, func(gameRoom *GameRoom, joinUser *JoinUser) error {
			if gameRoom.CurrLocation != joinUser.Location || gameRoom.SetLocationTime != delayMsg.Timestamp {
				// 延续消息处理过期
				return constant.DelayOperateExpiredError
			}
			return nil
		}, func(gameRoom *GameRoom, joinUser *JoinUser, callUpdateFunc func(bool, *UserPoker) error) error {
			// 整体放入同一个事物中
			// 扣除用户的跟注/加注筹码-操作数据库
			return c.UserService.DeductRaiseBetting(gameRoom.GameId, gameRoom.CurrRound, joinUser.UserId, delayMsg.BetChips, func(betChips int64) error {
				if joinUser.IsLookCard {
					// 明牌下注筹码
					gameRoom.ExposedBetChips = betChips
				} else {
					// 隐藏下注筹码
					gameRoom.ConcealedBetChips = betChips
				}

				// 下注筹码记录
				gameRoom.BetChips = append(gameRoom.BetChips, betChips)

				joinUser.TotalBetChips += betChips
				gameRoom.TotalBetChips += betChips
				return callUpdateFunc(false, nil)
			})
		})

		if err != nil {
			if errors.Is(err, constant.GameRaisBetNotEnoughError) {
				//  下注金额不足取消自动操作
				c.UserSetAutoBetting(delayMsg.UserId, false, delayMsg.CurrRound)
			}
			log.Printf("operate delay userId=%d, auto betting error: %s", delayMsg.UserId, err.Error())
		}
		break
	case constant.DELAY_GIVEUP:
		// 超时用户自动放弃
		err := c.UserGiveUpCard(delayMsg.UserId, delayMsg.CurrRound, func(gameRoom *GameRoom, joinUser *JoinUser) error {
			if gameRoom.CurrLocation != joinUser.Location || gameRoom.SetLocationTime != delayMsg.Timestamp {
				// 延续消息处理过期
				return constant.DelayOperateExpiredError
			}

			// 游戏玩家是否设置自动下注
			if joinUser.IsAutoBet {
				// 自动下注延迟队列
				AutoBetDelayFunc(c, gameRoom, joinUser)

				// 用户设置自动下注操作
				return constant.UserSetAutoBettingError
			}
			return nil
		})

		if err != nil {
			log.Printf("operate delay userId=%d, auto give up card error: %s", delayMsg.UserId, err.Error())
		}
		break
	}
	return true
}

// StartGame 游戏开始并下底注
func (c Game) StartGame(startUserId int64, handlerFunc func(*GameRoom, map[int64]*JoinUser, func(map[int64]UserPoker) error) error) error {
	ctx := context.Background()

	// 全局方法加锁->防止死锁(连接、离线、看牌、弃牌、下注及比较、延迟队列、游戏开始-排除重新开局)
	c.Mutex.Lock()
	defer c.Mutex.Unlock()

	gameRoom, err := c.GetGameRoom(ctx)
	if err != nil {
		return err
	}

	// 游戏已结束
	if gameRoom.State == constant.GAME_ENDED {
		return constant.GameEndError
	}

	// 游戏已开始
	if gameRoom.State == constant.GAME_PAYING {
		return constant.GamePayingError
	}

	// 你不是庄家不能开始游戏
	if gameRoom.CurrBankerId != startUserId {
		return constant.GameNotAuthorityStartError
	}

	readyUsers := make([]*JoinUser, 0)
	for userId := range gameRoom.JoinUsers {
		if gameRoom.JoinUsers[userId] == gameRoom.CurrRound {
			joinUser := c.GetJoinUser(ctx, userId, gameRoom.CurrRound)
			if joinUser == nil {
				continue
			}

			// 统计已准备好开始游戏的用户
			if joinUser.State == constant.EVENT_READY_USER {
				readyUsers = append(readyUsers, joinUser)
			} else if joinUser.UserId == gameRoom.CurrBankerId {
				// 庄家可直接进入开始状态(忽略准备状态)
				readyUsers = append(readyUsers, joinUser)
			}
		}
	}

	// 游戏开始未达到人数
	if len(readyUsers) < gameRoom.Minimum {
		return constant.GameStartNotReachedNumberError
	}

	joinUsers := make(map[int64]*JoinUser, 0)
	for index := range readyUsers {
		// 设置用户状态
		joinUser := readyUsers[index]
		joinUser.State = constant.EVENT_PLAYING_USER
		joinUser.IsLookCard = false
		joinUsers[joinUser.UserId] = joinUser
	}

	// 自动扣除每个用户的底注
	errs := handlerFunc(gameRoom, joinUsers, func(userPokers map[int64]UserPoker) error {
		// 更新缓存 gameRoom，joinUsers
		gameRoom.CurrLocation = 0
		gameRoom.CurrTimeStamp = 0
		gameRoom.ExposedBetChips = gameRoom.LowBetChips
		gameRoom.ConcealedBetChips = gameRoom.LowBetChips
		gameRoom.State = constant.GAME_PAYING
		return c.setPokerBatchCache(ctx, gameRoom, joinUsers, userPokers)
	})

	if errs != nil {
		return errs
	}

	// 广播消息通知所有用户
	c.BroadcastMsg(ctx, gameRoom, &EventMsg{Type: constant.EVENT_PLAYING_USER})

	// SetNextOperateUser 设置下个操作用户
	c.SetNextOperateUser(ctx, gameRoom, gameRoom.CurrLocation)

	return nil
}

// UserJoinRoom 加入游戏
func (c Game) UserJoinRoom(loginUser db.User, isReadJoin bool, callFunc func(*GameRoom, map[int64]*JoinUser), handlerFunc func(gameRoom *GameRoom) error) error {
	ctx := context.Background()

	// 全局方法加锁->防止死锁(连接、离线、看牌、弃牌、下注及比较、延迟队列、游戏开始-排除重新开局)
	if callFunc == nil {
		c.Mutex.Lock()
		defer c.Mutex.Unlock()
	}

	// 指定当前用户发现消息
	sendMsgByIdFunc := func(gameRoom *GameRoom, eventMsg *EventMsg) {
		msgJsonByte, _ := c.GetBroadcastMsg(ctx, gameRoom, eventMsg)
		c.SendMsgByUserId(ctx, gameRoom, loginUser.ID, msgJsonByte)
	}

	// 检查用户是否当前局游戏中
	gameRoom, err := c.CheckAvailability(ctx, loginUser.ID)
	if err != nil && !errors.Is(err, constant.GameNotInJoinError) && !errors.Is(err, constant.RoundError) {
		if errors.Is(err, constant.GameEndError) {
			sendMsgByIdFunc(gameRoom, func() *EventMsg {
				records := c.UserService.GetHisotryRecordList(gameRoom.GameId)
				if records != nil && len(records) > 0 {
					// 降序
					sort.Slice(records, func(i, j int) bool {
						return records[i].Amount > records[j].Amount
					})
				}

				return &EventMsg{
					Type:       constant.EVENT_OVER,
					IsGameOver: true,
					UserId:     records[0].UserId,
					Records:    records,
				}
			}())
			return nil
		}
		return err
	}

	// 手动点击游戏开始,当前账号筹码是否足够
	if isReadJoin && handlerFunc != nil {
		if errs := handlerFunc(gameRoom); errs != nil {
			return errs
		}
	}

	// 当前游戏中
	if gameRoom.State == constant.GAME_PAYING {
		sendMsgByIdFunc(gameRoom, func() *EventMsg {
			eventMsg := &EventMsg{Type: constant.EVENT_CURRENT_USER}
			for userId := range gameRoom.JoinUsers {
				if user := c.GetJoinUser(ctx, userId, gameRoom.CurrRound); user != nil && user.Location == gameRoom.CurrLocation {
					// 下注最低筹码
					lowBetChips, _ := c.GetCurrentLowBetChips(gameRoom, user, nil)

					// 倒计时秒+1
					diffTimeStamp := time.Now().Unix() - gameRoom.CurrTimeStamp
					if diffTimeStamp > 0 {
						eventMsg.TotalSecond = CountdownSecond
						if diffTimeStamp >= CountdownSecond {
							eventMsg.CountdownSecond = CountdownSecond
						} else {
							eventMsg.CountdownSecond = (CountdownSecond - diffTimeStamp) + 1
						}
					}

					eventMsg.UserId = user.UserId
					eventMsg.Location = user.Location
					eventMsg.BetChips = lowBetChips
					eventMsg.ListBetChips = c.GetListBetChips(gameRoom, lowBetChips)
				}

				// 已看牌或者已弃牌
				if userId == loginUser.ID {
					joinUser := c.GetJoinUser(ctx, loginUser.ID, gameRoom.CurrRound)
					if joinUser != nil && (joinUser.IsLookCard || joinUser.State > constant.EVENT_PLAYING_USER) {
						// 获取链上3张牌值
						if userPoker, errs := c.GetUserPokerCache(context.Background(), gameRoom, userId); errs == nil {
							eventMsg.MyselfCard = userPoker.ToString()
						}
					}
				}
			}
			return eventMsg
		}())

		// 当前用户游戏中或者当前用户作为旁观者在房间中
		if err == nil || errors.Is(err, constant.RoundError) {
			return nil
		}

		// 游戏已开始不能加入
		return constant.GamePayingJoinError
	}

	// 默认加入游戏->等待开始
	state := constant.EVENT_JOIN_USER
	if isReadJoin {
		state = constant.EVENT_READY_USER
	}

	// 更新缓存状态,设置已看牌
	joinUser := c.GetJoinUser(ctx, loginUser.ID, gameRoom.CurrRound)
	if joinUser != nil {
		// 状态一致无须修改
		if state == joinUser.State || joinUser.State == constant.EVENT_READY_USER {
			sendMsgByIdFunc(gameRoom, nil)
			return nil
		}
	} else {
		joinUser = &JoinUser{
			UserId:        loginUser.ID,
			State:         state,
			Address:       loginUser.Address,
			HeadPic:       loginUser.HeadPic,
			IsBanker:      gameRoom.CurrBankerId == loginUser.ID,
			IsLookCard:    false,
			IsAutoBet:     false,
			Location:      len(gameRoom.JoinUsers),
			TotalBetChips: 0,
		}
	}

	gameRoom.JoinUsers[loginUser.ID] = gameRoom.CurrRound
	joinUsers := make(map[int64]*JoinUser, 0)
	joinUser.State = state
	joinUser.HeadPic = loginUser.HeadPic
	joinUsers[loginUser.ID] = joinUser

	// 自定义回调函数
	if callFunc != nil {
		callFunc(gameRoom, joinUsers)
	}

	// 保存缓存 gameRoom,joinUser
	if errs := c.setBatchCache(ctx, gameRoom, joinUsers); errs != nil {
		return errs
	}

	// callFunc非空表示开新一局游戏,可以不发送广播消息
	if callFunc == nil {
		// 广播消息通知所有用户->游戏开始
		c.BroadcastMsg(ctx, gameRoom, &EventMsg{
			Type:   joinUser.State,
			UserId: loginUser.ID,
		})
	}
	return nil
}

// UserLookCard 用户查看自己的底牌
func (c Game) UserLookCard(userId int64, currRound int, handlerFunc func(*GameRoom) (string, error)) error {
	ctx := context.Background()

	// 全局方法加锁->防止死锁(连接、离线、看牌、弃牌、下注及比较、延迟队列、游戏开始-排除重新开局)
	c.Mutex.Lock()
	defer c.Mutex.Unlock()

	// 检查用户是否当前局游戏中
	gameRoom, err := c.CheckPlaying(ctx, userId, currRound)
	if err != nil {
		return err
	}

	// 获取链上3张牌值
	cardStr, errors := handlerFunc(gameRoom)
	if errors != nil {
		return errors
	}

	// 更新缓存状态,设置已看牌
	joinUser := c.GetJoinUser(ctx, userId, gameRoom.CurrRound)
	if !joinUser.IsLookCard {
		joinUser.IsLookCard = true
		joinUser.IsAutoBet = false

		// 更新缓存 joinUser
		if errs := c.setJoinUserCache(ctx, gameRoom, joinUser); errs != nil {
			return errs
		}
	}

	// 发送消息->指定用户
	lowBetChips, _ := c.GetCurrentLowBetChips(gameRoom, joinUser, nil)
	message := CardMessage{Card: cardStr, BetChips: lowBetChips}
	c.SendMsgByUserId(context.Background(), gameRoom, userId, message.ToJsonStr(constant.EVENT_LOOK_CARD))

	// 广播消息通知所有用户
	go func() {
		time.Sleep(time.Second)
		c.BroadcastMsg(ctx, gameRoom, &EventMsg{
			Type:   constant.EVENT_LOOK_CARD,
			UserId: userId,
		})
	}()

	return nil
}

// UserGiveUpCard 用户弃牌
func (c Game) UserGiveUpCard(userId int64, currRound int, autoDelayFunc func(*GameRoom, *JoinUser) error) error {
	ctx := context.Background()

	// 全局方法加锁->防止死锁(连接、离线、看牌、弃牌、下注及比较、延迟队列、游戏开始-排除重新开局)
	c.Mutex.Lock()
	defer c.Mutex.Unlock()

	// 检查用户是否当前局游戏中
	gameRoom, err := c.CheckPlaying(ctx, userId, currRound)
	if err != nil {
		return err
	}

	joinUser := c.GetJoinUser(ctx, userId, gameRoom.CurrRound)
	if joinUser == nil {
		return constant.UserNotExistError
	}

	// 来自用户延迟自动延迟弃牌校验
	if autoDelayFunc != nil {
		if errs := autoDelayFunc(gameRoom, joinUser); errs != nil {
			return errs
		}
	}

	// 更新缓存状态,设置已看牌
	if joinUser.State != constant.EVENT_GIVE_UP_USER {
		joinUser.IsAutoBet = false
		joinUser.State = constant.EVENT_GIVE_UP_USER

		// 更新缓存 joinUser
		err = c.setJoinUserCache(ctx, gameRoom, joinUser)
		if err != nil {
			return err
		}
	}

	// 发送消息->指定用户
	userPoker, err := c.GetUserPokerCache(ctx, gameRoom, joinUser.UserId)
	if err == nil {
		// 下注最低筹码
		lowBetChips, _ := c.GetCurrentLowBetChips(gameRoom, joinUser, nil)
		message := CardMessage{Card: userPoker.ToString(), BetChips: lowBetChips}
		c.SendMsgByUserId(context.Background(), gameRoom, userId, message.ToJsonStr(constant.EVENT_LOOK_CARD))
	}

	// 广播消息通知所有用户
	c.BroadcastMsg(ctx, gameRoom, &EventMsg{
		Type:   constant.EVENT_GIVE_UP_USER,
		UserId: userId,
	})

	// SetNextOperateUser 设置下个操作用户
	c.SetNextOperateUser(ctx, gameRoom, joinUser.Location)

	return nil
}

type HandlerCompareFunc func(*GameRoom, *JoinUser, func(bool, *UserPoker) error) error

// UserBetting 用户跟注\加注
func (c Game) UserBetting(userId, compareId int64, currRound int, betChips int64, autoDelayFunc func(*GameRoom, *JoinUser) error, handlerFunc HandlerCompareFunc) error {
	ctx := context.Background()

	// 全局方法加锁->防止死锁(连接、离线、看牌、弃牌、下注及比较、延迟队列、游戏开始-排除重新开局)
	c.Mutex.Lock()
	defer c.Mutex.Unlock()

	// 检查用户是否当前局游戏中
	gameRoom, err := c.CheckPlaying(ctx, userId, currRound)
	if err != nil {
		return err
	}

	// 检查当前用户的状态是否符合要求
	joinUser := c.GetJoinUser(ctx, userId, gameRoom.CurrRound)
	if joinUser == nil {
		return constant.GameNotInJoinError
	}

	// 用户状态错误不能操作
	if joinUser.State != constant.EVENT_PLAYING_USER {
		return constant.GameNotOperateError
	}

	// 非当前操作用户,请等待
	if joinUser.Location != gameRoom.CurrLocation {
		return constant.NotCurrentOperateError
	}

	// 来自用户自动延迟下注校验
	if autoDelayFunc != nil {
		if errs := autoDelayFunc(gameRoom, joinUser); errs != nil {
			return errs
		}
	}

	// 检查下注筹码是否符合要求
	_, err = c.GetCurrentLowBetChips(gameRoom, joinUser, func(lowBetChips int64) error {
		// 下注筹码不能低于前者
		if lowBetChips > betChips {
			return constant.GameRaisBetNotEnoughError
		}
		return nil
	})

	if err != nil {
		return err
	}

	// 下注并找用户比较大小(达到封顶则直接进入全部比牌)
	isPkRequest := false
	var compareUser *JoinUser
	if compareId > 0 && (gameRoom.TotalBetChips+betChips) < gameRoom.TopBetChips {

		// PK类型的请求
		isPkRequest = true

		// pk对象不能是自己
		if userId == compareId {
			return constant.GamePkUserMySelfError
		}

		// 检查当前用户的状态是否符合要求
		compareUser = c.GetJoinUser(ctx, compareId, gameRoom.CurrRound)
		if compareUser == nil || compareUser.State != constant.EVENT_PLAYING_USER {
			return constant.GamePkUserInvalidError
		}
	}

	pkResult := false
	errs := handlerFunc(gameRoom, joinUser, func(isPkSuccess bool, failUserPoker *UserPoker) error {
		// 记录比牌结果值
		pkResult = isPkSuccess

		switch isPkRequest {
		case true:
			// 设置默认pk失败的用户
			pkFailUserId := userId

			// PK类型的请求
			if isPkSuccess {
				// 设置对方PK失败
				compareUser.State = constant.EVENT_COMPARE_LOSE_USER
				pkFailUserId = compareUser.UserId
			} else {
				// 设置自己PK失败
				joinUser.State = constant.EVENT_COMPARE_LOSE_USER
			}

			// 更新缓存 gameRoom,joinUser
			joinUsers := make(map[int64]*JoinUser, 0)
			joinUsers[joinUser.UserId] = joinUser
			joinUsers[compareUser.UserId] = compareUser
			errs := c.setBatchCache(ctx, gameRoom, joinUsers)
			if errs == nil {
				// PK失败方明牌
				message := CardMessage{Card: failUserPoker.ToString()}
				c.SendMsgByUserId(context.Background(), gameRoom, pkFailUserId, message.ToJsonStr(constant.EVENT_COMPARE_LOSE_USER))
			}

			return errs
		default:
			return c.setJoinUserCache(ctx, gameRoom, joinUser)
		}
	})

	if errs != nil {
		return errs
	}

	// 操作事件
	eventMsg := &EventMsg{
		Type:     constant.EVENT_BET_CHIPS,
		UserId:   userId,
		BetChips: betChips,
	}

	// PK类型的请求
	if isPkRequest {
		eventMsg.CompareId = compareId
		eventMsg.Type = constant.EVENT_COMPARE_LOSE_USER
		eventMsg.AnimationSecond = AnimationSecond
		eventMsg.WinUserId = userId
		if !pkResult {
			// 设置自己PK失败
			eventMsg.WinUserId = compareId
		}
	}

	// 广播消息通知所有用户
	c.BroadcastMsg(ctx, gameRoom, eventMsg)

	// SetNextOperateUser 设置下个操作用户
	c.SetNextOperateUser(ctx, gameRoom, joinUser.Location)

	return err
}

// UserSetAutoBetting 用户设置自动下注
func (c Game) UserSetAutoBetting(userId int64, isAutoBet bool, currRound int) error {
	ctx := context.Background()

	// 全局方法加锁->防止死锁(连接、离线、看牌、弃牌、下注及比较、延迟队列、游戏开始-排除重新开局)
	c.Mutex.Lock()
	defer c.Mutex.Unlock()

	gameRoom, err := c.CheckAvailability(ctx, userId)
	if err != nil {
		return err
	}

	// 主要是检查用户操作是否已过期(前端传currRound值)
	if gameRoom.CurrRound != currRound {
		// 操作已过期
		return constant.RoundNotCurrentError
	}

	// 检查当前用户的状态是否符合要求
	joinUser := c.GetJoinUser(ctx, userId, gameRoom.CurrRound)
	if joinUser == nil {
		return constant.GameNotInJoinError
	}

	joinUser.IsAutoBet = isAutoBet
	if errs := c.setJoinUserCache(ctx, gameRoom, joinUser); errs != nil {
		return errs
	}

	// 自动下注延迟队列
	AutoBetDelayFunc(c, gameRoom, joinUser)

	// 发送消息->指定用户
	message := AutoBetMessage{IsAutoBet: isAutoBet}
	c.SendMsgByUserId(context.Background(), gameRoom, userId, message.ToJsonStr(constant.EVENT_AUTO_BETTING))
	return nil
}

// GetJoinUser 获取房间当前用户信息
func (c Game) GetJoinUser(ctx context.Context, userId int64, currRound int) *JoinUser {
	value, err := c.RedisClient.Get(ctx, fmt.Sprintf("join-user:%s-%d-%d", c.GameId, userId, currRound)).Result()
	if err == nil && len(value) > 0 {
		var joinUser *JoinUser
		if errs := json.Unmarshal([]byte(value), &joinUser); errs == nil {
			// TODO 不使用缓存中账户余额,每次实时获取数据库值
			joinUser.AccountBetChips = 0
			return joinUser
		}
	}
	return nil
}

// GetGameRoom 获取房间
func (c Game) GetGameRoom(ctx context.Context) (*GameRoom, error) {
	value, err := c.RedisClient.Get(ctx, fmt.Sprintf("game-room:%s", c.GameId)).Result()
	if err != nil {
		log.Print(err)
		// 获取服务数据内部异常
		return nil, constant.CacheGetInfoError
	}

	// 获取游戏数据不存在
	if len(value) <= 0 {
		return nil, constant.GamePareError
	}

	var gameRoom GameRoom
	err = json.Unmarshal([]byte(value), &gameRoom)
	if err != nil {
		return nil, constant.GamePareError
	}

	return &gameRoom, nil
}

// setGameRoomCache 更新游戏房间信息
func (c Game) setGameRoomCache(ctx context.Context, gameRoom *GameRoom) error {
	gameJson, err := json.Marshal(gameRoom)
	if err != nil {
		return err
	}

	// 更新到缓存
	c.RedisClient.Set(ctx, fmt.Sprintf("game-room:%s", gameRoom.GameId), gameJson, 24*time.Hour)
	return nil
}

// setBatchCache 批量更新缓存
func (c Game) setBatchCache(ctx context.Context, gameRoom *GameRoom, joinUsers map[int64]*JoinUser) error {
	// 更新缓存
	// gameRoom，joinUser
	pipeline := c.RedisClient.TxPipeline()

	gameJson, err := json.Marshal(gameRoom)
	if err != nil {
		return err
	}
	pipeline.Set(ctx, fmt.Sprintf("game-room:%s", gameRoom.GameId), gameJson, 24*time.Hour)

	for userId := range joinUsers {
		joinUser := joinUsers[userId]
		userJson, err := json.Marshal(joinUser)
		if err != nil {
			return err
		}
		pipeline.Set(ctx, fmt.Sprintf("join-user:%s-%d-%d", gameRoom.GameId, userId, gameRoom.CurrRound), userJson, 24*time.Hour)
	}

	// Execute all queued commands in a single round trip
	_, err2 := pipeline.Exec(ctx)
	if err2 != nil {
		return err2
	}

	return nil
}

// GetUserPokerCache 获取用户底牌
func (c Game) GetUserPokerCache(ctx context.Context, gameRoom *GameRoom, userId int64) (*UserPoker, error) {
	value, err := c.RedisClient.Get(ctx, fmt.Sprintf("user-poker:%s-%d-%d", gameRoom.GameId, userId, gameRoom.CurrRound)).Result()
	if err != nil {
		log.Print(err)
		// 获取服务数据内部异常
		return nil, constant.CacheGetInfoError
	}

	// 获取游戏数据不存在
	if len(value) <= 0 {
		return nil, constant.GamePareError
	}

	var userPoker *UserPoker
	err = json.Unmarshal([]byte(value), &userPoker)
	if err != nil {
		return nil, constant.GamePareError
	}

	return userPoker, nil
}

// setPokerBatchCache 批量更新缓存
func (c Game) setPokerBatchCache(ctx context.Context, gameRoom *GameRoom, joinUsers map[int64]*JoinUser, userPokers map[int64]UserPoker) error {
	// 更新缓存
	// gameRoom，joinUser
	pipeline := c.RedisClient.TxPipeline()

	gameJson, err := json.Marshal(gameRoom)
	if err != nil {
		return err
	}
	pipeline.Set(ctx, fmt.Sprintf("game-room:%s", gameRoom.GameId), gameJson, 24*time.Hour)

	for userId := range joinUsers {
		joinUser := joinUsers[userId]
		userJson, errs := json.Marshal(joinUser)
		if errs != nil {
			return errs
		}
		pipeline.Set(ctx, fmt.Sprintf("join-user:%s-%d-%d", gameRoom.GameId, userId, gameRoom.CurrRound), userJson, 24*time.Hour)
	}

	for userId := range userPokers {
		pokerJson, errs := json.Marshal(userPokers[userId])
		if errs != nil {
			return errs
		}
		pipeline.Set(ctx, fmt.Sprintf("user-poker:%s-%d-%d", gameRoom.GameId, userId, gameRoom.CurrRound), pokerJson, 24*time.Hour)
	}

	// Execute all queued commands in a single round trip
	_, err2 := pipeline.Exec(ctx)
	if err2 != nil {
		return err2
	}

	return nil
}

// setJoinUserCache 单个用户信息更新缓存
func (c Game) setJoinUserCache(ctx context.Context, gameRoom *GameRoom, joinUser *JoinUser) error {
	userJson, _ := json.Marshal(joinUser)
	c.RedisClient.Set(ctx, fmt.Sprintf("join-user:%s-%d-%d", gameRoom.GameId, joinUser.UserId, gameRoom.CurrRound), userJson, 24*time.Hour)
	return nil
}
