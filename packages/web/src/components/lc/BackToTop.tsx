import { createSignal, onMount, onCleanup } from 'solid-js'

export default function BackToTop() {
  const [shouldShowBackToTop, setShouldShowBackToTop] = createSignal(false)
  let scrollTimeout: ReturnType<typeof setTimeout> | null = null

  const handleScroll = () => {
    if (scrollTimeout) clearTimeout(scrollTimeout)
    scrollTimeout = setTimeout(() => {
      setShouldShowBackToTop(window.scrollY > 100)
    }, 30)
  }

  const scrollToTop = () => {
    window.scrollTo({ top: 0, behavior: 'smooth' })
  }

  onMount(() => {
    if (typeof window !== 'undefined') {
      window.addEventListener('scroll', handleScroll)
    }
  })

  onCleanup(() => {
    if (typeof window !== 'undefined') {
      window.removeEventListener('scroll', handleScroll)
    }
    if (scrollTimeout) clearTimeout(scrollTimeout)
  })

  return (
    <div
      class="sm:right-12 lg:right-20 bottom-16 cursor-pointer p-2 duration-200 backToTop"
      classList={{ hidden: !shouldShowBackToTop() }}
      style="border-radius: 55rem;"
      onClick={scrollToTop}
    >
      <svg width="22" height="22" viewBox="0 0 48 48" fill="none" xmlns="http://www.w3.org/2000/svg">
        <path d="M13 30L25 18L37 30" stroke="currentColor" stroke-width="4" stroke-linecap="round" stroke-linejoin="round" />
      </svg>
    </div>
  )
}
