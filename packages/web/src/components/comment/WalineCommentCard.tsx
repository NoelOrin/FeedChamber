import { createSignal, onMount } from 'solid-js'

interface WalineCommentCardProps {
  serverURL: string
  path?: string
  lang?: string
}

export default function WalineCommentCard(props: WalineCommentCardProps) {
  const [ready, setReady] = createSignal(false)

  onMount(async () => {
    try {
      // @waline/client provides an init function that works imperatively
      // This avoids the Vue/React component and works with any framework
      const waline = await import('@waline/client')
      waline.init({
        el: '#waline-comment',
        serverURL: props.serverURL,
        path: props.path || window.location.pathname,
        lang: props.lang || 'zh-CN',
      })
      setReady(true)
    } catch (err) {
      console.error('Failed to init Waline:', err)
    }
  })

  return (
    <div class="flex-1 card mb-[112px]" id="COMMENT">
      <div class="ml-2 flex flex-row items-center text-xl cardHeaderTitle">
        <svg
          class="mr-2 inline-block"
          fill="none"
          height="24"
          viewBox="0 0 48 48"
          width="24"
          xmlns="http://www.w3.org/2000/svg"
        >
          <path
            d="M42.1691 29.2451L29.2631 42.1511C28.5879 42.8271 27.6716 43.2069 26.7161 43.2069C25.7606 43.2069 24.8444 42.8271 24.1691 42.1511L8 26V8H26L42.1691 24.1691C43.5649 25.5732 43.5649 27.841 42.1691 29.2451Z"
            fill="none"
            stroke="currentColor"
            stroke-linejoin="round"
            stroke-width="4"
          />
          <path
            clip-rule="evenodd"
            d="M18.5 21C19.8807 21 21 19.8807 21 18.5C21 17.1193 19.8807 16 18.5 16C17.1193 16 16 17.1193 16 18.5C16 19.8807 17.1193 21 18.5 21Z"
            fill="currentColor"
            fill-rule="evenodd"
          />
        </svg>
        <div class="inline-block">
          <span class="text-base">评论区</span>
        </div>
      </div>
      <div id="waline-comment" />
    </div>
  )
}
