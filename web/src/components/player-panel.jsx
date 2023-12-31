import stateFetcher from '@/fetcher/state'
import useSWR from 'swr'
import Player from './player'
import useLocalStorageState from 'use-local-storage-state'
import StyledButton from './styled-button'
import { useCurrentGameRoom } from '@/hooks/use-game-room'


export default function PlayerPanel() {
  const { data: gamePlayersCard } = useSWR('local:gamePlayersCard', stateFetcher)
  const { data: gamePlayerInfo, mutate: gamePlayerInfoMutate } = useSWR('local:gamePlayerInfo', stateFetcher)
  const { data: gameUsers, mutate: gameUsersMutate } = useSWR('local:gameUsers', stateFetcher)
  const { data: gameCountdown, mutate: gameCountdownMutate } = useSWR('local:gameCountdown', stateFetcher)
  
  const currentUserLocation = gamePlayerInfo?.location

  const showPK = gameCountdown && gamePlayerInfo && gameCountdown.userId === gamePlayerInfo.userId

  return (
    <div className='relative'>
      <Player1 />
      <Player showPK={showPK} user={gameUsers?.[(currentUserLocation + 1) % 5]} name='玩家5' point={100} avatar={7} cards={gamePlayersCard?.[gameUsers?.[(currentUserLocation + 1) % 5]?.userId]} style={ { right: 300, top: 300 } } rightSide />
      <Player showPK={showPK} user={gameUsers?.[(currentUserLocation + 2) % 5]} name='玩家3' point={100} avatar={4} cards={gamePlayersCard?.[gameUsers?.[(currentUserLocation + 2) % 5]?.userId]} style={ { right: 300, top: 120 } } rightSide />
      <Player showPK={showPK} user={gameUsers?.[(currentUserLocation + 3) % 5]} name='玩家2' point={100} avatar={3} cards={gamePlayersCard?.[gameUsers?.[(currentUserLocation + 3) % 5]?.userId]} style={ { left: 80, top: 120 } } />
      <Player showPK={showPK} user={gameUsers?.[(currentUserLocation + 4) % 5]} name='玩家4' point={100} avatar={5} cards={gamePlayersCard?.[gameUsers?.[(currentUserLocation + 4) % 5]?.userId]} style={ { left: 80, top: 300 } } />
    </div>
  )
}

function Player1() {
  const { data: gamePlayerInfo, mutate: gamePlayerInfoMutate } = useSWR('local:gamePlayerInfo', stateFetcher)
  const { data: gameUsers, mutate: gameUsersMutate } = useSWR('local:gameUsers', stateFetcher)
  const { data: gameRoom, mutate: gameRoomMutate } = useSWR('local:gameRoom', stateFetcher)
  const { data: gamePlayersCard, mutate: gamePlayerCardMutate } = useSWR('local:gamePlayersCard', stateFetcher)
  const { data: gameCountdown, mutate: gameCountdownMutate } = useSWR('local:gameCountdown', stateFetcher)

  const gameServer = useCurrentGameRoom()

  const x = 360, y = 470
  return (
    <Player
      avatar={1}
      x={x} y={y}
      style={ { left: x, top: y } }
      user={gamePlayerInfo}
      isCurrentPlayer={true}
      cards={gamePlayersCard?.[gamePlayerInfo?.userId]}
    >
      {
        gamePlayerInfo && <div className='absolute top-12 left-20 px-6 py-1 text-center'>
          {
            [0,3,4,5].includes(gamePlayerInfo?.state) && <StyledButton roundedStyle='rounded-full' className='bg-[#ff9000]' onClick={ () => { gameServer.send({ type: 0, currRound: gameRoom.currRound }) } }>READY</StyledButton>
          }
          {
            gamePlayerInfo.isBanker && gameRoom.state === 0 && gameUsers.filter(u => u.state === 1).length >= gameRoom.minimum && <StyledButton roundedStyle='rounded-full' disabled={!(gamePlayerInfo?.state === 1 && gamePlayerInfo.isBanker && gameUsers.filter(u => u.state === 1).length >= gameRoom.minimum - 1)} onClick={ () => { gameServer.send({ type: 1, currRound: gameRoom.currRound }) } }>START</StyledButton>
          }
          {/* <StyledButton className='bg-[rgb(255,144,0)]' roundedStyle='rounded-full'
            onClick={ async () => { gameServer.send({ type: 2, currRound: gameRoom.currRound }) } }
            disabled={gameCountdown?.userId !== gamePlayerInfo?.userId}
          >CONTINUE</StyledButton> */}
          { gamePlayerInfo.state === 2 && !gamePlayerInfo.isLookCard &&
            <StyledButton className='bg-[rgb(1,145,186)]' roundedStyle='rounded-full'
              onClick={ async () => { gameServer.send({ type: 2, currRound: gameRoom.currRound }) } }
              // disabled={gameCountdown?.userId !== gamePlayerInfo?.userId}
            >CHECK</StyledButton>
          }
        </div>
      }
    </Player>
  )
}
