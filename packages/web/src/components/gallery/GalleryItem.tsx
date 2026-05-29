interface GalleryItemProps {
  src: string
  title: string
}

export default function GalleryItem(props: GalleryItemProps) {
  return (
    <div class="overflow-hidden rounded-lg">
      <img
        src={props.src}
        alt={props.title}
        class="w-full h-full object-cover"
        loading="lazy"
      />
    </div>
  )
}
