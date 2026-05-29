import { createSignal, onCleanup } from 'solid-js'

interface Position {
  x: number
  y: number
}

interface DragState {
  isDragging: boolean
  offset: Position
}

export default function HoverWindow(props: { children?: import('solid-js').JSX.Element }) {
  const [position, setPosition] = createSignal<Position>({ x: 0, y: 0 })

  let state: DragState = {
    isDragging: false,
    offset: { x: 0, y: 0 },
  }

  const onMouseMove = (e: MouseEvent) => {
    if (!state.isDragging) return
    setPosition({ x: e.clientX - state.offset.x, y: e.clientY - state.offset.y })
  }

  const onMouseUp = () => {
    state.isDragging = false
    document.removeEventListener('mousemove', onMouseMove)
    document.removeEventListener('mouseup', onMouseUp)
  }

  const onMouseDown = (e: MouseEvent) => {
    state.isDragging = true
    state.offset = { x: e.clientX - position().x, y: e.clientY - position().y }
    document.addEventListener('mousemove', onMouseMove)
    document.addEventListener('mouseup', onMouseUp)
  }

  onCleanup(() => {
    document.removeEventListener('mousemove', onMouseMove)
    document.removeEventListener('mouseup', onMouseUp)
  })

  return (
    <div
      class="bottom-16 left-1/2 -translate-x-1/2 p-8 draggable-card card"
      style={{
        transform: `translate(${position().x}px, ${position().y}px)`,
      }}
      onMouseDown={onMouseDown}
    >
      {props.children}
    </div>
  )
}
