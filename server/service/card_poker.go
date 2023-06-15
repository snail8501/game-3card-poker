package service

import (
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	PokerTriple        = 6 // 豹子（AAA最大，222最小）
	PokerFlushStraight = 5 // 同花顺（AKQ最大，A23最小）
	PokerFlush         = 4 // 同花（AKJ最大，352最小）
	PokerStraight      = 3 // 顺子（AKQ最大，A23最小）
	PokerDouble        = 2 // 对子（AAK最大，223最小）
	PokerSingle        = 1 // 单张（AKJ最大，352最小）
)

type Poker struct {
	Value int `json:"value"` // 牌 2-3-4-5-6-7-8-9-10-11-12-13-14
	Color int `json:"color"` // 花色 黑桃4，红桃3，梅花2，方块1
}

type CardPoker struct {
	PokerList []Poker // 52张扑克牌
}

// InitShufflePoker 初始化52张牌，规则：点数*10+花色，并洗牌
func (c *CardPoker) InitShufflePoker() {
	// 初始化
	c.PokerList = make([]Poker, 0)
	for value := 2; value < 15; value++ {
		for color := 1; color < 5; color++ {
			c.PokerList = append(c.PokerList, Poker{Value: value, Color: color})
		}
	}

	// 洗牌
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(c.PokerList), func(i, j int) {
		// 随机交换数组元素的位置
		c.PokerList[i], c.PokerList[j] = c.PokerList[j], c.PokerList[i]
	})
}

// CutCardPoker 切牌
func (c *CardPoker) CutCardPoker() {
	rand.Seed(time.Now().UnixNano())

	// 生成随机数切牌，随机数 1—51，最少切一张牌，最多切51张牌
	index := rand.Intn(51) + 1
	arr := make([]Poker, 0)
	arr = append(arr, c.PokerList[index:]...)
	arr = append(arr, c.PokerList[:index]...)
	c.PokerList = arr
}

// LicenseCardPoker 发牌
func (c *CardPoker) LicenseCardPoker(userIds []int64) map[int64]UserPoker {

	index := 0
	userIdPokerArry := make(map[int64][]Poker, 0)
	for i := 0; i < 3; i++ {
		for _, userId := range userIds {
			poker := c.PokerList[index]
			userIdPokerArry[userId] = append(userIdPokerArry[userId], poker)

			index++
		}
	}

	userPokerArry := make(map[int64]UserPoker, 0)
	for userId := range userIdPokerArry {
		pokers := userIdPokerArry[userId]
		userPokerArry[userId] = UserPoker{OnePoker: pokers[0], TowPoker: pokers[1], ThreePoker: pokers[2]}
	}
	return userPokerArry
}

type UserPoker struct {
	OnePoker   Poker `json:"onePoker"`
	TowPoker   Poker `json:"towPoker"`
	ThreePoker Poker `json:"threePoker"`
}

// ToString 转换成字符串逗号分割
func (u UserPoker) ToString() string {
	card := make([]string, 0)
	card = append(card, strconv.Itoa(u.OnePoker.Value*10+u.OnePoker.Color))
	card = append(card, strconv.Itoa(u.TowPoker.Value*10+u.TowPoker.Color))
	card = append(card, strconv.Itoa(u.ThreePoker.Value*10+u.ThreePoker.Color))
	return strings.Join(card, ", ")
}

// UserPokerPK 牌型大小比较
// 流程： 1.先判断用户牌型，如果牌型大者直接获胜  2.牌型一样进行点数和花色比较
func (u UserPoker) UserPokerPK(pkUser *UserPoker) bool {

	// 获取两个用户的牌型
	pokerType1 := u.getPokerType()
	pokerType2 := pkUser.getPokerType()

	samePkFunc := func() bool {
		// 得到user1和user2的三张牌，规则：点数 * 10 + 花色
		card := make([]int, 3)
		card = append(card, u.OnePoker.Value*10+u.OnePoker.Color)
		card = append(card, u.TowPoker.Value*10+u.TowPoker.Color)
		card = append(card, u.ThreePoker.Value*10+u.ThreePoker.Color)

		pkCard := make([]int, 3)
		pkCard = append(pkCard, pkUser.OnePoker.Value*10+pkUser.OnePoker.Color)
		pkCard = append(pkCard, pkUser.TowPoker.Value*10+pkUser.TowPoker.Color)
		pkCard = append(pkCard, pkUser.ThreePoker.Value*10+pkUser.ThreePoker.Color)

		// 从大到小排序
		sort.Sort(sort.Reverse(sort.IntSlice(card)))
		sort.Sort(sort.Reverse(sort.IntSlice(pkCard)))

		// card->特殊情况处理：A23顺子是最小的顺子
		if u.isStraight() && card[0]/10 == 14 && card[1]/10 == 3 && card[2]/10 == 2 {
			card[0] = 1*10 + card[0]%10
		}

		// pkCard->特殊情况处理：A23顺子是最小的顺子
		if pkUser.isStraight() && pkCard[0]/10 == 14 && pkCard[1]/10 == 3 && pkCard[2]/10 == 2 {
			pkCard[0] = 1*10 + pkCard[0]%10
		}

		isPkSuccess := false

		// 通过点数是否已经完成比较
		isFinish := false
		for i := 0; i < 3; i++ {
			// 循环从大到小依次比较，如果当前点数相同则比较下一张，如果其中一张牌大，则用户直接获胜
			if card[i]/10 == pkCard[i]/10 {
				continue
			} else if card[i]/10 > pkCard[i]/10 {
				isPkSuccess = true
				isFinish = true
				break
			} else {
				isPkSuccess = false
				isFinish = true
				break
			}
		}

		// 通过点数已完成比较直接返回赢家，否则进行花色比较
		if isFinish {
			return isPkSuccess
		}

		// user1和user2牌值一样，进行花色比较，因为不存在两张点数和花色一样的牌，所以只需要比较最大一张的花色即可
		if card[0]%10 > pkCard[0]%10 {
			isPkSuccess = true
		} else {
			isPkSuccess = false
		}
		return isPkSuccess
	}

	// 牌型进行比较，如果user1牌型大于user2，则直接判断用户1赢，若两人牌型一样，则进行点数和花色比较
	if pokerType1 == pokerType2 {
		return samePkFunc()
	} else if pokerType1 > pokerType2 {
		return true
	} else {
		return false
	}
}

// getPokerType 判断牌型，从大到小：6-豹子，5-同花顺，4-同花，3-顺子，2-对子，1-散牌
func (u UserPoker) getPokerType() int {

	// 豹子（AAA最大，222最小）
	if u.isTriple() {
		return PokerTriple
	}

	// 同花顺或同花
	if u.isFlush() {
		if u.isStraight() {
			// 同花顺（AKQ最大，A23最小）
			return PokerFlushStraight
		}

		// 同花（AKJ最大，352最小）
		return PokerFlush
	}

	// 顺子（AKQ最大，A23最小）
	if u.isStraight() {
		return PokerStraight
	}

	// 对子（AAK最大，223最小）
	if u.isDouble() {
		return PokerDouble
	}

	// 单张（AKJ最大，352最小）
	return PokerSingle
}

// isFlush 判断是否同花
func (u UserPoker) isFlush() bool {
	return u.OnePoker.Color == u.TowPoker.Color && u.TowPoker.Color == u.ThreePoker.Color
}

// isStraight 是否顺子
func (u UserPoker) isStraight() bool {
	card := make([]int, 3)
	card = append(card, u.OnePoker.Value)
	card = append(card, u.TowPoker.Value)
	card = append(card, u.ThreePoker.Value)

	// 三张牌从大到小排序，依次判断是否是顺子
	sort.Sort(sort.Reverse(sort.IntSlice(card)))
	flag1 := card[0] == card[1]+1 && card[1] == card[2]+1
	// A23特殊顺子情况处理
	flag2 := card[0] == 14 && card[1] == 3 && card[2] == 2
	return flag1 || flag2
}

// isDouble 是否对子
func (u UserPoker) isDouble() bool {
	return u.OnePoker.Value == u.TowPoker.Value || u.OnePoker.Value == u.ThreePoker.Value || u.TowPoker.Value == u.ThreePoker.Value
}

// isTriple 是否豹子
func (u UserPoker) isTriple() bool {
	return u.OnePoker.Value == u.TowPoker.Value && u.TowPoker.Value == u.ThreePoker.Value
}
