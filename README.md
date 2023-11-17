# game 3card poker

"3Card Poker" is a DAO Governance Game DApp where all transactions and assets are recorded on an immutable blockchain.
Players can choose to look at their own cards. The cards they have seen are open cards. They need to put their chips into the prize pool in the center of the table. Players who do not choose to "give up" can choose to look at their own cards when it is their turn to act. The cards they have seen are open cards. Players who give up in the game have no right to see their cards and other players' cards before ending the game. At the end of the game, the cards of all players will be disclosed. Compared to the card is an important part of the game, players can choose to compare cards, raise or compare cards, until the final win.

Basic rules of 1.
- The rules for Game 3card Poker are relatively simple and require at least two people to participate. The game uses a deck of playing cards with the size of the king removed. It is usually played by three or four people. Each person can get three cards, and the person with the largest face wins.

Game 3card Poker type is special, roughly divided into the following types:
1. Leopard: that is, three identical cards, the largest leopard is AAA, the smallest leopard is 222, leopard is the largest card type.
2. Shun Jin: three connected flush cards, such as 345,678, etc.
3. Golden Flower: three flush cards, not continuous.
4. Shunzi: three connected different flower cards, such as 345,678, etc.
5. Pair: There are two identical cards.
6. Scattered cards: that is, three different cards.

In Game 3card Poker, the leopard is the largest card type, followed by Shun Jin, Golden Flower, Shun Zi, Pair, Scattered cards. Compare face sizes when two or more of the same card type appear.

Competition Rules2.
- After the game begins, the player to the left of the dealer begins to bid. There are three ways to bid: see, compare and fold. Looking at the card means that the player can choose to continue playing after seeing his own card. Comparing the card means that the player can compare the size of the card face with other players to see whose card face is larger. Abandoning the card means giving up the current round of the game. If only one person sees the card, then he is the winner; if more than one person sees the card, then the person with the largest face is the winner; if everyone abandons the card, then the last person to see the card is the winner.

Special circumstances 3.
1. If the card face is the same size, then when comparing the color size, spades> hearts> plum blossom> squares, spades are the largest color and squares are the smallest color.
2. Three A's can knock over all other card types, which is the largest card type; Three 2's are the smallest leopard; Other special card types such as 235 may be defined differently in different regions and under different rules.
3. During the game, sometimes there will be situations that need to be doubled. For example, when one player is larger than another player, he can choose to double; if another player follows the card again, he needs to double again.

Game 3card Poker is a very interesting poker game. Although the rules are relatively simple, the player's wisdom and strategy are still required during the game. I hope this article can help readers better understand the basic rules of fried golden flowers, so that everyone can enjoy the fun of the game while enjoying leisure and entertainment.

Test Network Game Address: http://162.219.87.221

## 1、Create Game, Share a link Invite a friend to play
<img alt="zenet" width="1412" src=".resources/image-1.jpg">

## 2、Game Playing, interactive betting、making comparisons
<img alt="zenet" width="1412" src=".resources/image-2.jpg">

## 3、Game Over
<img alt="zenet" width="1412" src=".resources/image-3.jpg">


## Build Guide

To compile this Aleo program, run:
```bash

## contracts publish
cd contracts/game_3_card_poker
leo run
snarkos developer deploy "game_3_card_poker.aleo" --private-key "" --query "https://vm.aleo.org/api" --path "./build/" --broadcast "https://vm.aleo.org/api/testnet3/transaction/broadcast" --fee 1000 --record ""


## ui build
cd web
npm install 
npm run build
docker compose up -d

## server build
cd server
docker compose up -d
```