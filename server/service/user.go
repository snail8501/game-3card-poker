package service

import (
	"errors"
	"game-3-card-poker/server/constant"
	"game-3-card-poker/server/db"
	"gorm.io/gorm"
	"sync"
	"time"
)

// LoginBody represents the JSON body received by the endpoint.
type LoginBody struct {
	Email    string `json:"email" valid:"required"`
	Password string `json:"password" valid:"required"`
}

type UserService struct {
	userDB *db.UserDB
	mux    sync.Mutex
}

func NewUserService(userDB *db.UserDB) *UserService {
	return &UserService{userDB: userDB}
}

type RegisterReq struct {
	Email      string `json:"email" valid:"required"`
	Password   string `json:"password" valid:"required"`
	Nickname   string `json:"nickname" valid:"required"`
	VerifyCode string `json:"verifyCode" valid:"required"`
}

type SendCodeReq struct {
	Email string `json:"email" valid:"required"`
}

func (u *UserService) Register(req RegisterReq) (db.User, error) {
	u.mux.Lock()
	defer u.mux.Unlock()

	err := u.CheckUser(req)
	if err != nil {
		return db.User{}, err
	}

	return u.userDB.CreateUser(db.User{
		Email:    req.Email,
		Password: req.Password,
		Nickname: req.Nickname,
		Balance:  0,
	})
}

func (u *UserService) CheckUser(req RegisterReq) error {
	u.mux.Lock()
	defer u.mux.Unlock()

	dbUser, err := u.userDB.QueryByEmail(req.Email)
	if err != nil {
		return err
	}

	if dbUser != (db.User{}) {
		return errors.New(constant.EmailExist)
	}

	if len(req.Nickname) != 0 {
		dbUser, err = u.userDB.QueryByNickname(req.Nickname)
		if err != nil {
			return err
		}

		if dbUser != (db.User{}) {
			return errors.New(constant.NicknameExist)
		}
	}
	return nil
}

func (u *UserService) GetByEmail(email string) (db.User, error) {
	return u.userDB.QueryByEmail(email)
}

func (u *UserService) ChangePassword(body db.User) (db.User, error) {
	user := db.User{
		ID:         body.ID,
		Email:      body.Email,
		Password:   body.Password,
		UpdateTime: time.Now(),
	}
	return u.userDB.Update(user)
}

func (u *UserService) Logout(user db.User) (db.User, error) {
	return u.userDB.Update(user)
}

func (u *UserService) GetById(userId int64) (db.User, error) {
	return u.userDB.QueryById(userId)
}

func (u *UserService) GetBalancesByIds(userIds []int64) (map[int64]int64, error) {
	users, err := u.userDB.GetListByUserIds(userIds)
	if err != nil {
		return nil, err
	}

	balances := make(map[int64]int64, 0)
	for index := range users {
		user := users[index]
		balances[user.ID] = user.Balance
	}
	return balances, nil
}

func (u *UserService) DeductAnteBetting(gameId string, currRound int, userIds []int64, betChips int64, callUpdateFunc func(map[int64]int64) error) error {

	users, err := u.userDB.GetListByUserIds(userIds)
	if err != nil {
		return err
	}

	return u.userDB.BettingTransaction(func(tx *gorm.DB) error {

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
				GameId:        gameId,
				RoundID:       currRound,
				Status:        constant.BET_STATE_ANTE,
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

	return u.userDB.BettingTransaction(func(tx *gorm.DB) error {

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
			GameId:        gameId,
			RoundID:       currRound,
			Status:        constant.BET_STATE_RAISE,
			Amount:        betChips,
			BalanceBefore: user.Balance,
		}
		if errs := tx.Model(&db.UserHistory{}).Create(&history).Error; errs != nil {
			return errs
		}
		return callUpdateFunc(betChips)
	})
}

func (u *UserService) UpateWinBetting(gameId string, currRound int, userId int64, totalBetChips int64, callUpdateFunc func(int64) error) error {

	user, err := u.userDB.QueryById(userId)
	if err != nil {
		return err
	}

	return u.userDB.BettingTransaction(func(tx *gorm.DB) error {

		userBalances := user.Balance
		if userBalances < 0 {
			userBalances = 0
		}

		// 实际用户下注筹码
		var countBetChips int64
		if errs := tx.Model(&db.User{}).Where("game_id = ? and round_id = ? and state IN (?)", gameId, currRound, []int{constant.BET_STATE_ANTE, constant.BET_STATE_RAISE}).Count(&countBetChips).Error; errs != nil {
			return errs
		}

		if totalBetChips > countBetChips {
			totalBetChips = countBetChips
		}

		userBalances += totalBetChips
		if errs := tx.Model(&db.User{}).Where("id = ?", user.ID).UpdateColumn("balance", userBalances).Error; errs != nil {
			return errs
		}

		// record user history
		history := db.UserHistory{
			UserId:        user.ID,
			GameId:        gameId,
			RoundID:       currRound,
			Status:        constant.BET_STATE_WIN,
			Amount:        totalBetChips,
			BalanceBefore: user.Balance,
		}
		if errs := tx.Model(&db.UserHistory{}).Create(&history).Error; errs != nil {
			return errs
		}
		return callUpdateFunc(totalBetChips)
	})
}
