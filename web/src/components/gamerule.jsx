import FrameBox from './frame-box'
import StyledButton from './styled-button'

export default function GameRule({ onClose }) {
  return (
    <FrameBox
      onClose={onClose}
    >
      <div className='w-[540px] m-10 text-center text-white'>
        <img src='/gamerule-content.png' />
      </div>
    </FrameBox>
  )
}