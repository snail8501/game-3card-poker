package constant

import "errors"

var (
	RoundError = errors.New("不在当前游戏中-或者加入旁观者")

	RoundNotCurrentError = errors.New("操作已过期,请刷新")

	GameNotOperateError = errors.New("游戏状态不能操作-弃牌或者PK失败")

	CacheGetInfoError = errors.New("获取服务数据内部异常")

	GamePareError = errors.New("游戏对象数据错误")

	GameEndError = errors.New("游戏已结束")

	GameNotAuthorityStartError = errors.New("你不是庄家不能开始游戏")

	GameStartNotReachedNumberError = errors.New("游戏开始未达到人数")

	GamePayingError = errors.New("游戏已开始")

	GameNotInJoinError = errors.New("用还未加入游戏")

	GamePayingJoinError = errors.New("游戏已开始不能加入")

	GamePkUserInvalidError = errors.New("游戏选择PK用户无效")

	GamePkUserMySelfError = errors.New("游戏选择PK用户不能是自己")

	GameRaisBetNotEnoughError = errors.New("下注筹码不能低于前者")

	NotCurrentOperateError = errors.New("非当前操作用户,请等待")

	UserNotExistError = errors.New("用户不能存在")

	UserNotEnoughBetError = errors.New("用户筹码不足")

	UserSetAutoBettingError = errors.New("用户设置自动下注操作")

	DelayOperateExpiredError = errors.New("延续消息处理过期")
)
