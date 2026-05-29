import { For } from 'solid-js'
import GalleryItem from './GalleryItem'

interface GalleryProps {
  imgList: Array<{ src: string; title: string }>
}

export default function Gallery(props: GalleryProps) {
  return (
    <div class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-4">
      <For each={props.imgList}>
        {(item) => (
          <GalleryItem src={item.src} title={item.title} />
        )}
      </For>
    </div>
  )
}
