package service

import (
	"encoding/json"
	"github.com/google/uuid"
	"strings"
	"time"
)

const (
	CountdownSecond = 60
	AnimationSecond = 6
)

type DelayMsg struct {
	GameId    string `json:"gameId"`    // 游戏ID
	DelayType int    `json:"delayType"` // 延迟类型
	UserId    int64  `json:"userId"`    // 用户ID
	CurrRound int    `json:"currRound"` // 当前第几局
	Timestamp int64  `json:"timestamp"` // 当前时间戳
	BetChips  int64  `json:"betChips"`  // 下注金额
}

func (d *DelayMsg) ToJsonStr() string {
	marshal, _ := json.Marshal(d)
	return string(marshal)
}

func (d *DelayMsg) ToDelayMsg(jsonStr string) error {
	return json.Unmarshal([]byte(jsonStr), &d)
}

type JoinUser struct {
	UserId          int64  `json:"userId"`          // 用户ID
	State           int    `json:"state"`           // 用户状态
	Address         string `json:"address"`         // 钱包地址
	HeadPic         string `json:"headPic"`         // 用户头像
	IsBanker        bool   `json:"isBanker"`        // 是否庄家
	IsLookCard      bool   `json:"isLookCard"`      // 是否看牌
	IsAutoBet       bool   `json:"isAutoBet"`       // 是否自动跟注
	Location        int    `json:"location"`        // 当前位置
	TotalBetChips   int64  `json:"totalBetChips"`   // 总投注筹码
	AccountBetChips int64  `json:"accountBetChips"` // 账号余额
}

type GameRoom struct {
	GameId            string            `json:"gameId"`            // 游戏ID
	JoinUsers         map[int64]int     `json:"joinUsers"`         // 加入用户ID
	Minimum           int               `json:"minimum"`           // 最低人数
	State             int               `json:"state"`             // 游戏状态
	TotalRounds       int               `json:"totalRounds"`       // 总游戏局数
	CurrRound         int               `json:"currRound"`         // 当前第几局
	CurrLocation      int               `json:"currLocation"`      // 当前操作用户
	CurrTimeStamp     int64             `json:"currTimeStamp"`     // 当前操作开始时间戳
	CurrBetChips      int64             `json:"currBetChips"`      // 当前下注筹码
	CurrBankerId      int64             `json:"currBankerId"`      // 当前庄家ID
	TotalBetChips     int64             `json:"totalBetChips"`     // 总下注筹码
	LowBetChips       int64             `json:"lowBetChips"`       // 最低下注筹码
	TopBetChips       int64             `json:"topBetChips"`       // 封顶下注筹码
	ExposedBetChips   int64             `json:"exposedBetChips"`   // 明牌下注筹码
	ConcealedBetChips int64             `json:"concealedBetChips"` // 隐藏下注筹码
	SetLocationTime   int64             `json:"setLocationTime"`   // 设置操作用户时间戳
	Records           map[int64][]int64 `json:"records"`           // PK记录
	BetChips          []int64           `json:"betChips"`          // 下注筹码记录
	CreateUser        int64             `json:"createUser"`        // 创建用户
	CreateAt          time.Time         `json:"createAt"`          // 创建时间
}

type Message struct {
	MsgType int    `json:"msgType"` // 消息类型
	MsgId   string `json:"msgId"`   // 消息id,uuid全局唯一标识
}

func (m *Message) ToJsonStr(msgType int) []byte {
	m.MsgId = strings.ReplaceAll(uuid.New().String(), "-", "")
	m.MsgType = msgType
	marshal, _ := json.Marshal(m)
	return marshal
}

type ErrorMessage struct {
	Message
	ErrorMsg string `json:"message"` // 错误消息内容
}

func (d *ErrorMessage) ToJsonStr(msgType int) []byte {
	d.Message = Message{MsgType: msgType, MsgId: strings.ReplaceAll(uuid.New().String(), "-", "")}
	marshal, _ := json.Marshal(d)
	return marshal
}

type CardMessage struct {
	Message
	Card     string `json:"card"`               // 用户底牌内容
	BetChips int64  `json:"betChips,omitempty"` // 下注筹码
}

func (d *CardMessage) ToJsonStr(msgType int) []byte {
	d.Message = Message{MsgType: msgType, MsgId: strings.ReplaceAll(uuid.New().String(), "-", "")}
	marshal, _ := json.Marshal(d)
	return marshal
}

type CardEndMessage struct {
	Message
	CardList map[int64]string `json:"cards"` // 用户底牌内容
}

func (d *CardEndMessage) ToJsonStr(msgType int) []byte {
	d.Message = Message{MsgType: msgType, MsgId: strings.ReplaceAll(uuid.New().String(), "-", "")}
	marshal, _ := json.Marshal(d)
	return marshal
}

type AutoBetMessage struct {
	Message
	IsAutoBet bool `json:"isAutoBet"` // 是否配置自动下注
}

func (a *AutoBetMessage) ToJsonStr(msgType int) []byte {
	a.Message = Message{MsgType: msgType, MsgId: strings.ReplaceAll(uuid.New().String(), "-", "")}
	marshal, _ := json.Marshal(a)
	return marshal
}

type EventMsg struct {
	Type            int             `json:"type"`                      // 事件类型
	UserId          int64           `json:"userId,omitempty"`          // 事件用户
	WinUserId       int64           `json:"WinUserId,omitempty"`       // PK赢家用户ID
	CompareId       int64           `json:"compareId,omitempty"`       // PK目标用户
	BetChips        int64           `json:"betChips,omitempty"`        // 下注筹码
	Location        int             `json:"location,omitempty"`        // 当前操作用户
	TotalSecond     int64           `json:"totalSecond,omitempty"`     // 总计->倒计时秒
	CountdownSecond int64           `json:"countdownSecond,omitempty"` // 剩余->倒计时秒
	AnimationSecond int64           `json:"animationSecond,omitempty"` // 动画(pk效果,最终赢家效果)->倒计时秒
	MyselfCard      string          `json:"myselfCard,omitempty"`      // 用户底牌内容
	IsGameOver      bool            `json:"isGameOver,omitempty"`      // 是否游戏结束
	ListBetChips    []int64         `json:"listBetChips,omitempty"`    // 加注筹码列表值
	Records         []HistoryRecord `json:"records,omitempty"`         // 获取记录
}

type BroadcastMsg struct {
	Message
	Room      *GameRoom   `json:"room"`            // 游戏房间
	Users     []*JoinUser `json:"users"`           // 游戏加入用户列表
	Event     *EventMsg   `json:"event,omitempty"` // 游戏操作事件类型
	BetChips  []int64     `json:"betChips"`        // 下注筹码记录
	Timestamp int64       `json:"timestamp"`       //时间戳
}

type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}
