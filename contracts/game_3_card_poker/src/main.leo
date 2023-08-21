// The 'game_3_card_poker' program.
program game_3_card_poker.aleo {
    transition main(public a: u32, b: u32) -> u32 {
        let c: u32 = a + b;
        return c;
    }

    struct Round {
        game_id: field,
        round_id: u64,
        winner: u64,
        amount: u64
    }

    record Poker {
        owner: address,
        gates: u64,
        game_id: field,
        round_id: u64,
        user_id: u64,
        poker: field
    }

    // 保存游戏创建数据
    record Game {
        owner: address,
        gates: u64,
        game_id: field,
        creator: address
    }

    // 保存每局每个人结果
    record Result {
        owner: address,
        gates: u64,
        game_id: field,
        round_id: u64,
        user: address,
        // 0:loser 1:winner
        status: u64,
        amount: u64
    }

    // 保存用户手牌，key为gameID + roundID + userID，value为poker
    mapping pokerMap: field => field;
    // 保存每一局信息，key为gameID + roundID，value为赢家
    mapping recordMap: field => Round;
    // 保存用户余额
    mapping userMap: address => u64;


    transition save_poker (
        public game_id: field,
        public round_id: u64,
        public user_id: u64,
        public poker: field,
        public map_key: field,
    ) -> Poker {

        let p : Poker = Poker {
            owner: self.caller,
            gates: 0u64,
            game_id: game_id,
            round_id: round_id,
            user_id: user_id,
            poker: poker
        };

        return p then finalize(map_key, poker);

    }

    finalize save_poker (public key: field, public poker: field) {
        Mapping::set(pokerMap, key, poker);
    }


    transition save_round (
        public game_id: field,
        public round_id: u64,
        public amount: u64,
        public winner: u64,
        public map_key : field,
    ) {
        return then finalize(game_id, round_id, amount, winner, map_key);
    }

    finalize save_round ( public game_id: field, public round_id: u64, public amount: u64, public winner: u64, public map_key: field,) {
        let r : Round = Round {
            game_id: game_id,
            round_id: round_id,
            amount: amount,
            winner: winner,
        };
        Mapping::set(recordMap, map_key, r);
    }


    transition add_user_balance (
        public user_id: address,
        public balance: u64,
    ) {
        assert_eq(aleo1l44dfmwcu7j2e26yhrlhrxla4lsrmr0rxymxfzxj6h8m2mnegyqs8x0end, self.caller);
        return then finalize(user_id, balance);
    }

    finalize add_user_balance (public user_id: address, public balance: u64,) {
        let b:u64 = Mapping::get_or_use(userMap, user_id, 0u64);
        Mapping::set(userMap, user_id, b + balance);
    }

    transition sub_user_balance (
        public user_id: address,
        public balance: u64,
    ) {
        assert_eq(user_id, self.caller);
        return then finalize(user_id, balance);
    }

    finalize sub_user_balance (public user_id: address,public balance: u64,) {
        let b:u64 = Mapping::get_or_use(userMap, user_id, 0u64);
        if b.lt(balance) {
            Mapping::set(userMap, user_id, 0u64);
        }
        Mapping::set(userMap, user_id, b - balance);
    }


    transition save_user_balance (
        public user_id: address,
        public balance: u64,
    ) {
        return then finalize(user_id, balance);
    }

    finalize save_user_balance (public user_id: address, public balance: u64,) {
        Mapping::set(userMap, user_id, balance);
    }


    transition save_game (public game_id: field, public creator: address) -> Game {
        return Game{
            owner: self.caller,
            gates: 0u64,
            game_id: game_id,
            creator: creator
        };
    }


    transition save_result (
        public game_id: field,
        public round_id: u64,
        public user: address,
        public status: u64,
        public amount: u64
    ) -> Result {
        return Result {
            owner: self.caller,
            gates: 0u64,
            game_id: game_id,
            round_id: round_id,
            user: user,
            status: status,
            amount: amount
        };
    }

}
