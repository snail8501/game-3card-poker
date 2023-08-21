'use client'
import WithSignined from '../with-signin'
import { useSearchParams } from 'next/navigation'
import GameRoom from './room'

export default function GamePage() {
  const gameId = useSearchParams().get('gameId')
  if(!/^[\dabcedf]{32,32}$/.test(gameId)) {
    return (
      <>非法的房间号{ gameId }</>
    )
  }

  return (
    <WithSignined>{
      userInfo => (<GameRoom gameId={gameId} token={userInfo.token} />)
    }</WithSignined>
  )
}
