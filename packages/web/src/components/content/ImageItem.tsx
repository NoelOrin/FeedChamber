import { Show } from 'solid-js'

interface ImageItemProps {
  src: string
  alt?: string
  title?: string
}

export default function ImageItem(props: ImageItemProps) {
  return (
    <>
      <div class="flex select-none flex-col items-center justify-center overflow-hidden max-h-[385px] rounded-[0.25rem] mb-[8px]">
        <img
          alt={props.alt || ''}
          src={props.src}
          class="max-w-full max-h-[385px] object-contain"
          loading="lazy"
        />
        <Show when={props.title}>
          <div class="-translate-y-1/4 text-xs opacity-80">{props.title}</div>
        </Show>
      </div>
      <link rel="preload" href={props.src} as="image" />
    </>
  )
}
