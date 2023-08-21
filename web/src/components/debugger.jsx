export default function DebuggerLayer() {
  return (
    <div className='fixed top-1 left-1 min-w-min bg-white/30 p-4 backdrop-blur-sm'>
      <button type='button'
        className='rounded bg-indigo-600 px-2 py-1 text-xs font-semibold text-white shadow-sm hover:bg-indigo-500 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-indigo-600'
        onClick={ async () => {
          const payload = await (await fetch(`http://162.219.87.221/api/game/create`, {
            method: 'POST',
            credentials: 'include',
            body: JSON.stringify({
              lowBetChips: 6,
              minimum: 2,
              topBetChips: 6,
              totalRounds: 100,
            })
          })).json()
          console.log(payload)
        } }
      >
        创建游戏房间
      </button>

      <button type='button'
        className='rounded bg-indigo-600 px-2 py-1 text-xs font-semibold text-white shadow-sm hover:bg-indigo-500 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-indigo-600'
        onClick={ async () => {
          `ws://162.219.87.221/ws?gameId=d01d687d4bb444e48fcb999facb33db5`
        } }
      >
        进入房间
      </button>

      <button type='button'
        className='rounded bg-indigo-600 px-2 py-1 text-xs font-semibold text-white shadow-sm hover:bg-indigo-500 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-indigo-600'
      >玩家下注
      </button>
    </div>
  )
}