import { useTheme, initTheme } from '~/utils/theme'
import { onMount } from 'solid-js'

export default function AboutNav() {
  const { themeMode, themeTitle, toggleTheme } = useTheme()

  onMount(() => {
    initTheme()
  })

  return (
    <div class="pointer-events-none flex w-full">
      <nav class="flex w-full justify-center duration-250 scrolled-to-top">
        <div class="flex w-full flex-wrap items-center justify-between py-1.5 px-[8px]">
          <a href="/" class="pointer-events-auto">
            <div id="_logo" class="ml-0 select-none">
              <div id="site-name" style="font-family: 'Segoe UI',Arial,'Microsoft Yahei',sans-serif;">
                NoelOrin's Blog
              </div>
            </div>
          </a>
          <div class="flex flex-row">
            <button
              class="pointer-events-auto mx-1 hidden p-2 duration-200 btn sm:flex"
              data-id="theme-btn"
              title={themeTitle()}
              onClick={toggleTheme}
            >
              {/* Sun - light */}
              <svg class={themeMode() === 'light' ? '' : 'hidden'} fill="none" height="16" viewBox="0 0 48 48" width="16" xmlns="http://www.w3.org/2000/svg">
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
              {/* Moon - dark */}
              <svg class={themeMode() === 'dark' ? '' : 'hidden'} fill="none" height="16" viewBox="0 0 48 48" width="16" xmlns="http://www.w3.org/2000/svg">
                <path d="M28.0527 4.41085C22.5828 5.83695 18.5455 10.8106 18.5455 16.7273C18.5455 23.7564 24.2436 29.4545 31.2727 29.4545C37.1894 29.4545 42.1631 25.4172 43.5891 19.9473C43.8585 21.256 44 22.6115 44 24C44 35.0457 35.0457 44 24 44C12.9543 44 4 35.0457 4 24C4 12.9543 12.9543 4 24 4C25.3885 4 26.744 4.14149 28.0527 4.41085Z" fill="none" stroke="currentColor" stroke-linejoin="round" stroke-width="4" />
              </svg>
              {/* Monitor - system */}
              <svg class={themeMode() === 'system' ? '' : 'hidden'} fill="none" height="16" viewBox="0 0 24 24" width="16" xmlns="http://www.w3.org/2000/svg">
                <rect x="2" y="3" width="20" height="14" rx="2" stroke="currentColor" stroke-width="2" />
                <path d="M8 21h8M12 17v4" stroke="currentColor" stroke-width="2" stroke-linecap="round" />
              </svg>
            </button>
          </div>
        </div>
      </nav>
    </div>
  )
}
