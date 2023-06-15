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

// CheckAvailability 检查用户是否在当前游戏局中
func (c Game) CheckAvailability(ctx context.Context, userId int64) (*GameRoom, error) {
	gameRoom, err := c.GetGameRoom(ctx)
	if err != nil {
		return nil, err
	}

	// 游戏已结束
	if gameRoom.State == constant.GAME_ENDED {
		return nil, constant.GameEndError
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
func (c Game) GetBroadcastMsg(ctx context.Context, gameRoom *GameRoom, eventMsg *EventMsg) ([]*JoinUser, []byte, error) {

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
		balanceMap, err := c.UserService.GetBalancesByIds(userIds)
		if err == nil && balanceMap != nil && len(balanceMap) > 0 {
			for index := range joinUsers {
				user := joinUsers[index]
				balance, ok := balanceMap[user.UserId]
				if ok {
					user.AccountBetChips = balance
				}
			}
		}
	}

	// 升序
	sort.Slice(joinUsers, func(i, j int) bool { return joinUsers[i].Location < joinUsers[j].Location })

	// 广播json字符串数组对象
	msgJsonByte, err := json.Marshal(&BroadcastMsg{
		Timestamp: time.Now().Unix(),
		Message:   Message{constant.EVENT_JOIN_USER, strings.ReplaceAll(uuid.New().String(), "-", "")},
		Event:     eventMsg,
		Room:      gameRoom,
		Users:     joinUsers,
	})
	return joinUsers, msgJsonByte, err
}

// BroadcastMsg 广播游戏状态->所有在线用户
func (c Game) BroadcastMsg(ctx context.Context, gameRoom *GameRoom, eventMsg *EventMsg) {
	if gameRoom == nil || c.Clients == nil || len(c.Clients) <= 0 {
		return
	}

	// 广播消息
	joinUsers, msgJsonByte, _ := c.GetBroadcastMsg(ctx, gameRoom, eventMsg)
	for userId := range c.Clients {
		clients := c.Clients[userId]
		for token := range clients {
			clients[token].WriteMessage(websocket.TextMessage, msgJsonByte)
		}
	}

	// 游戏中,每次广播遍历用户是否最终赢家
	if gameRoom.State == constant.GAME_PAYING {
		c.CheckGameWinUser(ctx, gameRoom, joinUsers)
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

// CheckGameWinUser 最终赢家用户
func (c Game) CheckGameWinUser(ctx context.Context, gameRoom *GameRoom, joinUsers []*JoinUser) {

	if joinUsers == nil || len(joinUsers) <= 0 {
		return
	}

	winUsers := make([]*JoinUser, 0)
	otherUsers := make([]*JoinUser, 0)
	for index := range joinUsers {
		user := joinUsers[index]

		// 还在游戏中的用户
		if user.State == constant.EVENT_PLAYING_USER {
			winUsers = append(winUsers, user)
		}

		// 其他非等待用户
		if user.State != constant.EVENT_JOIN_USER {
			otherUsers = append(otherUsers, user)
		}
	}

	// 仅剩一个游戏中用户则认为最终游戏获胜者
	if len(winUsers) != 1 {
		return
	}

	// 整体放入同一个事物中
	// 总筹码提现到最终赢家账户-操作数据库
	user := winUsers[0]
	err := c.UserService.UpateWinBetting(gameRoom.GameId, gameRoom.CurrRound, user.UserId, gameRoom.TotalBetChips, func(betChips int64) error {
		user.State = constant.EVENT_WIN_USER
		gameRoom.State = constant.GAME_ENDED

		// 更新游戏状态gameRoom,users
		users := make(map[int64]*JoinUser, 0)
		users[user.UserId] = user
		return c.setBatchCache(ctx, gameRoom, users)
	})

	if err != nil {
		log.Println("Update win game user error", err)
		return
	}

	// 发送获胜者的广播消息
	eventMsg := &EventMsg{
		Type:   constant.EVENT_WIN_USER,
		UserId: user.UserId,
	}
	c.BroadcastMsg(ctx, gameRoom, eventMsg)

	// 下一局开始,获胜者成为新庄家
	if gameRoom.TotalRounds > gameRoom.CurrRound {
		// 延迟2秒执行
		go func() {
			timer := time.NewTimer(2 * time.Second)
			<-timer.C

			// 重置游戏设置
			gameRoom.CurrLocation = 0
			gameRoom.CurrBetChips = 0
			gameRoom.CurrLocation = 0
			gameRoom.State = constant.GAME_WAIT
			gameRoom.CurrBankerId = user.UserId
			gameRoom.CurrRound = gameRoom.CurrRound + 1
			gameRoom.ExposedBetChips = gameRoom.LowBetChips
			gameRoom.ConcealedBetChips = gameRoom.LowBetChips
			gameRoom.JoinUsers = make(map[int64]int, 0)

			callFunc := func(room *GameRoom, joinUser map[int64]*JoinUser) {
				// 将当前获取赢家排第一位
				addTenNum := true
				newUsers := make([]*JoinUser, 0)
				for index := range otherUsers {
					newUser := otherUsers[index]
					if newUser.UserId == user.UserId {
						addTenNum = false
						continue
					}

					// 通过+100将赢家排第一位(前提otherUsers是有序)
					if addTenNum {
						newUser.Location += 100
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
				ID:       user.UserId,
				Nickname: user.Nickname,
				HeadPic:  user.HeadPic,
			}, callFunc)
		}()
	}
}

// SetNextOperateUser 设置下个操作用户
func (c Game) SetNextOperateUser(ctx context.Context, gameRoom *GameRoom, operateLocation int) {

	// 确认是否游戏中
	if gameRoom.State != constant.GAME_PAYING {
		return
	}

	// 校验操作用户与游戏中的位置是否匹配
	if gameRoom.CurrLocation != operateLocation {
		return
	}

	payingUsers := make([]*JoinUser, 0)
	for userId := range gameRoom.JoinUsers {
		if gameRoom.JoinUsers[userId] == gameRoom.CurrRound {
			joinUser := c.GetJoinUser(ctx, userId, gameRoom.CurrRound)
			if joinUser != nil && joinUser.State == constant.EVENT_PLAYING_USER {
				payingUsers = append(payingUsers, joinUser)
			}
		}
	}

	// 玩家必须大于
	if len(payingUsers) <= 1 {
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
	gameRoom.SetLocationTime = time.Now().Unix()
	err := c.setGameRoomCache(context.Background(), gameRoom)
	if err != nil {
		log.Println(err)
		return
	}

	// 延迟1秒执行
	go func() {
		timer := time.NewTimer(1 * time.Second)
		<-timer.C

		operateUser := locationUsers[location]

		// 用户已设置自动跟注
		if operateUser.IsAutoBet {
			// 延迟15秒倒计时->(用户设置自动跟注)
			delayMsg := DelayMsg{
				DelayType: constant.DELAY_AUTOBET,
				GameId:    gameRoom.GameId,
				UserId:    operateUser.UserId,
				CurrRound: gameRoom.CurrRound,
				Timestamp: gameRoom.SetLocationTime,
			}
			c.DelayQueue.SendDelayMsg(delayMsg.ToJsonStr(), 1*time.Second, daley.WithRetryCount(5))
		} else {
			// 延迟15秒倒计时->(超时用户自动放弃)
			delayMsg := DelayMsg{
				DelayType: constant.DELAY_GIVEUP,
				GameId:    gameRoom.GameId,
				UserId:    operateUser.UserId,
				CurrRound: gameRoom.CurrRound,
				Timestamp: gameRoom.SetLocationTime,
			}
			c.DelayQueue.SendDelayMsg(delayMsg.ToJsonStr(), 15*time.Second, daley.WithRetryCount(5))
		}

		// 广播消息通知所有用户
		c.BroadcastMsg(ctx, gameRoom, &EventMsg{Type: constant.EVENT_CURRENT_USER, UserId: operateUser.UserId, Location: operateUser.Location, CountdownSecond: 15})
	}()
}

// CreateGames 创建游戏
func (c Game) CreateGames(gameRoom *GameRoom, user db.User, callFunc func(*GameRoom, map[int64]*JoinUser)) error {
	// 更新游戏房间信息
	c.setGameRoomCache(context.Background(), gameRoom)

	// UserJoinRoom 庄家默认加入游戏
	return c.UserJoinRoom(user, false, callFunc, nil)
}

func (c Game) CheckBetChips(gameRoom *GameRoom, joinUser *JoinUser, checkFunc func(int64) error) (int64, error) {
	var betChips int64
	if joinUser.IsLookCard {
		exposedBetChips := gameRoom.ConcealedBetChips * 2
		if exposedBetChips < gameRoom.ExposedBetChips {
			exposedBetChips = gameRoom.ExposedBetChips
		}

		return

		if exposedBetChips > betChips {

		}
	}

	if checkFunc != nil {
		if err := checkFunc(betChips); err != nil {
			return betChips, err
		}
	}
	return 0, nil
}

// DelayCallback 延迟队列消息
func (c Game) DelayCallback(delayMsg DelayMsg) bool {

	switch delayMsg.DelayType {
	case constant.DELAY_AUTOBET:
		err := c.UserBetting(delayMsg.UserId, 0, delayMsg.CurrRound, 0, func(gameRoom *GameRoom, joinUser *JoinUser) error {
			if gameRoom.CurrLocation != joinUser.Location || gameRoom.SetLocationTime != delayMsg.Timestamp {
				// 延续消息处理过期
				return constant.DelayOperateExpiredError
			}
			return nil
		}, func(gameRoom *GameRoom, joinUser *JoinUser, callUpdateFunc func(bool, *UserPoker) error) error {
			// 整体放入同一个事物中
			// 扣除用户的跟注/加注筹码-操作数据库
			return c.UserService.DeductRaiseBetting(gameRoom.GameId, gameRoom.CurrRound, joinUser.UserId, 0, func(betChips int64) error {
				if joinUser.IsLookCard {
					// 明牌下注筹码
					gameRoom.ExposedBetChips = betChips
				} else {
					// 隐藏下注筹码
					gameRoom.ConcealedBetChips = betChips
				}

				joinUser.TotalBetChips += betChips
				gameRoom.TotalBetChips += betChips
				return callUpdateFunc(false, nil)
			})
		})

		//  下注金额不足取消自动操作
		c.UserSetAutoBetting(delayMsg.UserId, false, delayMsg.CurrRound)

		if err != nil {
			log.Printf("operate delay userId=%d, auto betting error: %s", delayMsg.UserId, err.Error())
		}
		break
	case constant.DELAY_GIVEUP:
		// 超时用户自动放弃
		//err := c.UserGiveUpCard(delayMsg.UserId, delayMsg.CurrRound, func(gameRoom *GameRoom, joinUser *JoinUser) error {
		//	if gameRoom.CurrLocation != joinUser.Location || gameRoom.SetLocationTime != delayMsg.Timestamp {
		//		// 延续消息处理过期
		//		return constant.DelayOperateExpiredError
		//	}
		//	return nil
		//})
		//
		//if err != nil {
		//	log.Printf("operate delay userId=%d, auto give up card error: %s", delayMsg.UserId, err.Error())
		//}
		//break
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
			} else if gameRoom.CurrBankerId == startUserId {
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
		joinUser.IsAutoBet = false
		joinUsers[joinUser.UserId] = joinUser
	}

	// 自动扣除每个用户的底注
	errs := handlerFunc(gameRoom, joinUsers, func(userPokers map[int64]UserPoker) error {
		// 更新缓存 gameRoom，joinUsers
		gameRoom.CurrLocation = 0
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

	// 检查用户是否当前局游戏中
	gameRoom, err := c.CheckAvailability(ctx, loginUser.ID)
	if err != nil && !errors.Is(err, constant.GameNotInJoinError) && !errors.Is(err, constant.RoundError) {
		return err
	}

	// 手动点击游戏开始,当前账号筹码是否足够
	if isReadJoin && handlerFunc != nil {
		if errs := handlerFunc(gameRoom); errs != nil {
			return errs
		}
	}

	// 指定当前用户发现消息
	sendMsgByIdFunc := func() {
		_, msgJsonByte, _ := c.GetBroadcastMsg(ctx, gameRoom, nil)
		c.SendMsgByUserId(ctx, gameRoom, loginUser.ID, msgJsonByte)
	}

	// 当前游戏中
	if gameRoom.State == constant.GAME_PAYING {
		// 当前用户游戏中或者当前用户作为旁观者在房间中
		if err == nil || errors.Is(err, constant.RoundError) {
			if c.Clients != nil && c.Clients[loginUser.ID] != nil && len(c.Clients[loginUser.ID]) > 0 {
				sendMsgByIdFunc()
			}
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
			sendMsgByIdFunc()
			return nil
		}
	} else {
		joinUser = &JoinUser{
			UserId:        loginUser.ID,
			State:         state,
			Nickname:      loginUser.Nickname,
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

		// 更新缓存 joinUser
		if errs := c.setJoinUserCache(ctx, gameRoom, joinUser); errs != nil {
			return errs
		}
	}

	// 发送消息->指定用户
	message := CardMessage{Card: cardStr}
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
		joinUser.State = constant.EVENT_GIVE_UP_USER

		// 更新缓存 joinUser
		err = c.setJoinUserCache(ctx, gameRoom, joinUser)
		if err != nil {
			return err
		}
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

	// 下注并找用户比较大小
	isPkRequest := false
	var compareUser *JoinUser
	if compareId > 0 {
		// PK类型的请求
		isPkRequest = true

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
		eventMsg.Type = constant.EVENT_WIN_USER
		if !pkResult {
			// 设置自己PK失败
			eventMsg.Type = joinUser.State
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
	c.RedisClient.Set(ctx, fmt.Sprintf("game-room:%s", gameRoom.GameId), gameJson, 0)
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
	pipeline.Set(ctx, fmt.Sprintf("game-room:%s", gameRoom.GameId), gameJson, 0)

	for userId := range joinUsers {
		joinUser := joinUsers[userId]
		userJson, err := json.Marshal(joinUser)
		if err != nil {
			return err
		}
		pipeline.Set(ctx, fmt.Sprintf("join-user:%s-%d-%d", gameRoom.GameId, userId, gameRoom.CurrRound), userJson, 0)
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
	pipeline.Set(ctx, fmt.Sprintf("game-room:%s", gameRoom.GameId), gameJson, 0)

	for userId := range joinUsers {
		joinUser := joinUsers[userId]
		userJson, errs := json.Marshal(joinUser)
		if errs != nil {
			return errs
		}
		pipeline.Set(ctx, fmt.Sprintf("join-user:%s-%d-%d", gameRoom.GameId, userId, gameRoom.CurrRound), userJson, 0)
	}

	for userId := range userPokers {
		pokerJson, errs := json.Marshal(userPokers[userId])
		if errs != nil {
			return errs
		}
		pipeline.Set(ctx, fmt.Sprintf("user-poker:%s-%d-%d", gameRoom.GameId, userId, gameRoom.CurrRound), pokerJson, 0)
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
	c.RedisClient.Set(ctx, fmt.Sprintf("join-user:%s-%d-%d", gameRoom.GameId, joinUser.UserId, gameRoom.CurrRound), userJson, 0)
	return nil
}
