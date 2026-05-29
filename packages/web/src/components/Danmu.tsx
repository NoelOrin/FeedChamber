import { createSignal } from 'solid-js'

export default function Danmu(props: { children?: import('solid-js').JSX.Element }) {
  let danmuContainer: HTMLDivElement | undefined

  return (
    <div
      class="fixed top-1/2 bottom-1/2 -translate-x-1/2 -translate-y-1/2"
      ref={danmuContainer}
    >
      {props.children}
    </div>
  )
}
