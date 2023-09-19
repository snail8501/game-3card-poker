package service

import (
	"game-3-card-poker/server/constant"
	"game-3-card-poker/server/db"
	"gorm.io/gorm"
	"sort"
	"sync"
	"time"
)

type UserService struct {
	userDB *db.UserDB
	mux    sync.Mutex
}

func NewUserService(userDB *db.UserDB) *UserService {
	return &UserService{userDB: userDB}
}

type HistoryRecord struct {
	UserId  int64  `json:"userId"`
	Address string `json:"address"`
	Amount  int64  `json:"amount"`
}

type ReceiveReq struct {
	Receive bool `json:"receive"`
}

type Verify struct {
	Address   string `json:"address" valid:"required"`
	Signature string `json:"signature" valid:"required"`
}

type UpdateHeadPic struct {
	HeadPic string `json:"headPic" valid:"required"`
}

type RequestHistory struct {
	GameId string `json:"gameId" valid:"required"`
}

type UserReq struct {
	Address string `json:"address" valid:"required"`
}

func (u *UserService) GetById(userId int64) (db.User, error) {
	return u.userDB.QueryById(userId)
}

func (u *UserService) GetByAddress(address string) (db.User, error) {
	return u.userDB.GetByAddress(address)
}

func (u *UserService) GetUsersByIds(userIds []int64) (map[int64]db.User, error) {
	users, err := u.userDB.GetListByUserIds(userIds)
	if err != nil {
		return nil, err
	}

	userMap := make(map[int64]db.User, 0)
	for index := range users {
		user := users[index]
		userMap[user.ID] = *user
	}
	return userMap, nil
}

func (u *UserService) DeductAnteBetting(gameId string, currRound int, userIds []int64, betChips int64, callUpdateFunc func(map[int64]int64) error) error {

	users, err := u.userDB.GetListByUserIds(userIds)
	if err != nil {
		return err
	}

	return u.userDB.Transaction(func(tx *gorm.DB) error {

		balanceMap := make(map[int64]int64, 0)
		for index := range users {
			user := users[index]

			userBetChips := int64(0)
			userBalances := int64(0)
			if user.Balance < 0 {
				userBalances = 0
				userBetChips = 0
			} else if user.Balance < betChips {
				userBalances = 0
				userBetChips = user.Balance
			} else {
				userBalances = user.Balance - betChips
				userBetChips = betChips
			}

			// 实际用户下注筹码
			balanceMap[user.ID] = userBetChips

			if errs := tx.Model(&db.User{}).Where("id = ?", user.ID).UpdateColumn("balance", userBalances).Error; errs != nil {
				return errs
			}

			// record user history
			history := db.UserHistory{
				UserId:        user.ID,
				Address:       user.Address,
				GameId:        gameId,
				RoundID:       currRound,
				State:         constant.BET_STATE_ANTE,
				Amount:        userBetChips,
				BalanceBefore: user.Balance,
			}
			if errs := tx.Model(&db.UserHistory{}).Create(&history).Error; errs != nil {
				return errs
			}
		}
		return callUpdateFunc(balanceMap)
	})
}

func (u *UserService) DeductRaiseBetting(gameId string, currRound int, userId int64, betChips int64, callUpdateFunc func(int64) error) error {

	user, err := u.userDB.QueryById(userId)
	if err != nil {
		return err
	}

	// 用户筹码不足
	if user.Balance < betChips {
		return constant.UserNotEnoughBetError
	}

	return u.userDB.Transaction(func(tx *gorm.DB) error {

		userBalances := user.Balance - betChips
		if userBalances < 0 {
			betChips = user.Balance
			userBalances = 0
		}

		if errs := tx.Model(&db.User{}).Where("id = ?", user.ID).UpdateColumn("balance", userBalances).Error; errs != nil {
			return errs
		}

		// record user history
		history := db.UserHistory{
			UserId:        user.ID,
			Address:       user.Address,
			GameId:        gameId,
			RoundID:       currRound,
			State:         constant.BET_STATE_RAISE,
			Amount:        betChips,
			BalanceBefore: user.Balance,
		}
		if errs := tx.Model(&db.UserHistory{}).Create(&history).Error; errs != nil {
			return errs
		}
		return callUpdateFunc(betChips)
	})
}

func (u *UserService) UpateWinBetting(gameId string, currRound int, userId int64, totalBetChips int64, callUpdateFunc func(int64) error) (records []HistoryRecord, err error) {

	user, err := u.userDB.QueryById(userId)
	if err != nil {
		return records, err
	}

	return records, u.userDB.Transaction(func(tx *gorm.DB) error {
		userBalances := user.Balance
		if userBalances < 0 {
			userBalances = 0
		}

		// 实际用户下注筹码
		historys := make([]HistoryRecord, 0)
		states := []int{constant.BET_STATE_ANTE, constant.BET_STATE_RAISE}
		if errs := tx.Model(&db.UserHistory{}).
			Where("game_id = ? and round_id = ? and state IN (?)", gameId, currRound, states).
			Group("user_id").
			Select("SUM(amount) as amount, user_id, address").Scan(&historys).Error; errs != nil {
			return errs
		}

		if len(historys) > 0 {
			countBetChips := int64(0)
			records = make([]HistoryRecord, 0)
			for index := range historys {
				record := historys[index]
				countBetChips += record.Amount

				if record.UserId != user.ID {
					records = append(records, HistoryRecord{
						UserId:  record.UserId,
						Address: record.Address,
						Amount:  -record.Amount,
					})
				}
			}

			if totalBetChips > countBetChips {
				totalBetChips = countBetChips
			}

			records = append(records, HistoryRecord{
				UserId:  user.ID,
				Address: user.Address,
				Amount:  totalBetChips,
			})

			// 降序
			sort.Slice(records, func(i, j int) bool {
				return records[i].Amount > records[j].Amount
			})
		}

		userBalances += totalBetChips
		if errs := tx.Model(&db.User{}).Where("id = ?", user.ID).UpdateColumn("balance", userBalances).Error; errs != nil {
			return errs
		}

		// record user history
		history := db.UserHistory{
			UserId:        user.ID,
			Address:       user.Address,
			GameId:        gameId,
			RoundID:       currRound,
			State:         constant.BET_STATE_WIN,
			Amount:        totalBetChips,
			BalanceBefore: user.Balance,
		}

		if errs := tx.Model(&db.UserHistory{}).Create(&history).Error; errs != nil {
			return errs
		}
		return callUpdateFunc(totalBetChips)
	})
}

func (u *UserService) GetHisotryRecordList(gameId string) []HistoryRecord {
	historys := make([]HistoryRecord, 0)
	u.userDB.Transaction(func(tx *gorm.DB) error {
		if errs := tx.Model(&db.UserHistory{}).
			Where("game_id = ?", gameId).
			Group("user_id").
			Order("amount desc").
			Select("user_id, address, SUM(CASE WHEN state = 2 THEN amount ELSE -amount END) as amount").
			Scan(&historys).Error; errs != nil {
			return errs
		}
		return nil
	})
	return historys
}

func (u *UserService) UpdateRecord(id int64, record string) error {
	user := db.User{
		ID: id,
		//Record: record,
	}
	_, err := u.userDB.Update(user)
	return err
}

func (u *UserService) ReceiveCoin(coinCount int64, user db.User) error {
	user.Balance += coinCount
	_, err := u.userDB.Update(user)
	return err
}

func (u *UserService) SignatureVerify(address string, defaultBalance int64, randHeadPic string) (db.User, error) {
	user, err := u.userDB.GetByAddress(address)
	if err != nil {
		return db.User{}, err
	}

	if user == (db.User{}) {
		user, err := u.userDB.CreateUser(db.User{
			Address:  address,
			HeadPic:  randHeadPic,
			Balance:  defaultBalance,
			CreateAt: time.Now(),
			UpdateAt: time.Now(),
		})
		if err != nil {
			return db.User{}, err
		}
		return user, nil
	}
	return user, nil
}

func (u *UserService) UpdateHeadPic(user db.User) error {
	return u.userDB.UpdateHeadPic(user)
}
