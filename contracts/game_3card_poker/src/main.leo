// The 'game_3_card_poker' program.
program game_3_card_poker.aleo {
    transition main(public a: u32, b: u32) -> u32 {
        let c: u32 = a + b;
        return c;
    }

    record Round {
        owner: address,
        gates: u64,
        game_id: field,
        round_id: u64,
        total_amount: u64,
        winner: field
    }

    record UserRecord {
        owner: address,
        gates: u64,
        game_id: field,
        round_id: u64,
        user: field,
        poker: field,
        status: i8,
        amount: u64
    }


    transition save_round (
        public game_id: field,
        public round_id: u64,
        public owner: address,
        public total_amount: u64,
        public winner: field
    ) -> Round {
        return Round {
            owner: owner,
            gates: 0u64,
            game_id: game_id,
            round_id: round_id,
            total_amount: total_amount,
            winner: winner
        };
    }


    transition save_user_record (
        public game_id: field,
        public round_id: u64,
        public owner: address,
        public user: field,
        public poker: field,
        public status: i8,
        public amount: u64
    ) -> UserRecord {
        return UserRecord {
            owner: owner,
            gates: 0u64,
            game_id: game_id,
            round_id: round_id,
            user: user,
            poker: poker,
            status: status,
            amount: amount
        };
    }
}
