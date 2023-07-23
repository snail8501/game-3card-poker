package constant

const (
	HeaderCustomUser    = "X-Custom-User"
	HeaderCustomToken   = "X-Custom-Token"
	HeaderCurrentGameId = "X-Current-GameId"
)

// 游戏状态
const (
	GAME_WAIT   = iota + 0 // 0、等待游戏
	GAME_PAYING            // 1、游戏中
	GAME_ENDED             // 2、游戏结束
)

// 延迟消息类型
const (
	DELAY_AUTOBET = iota + 1 // 1、用户设置自动跟注
	DELAY_GIVEUP             // 2、超时用户自动放弃
)

// 游戏请求事件类型
const (
	POKER_READY     = iota + 0 // 0、准备游戏
	POKER_START                // 1、开始游戏->仅庄家操作
	POKER_LOOK_CARD            // 2、看牌
	POKER_GIVE_UP              // 3、弃牌
	POKER_BET                  // 4、跟注/加注
	POKER_COMPARE              // 5、下注比牌
	POKER_AUTOBET              // 6、自动下注
)

// 游戏响应事件类型
const (
	EVENT_JOIN_USER         = iota + 0 // 0、加入游戏(如果消息通知，则表示广播)->用户状态
	EVENT_READY_USER                   // 1、准备游戏->用户状态
	EVENT_PLAYING_USER                 // 2、游戏中->用户状态
	EVENT_GIVE_UP_USER                 // 3、弃牌->用户状态
	EVENT_COMPARE_LOSE_USER            // 4、PK结果消息(PK输家->用户状态)
	EVENT_WIN_USER                     // 5、游戏结束消息(最终赢家->用户状态)
	EVENT_LOOK_CARD                    // 6、看牌
	EVENT_BET_CHIPS                    // 7、跟注/加注
	EVENT_AUTO_BETTING                 // 8、用户设置自动下注
	EVENT_CURRENT_USER                 // 9、当前活动用户
	EVENT_ERROR                        // 10、错误请求
	EVENT_OVER                         // 11、游戏结束
)

// 筹码历史记录状态
const (
	BET_STATE_ANTE  = iota + 0 // 1、底注
	BET_STATE_RAISE            // 2、下注
	BET_STATE_WIN              // 3、获胜
)

// 合约相关常量
const (
	MissTransaction      = "Something went wrong: Missing transaction for ID"
	MissTransactionError = "miss transaction"
	TransactionBaseUrl   = "https://vm.aleo.org/api/testnet3/transaction/"
	AppName              = "game_3_card_poker"
	SavePokerName        = "save_poker"
	SaveRoundName        = "save_round"
	AddUserBalanceName   = "add_user_balance"
	SubUserBalanceName   = "sub_user_balance"
	AdminAddress         = "aleo1l44dfmwcu7j2e26yhrlhrxla4lsrmr0rxymxfzxj6h8m2mnegyqs8x0end"
	BlockHeight          = "block-height"
	GetHeightUrl         = "https://vm.aleo.org/api/testnet3/latest/height"
	GetBlockByHeightUrl  = "https://vm.aleo.org/api/testnet3/block/"
	InvalidViewKey       = "Invalid view key for the provided record ciphertext"
	Owner                = "owner"
	Microcredits         = "microcredits"
	Nonce                = "_nonce"
)

// 响应码
const (
	Code10000 = 10000 // OK
	Code10001 = 10001 // 参数异常
	Code10002 = 10002 // 邮箱已存在
	Code10003 = 10003 // 验证码已发送
	Code10004 = 10004 // 验证码错误
	Code10005 = 10005 // 昵称重复
	Code10006 = 10006 // 两次密码不一致
	Code10007 = 10007 // 登录失败
	Code10008 = 10008 // 底注必须比顶注低
	Code10009 = 10009 // 领取超过3次
	Code10010 = 10010 // 用户不存在
	Code10011 = 10011 // 验证码发送异常
	Code10012 = 10012 // 用户未登录
	Code10013 = 10013 // 金币大于1000不允许领取
	Code20001 = 20001 // 游戏链接不存在
	Code99999 = 99999 // 系统异常
)

// 错误信息
const (
	OK                  = "OK"
	ParamError          = "参数异常"
	EmailExist          = "邮箱已存在"
	VerifyCodeExist     = "验证码已发送"
	VerifyCodeError     = "验证码错误"
	NicknameExist       = "昵称重复"
	PasswordNotSame     = "两次密码不一致"
	LoginFailed         = "登录失败，请检查凭证"
	LowBetMustLowTopBet = "底注必须比顶注低"
	PushThanThree       = "领取超过3次"
	UserNotExist        = "用户不存在"
	VerifyCodeSendError = "验证码发送异常"
	UserNotLogin        = "用户未登录"
	BalanceThan1000     = "金币大于1000不允许领取"
	GameNotExist        = "游戏链接不存在"
	Error               = "系统异常"
)
