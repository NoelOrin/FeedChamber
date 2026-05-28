import { Link } from '@tanstack/solid-router'

const navLinks = [
  { href: '/about', label: '关于' },
  { href: '/friends', label: '友链' },
  { href: '/contact', label: '联系我' },
]

export default function Header() {
  return (
    <div class="fixed top-0 left-0 right-0 z-40 flex w-full border-b border-transparent bg-inherit/80 backdrop-blur-sm">
      <nav class="flex justify-center w-full !duration-200">
        <div class="flex w-full flex-wrap items-center justify-between py-1.5 px-[8px] lg:max-w-[1080px] xl:max-w-[1080px] 2xl:max-w-[1248px]">
          <div class="flex sm:flex-1 xl:flex-[0_0_auto] justify-start">
            <button class="mx-1 p-2 duration-200 btn xl:hidden">
              <svg fill="none" height="20" viewBox="0 0 48 48" width="20" xmlns="http://www.w3.org/2000/svg">
                <path d="M7.94971 11.9497H39.9497" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="4" />
                <path d="M7.94971 23.9497H39.9497" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="4" />
                <path d="M7.94971 35.9497H39.9497" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="4" />
              </svg>
            </button>
          </div>

          <div class="flex xl:justify-start sm:justify-center sm:flex-[0_0_auto]">
            <Link to="/" class="flex items-center gap-2 rounded-lg cursor-pointer font-black px-2 py-1 hover:opacity-80">
              <div id="logo" class="ml-0 select-none">
                <div id="site-name" style="font-family:'Segoe UI',Arial,'Microsoft Yahei',sans-serif;">
                  NoelOrin's Blog
                </div>
              </div>
            </Link>
          </div>

          <div class="space-x-[12px] sm:hidden xl:flex select-none box-border flex-1 justify-center" style="font-family:'Segoe UI',Arial,'Microsoft Yahei',sans-serif;">
            {navLinks.map(link => (
              <a href={link.href} class="flex items-center justify-center font-black px-[10px] py-2 cursor-pointer rounded-lg hover:opacity-80">
                {link.label}
              </a>
            ))}
          </div>

          <div class="flex sm:flex-1 xl:flex-[0_0_auto] justify-end">
            <button class="mx-1 p-2 duration-200 btn flex" title="搜索">
              <svg fill="none" height="20" viewBox="0 0 48 48" width="20" xmlns="http://www.w3.org/2000/svg">
                <path d="M21 38C30.3888 38 38 30.3888 38 21C38 11.6112 30.3888 4 21 4C11.6112 4 4 11.6112 4 21C4 30.3888 11.6112 38 21 38Z" fill="none" stroke="currentColor" stroke-linejoin="round" stroke-width="3" />
                <path d="M26.657 14.3431C25.2093 12.8954 23.2093 12 21.0001 12C18.791 12 16.791 12.8954 15.3433 14.3431" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="3" />
                <path d="M33.2216 33.2217L41.7069 41.707" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="3" />
              </svg>
            </button>
            <button class="mx-1 p-2 duration-200 btn flex" title="主题切换">
              <svg fill="none" height="20" viewBox="0 0 48 48" width="20" xmlns="http://www.w3.org/2000/svg">
                <path d="M28.0527 4.41085C22.5828 5.83695 18.5455 10.8106 18.5455 16.7273C18.5455 23.7564 24.2436 29.4545 31.2727 29.4545C37.1894 29.4545 42.1631 25.4172 43.5891 19.9473C43.8585 21.256 44 22.6115 44 24C44 35.0457 35.0457 44 24 44C12.9543 44 4 35.0457 4 24C4 12.9543 12.9543 4 24 4C25.3885 4 26.744 4.14149 28.0527 4.41085Z" fill="none" stroke="currentColor" stroke-linejoin="round" stroke-width="4" />
              </svg>
            </button>
          </div>
        </div>
      </nav>
    </div>
  )
}