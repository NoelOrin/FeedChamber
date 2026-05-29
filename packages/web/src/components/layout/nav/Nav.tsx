import { createSignal, onMount, onCleanup, lazy, Suspense } from 'solid-js'
import clsx from 'clsx'
import LinkGroup from './LinkGroup'
import SiteName from './SiteName'
import { useTheme, initTheme } from '~/utils/theme'

const Search = lazy(() => import('../../lc/Search'))

interface NavProps {
  allowBg?: boolean
  transparent?: boolean
  onToggleSidebar?: () => void
}

export default function Nav(props: NavProps) {
  const [isScrolledToTop, setIsScrolledToTop] = createSignal(true)
  const [searchVisible, setSearchVisible] = createSignal(false)
  const { themeMode, themeTitle, toggleTheme } = useTheme()

  let scrollTimeout: ReturnType<typeof setTimeout> | null = null

  const handleScroll = () => {
    if (scrollTimeout) clearTimeout(scrollTimeout)
    scrollTimeout = setTimeout(() => {
      setIsScrolledToTop(window.scrollY <= 0)
    }, 30)
  }

  const setVisible = () => setSearchVisible(true)
  const toggleSidebar = () => props.onToggleSidebar?.()

  onMount(() => {
    initTheme()
    window.addEventListener('scroll', handleScroll, { passive: true })
    handleScroll()
  })

  onCleanup(() => {
    window.removeEventListener('scroll', handleScroll)
    if (scrollTimeout) clearTimeout(scrollTimeout)
  })

  return (
    <div class="flex w-full">
      <nav
        class={clsx(
          'flex justify-center w-full !duration-200',
          props.transparent
            ? 'scrolled-to-top'
            : isScrolledToTop() && 'scrolled-to-top',
        )}
      >
        <div class="flex w-full flex-wrap items-center justify-between py-1.5 px-[8px] lg:max-w-[1080px] xl:max-w-[1080px] 2xl:max-w-[1248px]">
          <div class="flex sm:flex-1 xl:flex-[0_0_auto] justify-start">
            <button
              class="mx-1 p-2 duration-200 btn xl:hidden"
              onClick={toggleSidebar}
            >
              <svg fill="none" height="16" viewBox="0 0 48 48" width="16" xmlns="http://www.w3.org/2000/svg">
                <path d="M7.94971 11.9497H39.9497" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="4" />
                <path d="M7.94971 23.9497H39.9497" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="4" />
                <path d="M7.94971 35.9497H39.9497" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="4" />
              </svg>
            </button>
          </div>

          <div class="flex xl:justify-start sm:justify-center sm:flex-[0_0_auto]">
            <SiteName />
          </div>

          <LinkGroup />

          <div class="flex sm:flex-1 xl:flex-[0_0_auto] justify-end">
            <button class="mx-1 p-2 duration-200 btn flex" title="搜索" onClick={setVisible}>
              <svg fill="none" height="16" viewBox="0 0 48 48" width="16" xmlns="http://www.w3.org/2000/svg">
                <path d="M21 38C30.3888 38 38 30.3888 38 21C38 11.6112 30.3888 4 21 4C11.6112 4 4 11.6112 4 21C4 30.3888 11.6112 38 21 38Z" fill="none" stroke="currentColor" stroke-linejoin="round" stroke-width="3" />
                <path d="M26.657 14.3431C25.2093 12.8954 23.2093 12 21.0001 12C18.791 12 16.791 12.8954 15.3433 14.3431" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="3" />
                <path d="M33.2216 33.2217L41.7069 41.707" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="3" />
              </svg>
            </button>

            <button class="mx-1 p-2 duration-200 btn flex" title="网站数据" onClick={() => window.open('https://analytics.noelorin.top/share/7wQ1dMRJwfHgJNHO', '_blank', 'noopener,noreferrer')}>
              <svg fill="none" height="16" viewBox="0 0 48 48" width="16" xmlns="http://www.w3.org/2000/svg">
                <path d="M44 5H3.99998V17H44V5Z" fill="none" stroke="currentColor" stroke-linejoin="round" stroke-width="3" />
                <path d="M3.99998 41.0301L16.1756 28.7293L22.7549 35.0301L30.7982 27L35.2786 31.368" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="3" />
                <path d="M44 16.1719V42.1719" stroke="currentColor" stroke-linecap="round" stroke-width="3" />
                <path d="M3.99998 16.1719V30.1719" stroke="currentColor" stroke-linecap="round" stroke-width="3" />
                <path d="M13.0155 43H44" stroke="currentColor" stroke-linecap="round" stroke-width="3" />
                <path d="M17 11H38" stroke="currentColor" stroke-linecap="round" stroke-width="3" />
                <path d="M9.99998 10.9966H11" stroke="currentColor" stroke-linecap="round" stroke-width="3" />
              </svg>
            </button>

            <button title={themeTitle()} class="mx-1 p-2 duration-200 btn flex" data-id="theme-btn" onClick={toggleTheme}>
              {/* Sun */}
              <svg class={themeMode() !== 'dark' ? '' : 'hidden'} fill="none" height="16" viewBox="0 0 48 48" width="16" xmlns="http://www.w3.org/2000/svg">
                <path d="M24 37C31.1797 37 37 31.1797 37 24C37 16.8203 31.1797 11 24 11C16.8203 11 11 16.8203 11 24C11 31.1797 16.8203 37 24 37Z" fill="none" stroke="currentColor" stroke-linejoin="round" stroke-width="4" />
                <path d="M24 6C25.3807 6 26.5 4.88071 26.5 3.5C26.5 2.11929 25.3807 1 24 1C22.6193 1 21.5 2.11929 21.5 3.5C21.5 4.88071 22.6193 6 24 6Z" fill="currentColor" />
                <path d="M38.5 12C39.8807 12 41 10.8807 41 9.5C41 8.11929 39.8807 7 38.5 7C37.1193 7 36 8.11929 36 9.5C36 10.8807 37.1193 12 38.5 12Z" fill="currentColor" />
                <path d="M44.5 26.5C45.8807 26.5 47 25.3807 47 24C47 22.6193 45.8807 21.5 44.5 21.5C43.1193 21.5 42 22.6193 42 24C42 25.3807 43.1193 26.5 44.5 26.5Z" fill="currentColor" />
                <path d="M38.5 41C39.8807 41 41 39.8807 41 38.5C41 37.1193 39.8807 36 38.5 36C37.1193 36 36 37.1193 36 38.5C36 39.8807 37.1193 41 38.5 41Z" fill="currentColor" />
                <path d="M24 47C25.3807 47 26.5 45.8807 26.5 44.5C26.5 43.1193 25.3807 42 24 42C22.6193 42 21.5 43.1193 21.5 44.5C21.5 45.8807 22.6193 47 24 47Z" fill="currentColor" />
                <path d="M9.5 41C10.8807 41 12 39.8807 12 38.5C12 37.1193 10.8807 36 9.5 36C8.11929 36 7 37.1193 7 38.5C7 39.8807 8.11929 41 9.5 41Z" fill="currentColor" />
                <path d="M3.5 26.5C4.88071 26.5 6 25.3807 6 24C6 22.6193 4.88071 21.5 3.5 21.5C2.11929 21.5 1 22.6193 1 24C1 25.3807 2.11929 26.5 3.5 26.5Z" fill="currentColor" />
                <path d="M9.5 12C10.8807 12 12 10.8807 12 9.5C12 8.11929 10.8807 7 9.5 7C8.11929 7 7 8.11929 7 9.5C7 10.8807 8.11929 12 9.5 12Z" fill="currentColor" />
              </svg>
              {/* Moon */}
              <svg class={themeMode() !== 'light' ? '' : 'hidden'} fill="none" height="16" viewBox="0 0 48 48" width="16" xmlns="http://www.w3.org/2000/svg">
                <path d="M28.0527 4.41085C22.5828 5.83695 18.5455 10.8106 18.5455 16.7273C18.5455 23.7564 24.2436 29.4545 31.2727 29.4545C37.1894 29.4545 42.1631 25.4172 43.5891 19.9473C43.8585 21.256 44 22.6115 44 24C44 35.0457 35.0457 44 24 44C12.9543 44 4 35.0457 4 24C4 12.9543 12.9543 4 24 4C25.3885 4 26.744 4.14149 28.0527 4.41085Z" fill="none" stroke="currentColor" stroke-linejoin="round" stroke-width="4" />
              </svg>
            </button>
          </div>
        </div>
      </nav>

      {searchVisible() && (
        <Suspense>
          <Search onClose={() => setSearchVisible(false)} />
        </Suspense>
      )}
    </div>
  )
}
