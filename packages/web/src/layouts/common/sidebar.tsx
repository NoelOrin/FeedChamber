import { Link } from '@tanstack/solid-router'

const categories = [
  { name: '全部文章', count: 8, href: '/?page=1' },
  { name: '技术', count: 7, href: '/?page=1&category=技术' },
  { name: '游戏', count: 1, href: '/?page=1&category=游戏' },
]

const tags = [
  { name: 'BOT', count: 1, href: '/?page=1&tag=BOT' },
  { name: 'MineCraft', count: 1, href: '/?page=1&tag=MineCraft' },
  { name: '前端', count: 3, href: '/?page=1&tag=前端' },
  { name: '博客', count: 1, href: '/?page=1&tag=博客' },
]

export default function Sidebar() {
  return (
    <aside id="sidebar-left" class="w-[260px] shrink-0 hidden xl:block">
      <div class="relative text-center card rounded-lg bg-white/60 dark:bg-[#3e3e44ad] p-6">
        <Link to="/" class="mx-auto w-[114px] h-[114px] block">
          <img
            class="rounded-full"
            src="https://avatars.githubusercontent.com/u/112020558"
            alt="NoelOrin"
            width={114}
            height={114}
          />
        </Link>
        <div class="mt-2 flex flex-row items-center justify-between">
          <div class="flex flex-col items-start font-semibold">
            <span class="text-[22px]">NoelOrin</span>
            <span class="text-sm">只是个占位的</span>
          </div>
          <div class="flex flex-row items-center">
            <a href="https://github.com/NoelOrin" target="_blank" rel="noopener noreferrer">
              <div class="rounded-3xl bg-black p-2">
                <svg viewBox="0 0 1024 1024" width="24" height="24" fill="#fff">
                  <path d="M682.215454 981.446137c-25.532318 0-42.553863-17.021545-42.553864-42.553864v-165.960067c4.255386-34.043091-8.510773-59.575409-29.787704-80.852341-12.766159-12.766159-17.021545-29.787704-8.510773-42.553864 4.255386-17.021545 21.276932-25.532318 34.043091-29.787704 123.406204-12.766159 238.301635-55.320023 238.301635-255.323181 0-46.80925-17.021545-93.6185-51.064636-131.916976-12.766159-12.766159-12.766159-29.787704-8.510772-42.553864 12.766159-34.043091 12.766159-68.086182 4.255386-102.129272-21.276932 4.255386-55.320023 17.021545-110.640045 55.320022-8.510773 8.510773-21.276932 8.510773-34.043091 4.255387-89.363113-25.532318-187.236999-25.532318-276.600112 0-12.766159 4.255386-25.532318 4.255386-38.298477-4.255387C307.741455 104.836549 269.442978 92.07039 248.166047 87.815004c-8.510773 34.043091-8.510773 68.086182 4.255386 102.129272 4.255386 17.021545 4.255386 34.043091-8.510773 42.553864-34.043091 38.298477-51.064636 85.107727-51.064636 131.916976 0 200.003158 114.895431 242.557022 238.301635 255.323181 17.021545 0 29.787704 12.766159 34.043091 29.787704 4.255386 17.021545 0 34.043091-8.510773 42.553864-21.276932 21.276932-29.787704 46.80925-29.787704 76.596954v165.960068c0 25.532318-17.021545 42.553863-42.553863 42.553863s-42.553863-17.021545-42.553864-42.553863v-72.341568c-127.66159 21.276932-182.981613-51.064636-221.28009-97.873886-17.021545-21.276932-29.787704-38.298477-46.80925-42.553864-21.276932-4.255386-38.298477-29.787704-29.787704-51.064636 4.255386-21.276932 29.787704-38.298477 51.064636-29.787704 42.553863 12.766159 68.086182 42.553863 93.6185 72.341568 34.043091 46.80925 63.830795 80.852341 153.193908 63.830795v-4.255386c0-25.532318 4.255386-55.320023 12.766159-76.596955-119.150818-25.532318-246.812408-102.129272-246.812408-327.664748 0-63.830795 21.276932-123.406204 59.575409-170.215454-17.021545-59.575409-12.766159-114.895431 12.766159-170.215454 4.255386-12.766159 12.766159-21.276932 25.532318-25.532318 17.021545-4.255386 72.341568-12.766159 187.236999 59.575409 93.6185-21.276932 191.492386-21.276932 280.855499 0 110.640045-72.341568 170.215454-63.830795 187.236999-59.575409 12.766159 4.255386 21.276932 12.766159 25.532319 25.532318 21.276932 55.320023 25.532318 110.640045 12.766159 165.960067 38.298477 46.80925 59.575409 106.384659 59.575408 170.215454 0 242.557022-144.683136 306.387817-246.812408 331.920135 8.510773 25.532318 12.766159 55.320023 12.766159 80.852341V938.892273c0 25.532318-17.021545 42.553863-42.553863 42.553864z" />
                </svg>
              </div>
            </a>
          </div>
        </div>
        <a href="/about">
          <button class="mt-[20px] w-full cursor-pointer p-2 rounded-full border border-gray-300 dark:border-gray-600 font-bold hover:bg-gray-100 dark:hover:bg-gray-700">
            About me
          </button>
        </a>
      </div>

      <div class="sticky top-12 flex flex-col">
        {/* Categories */}
        <div class="mt-4 flex flex-col card rounded-lg bg-white/60 dark:bg-[#3e3e44ad] p-[16px] pt-[20px]">
          <div class="ml-2 flex flex-row items-end text-xl cardHeaderTitle">
            <svg class="mr-2 inline-block mb-0.5" width="22" height="22" viewBox="0 0 48 48" fill="none" xmlns="http://www.w3.org/2000/svg">
              <path d="M8 44L8 6C8 4.89543 8.89543 4 10 4H38C39.1046 4 40 4.89543 40 6V44L24 35.7273L8 44Z" fill="none" stroke="currentColor" stroke-width="4" stroke-linejoin="round" />
              <path d="M16 18H32" stroke="currentColor" stroke-width="4" stroke-linecap="round" stroke-linejoin="round" />
            </svg>
            <div class="inline-block">
              <span> Category <span class="ml-2 text-base">分类</span></span>
            </div>
          </div>
          <div class="mt-4 text-sm card-body">
            <ul>
              {categories.map(cat => (
                <li class="my-0.5">
                  <a href={cat.href}>
                    <span class="flex w-full cursor-pointer flex-row items-center justify-between truncate px-2 py-1 duration-200 rounded hover:bg-gray-100 dark:hover:bg-zinc-700">
                      <span>{cat.name}</span>
                      <span class="px-2 py-1 text-xs rounded bg-gray-100 dark:bg-zinc-700">{cat.count}</span>
                    </span>
                  </a>
                </li>
              ))}
            </ul>
          </div>
        </div>

        {/* Tags */}
        <div class="mt-4 flex flex-col card rounded-lg bg-white/60 dark:bg-[#3e3e44ad] p-[16px] pt-[20px]">
          <div class="ml-2 flex flex-row items-center text-xl cardHeaderTitle">
            <svg class="mr-2 inline-block" width="24" height="24" viewBox="0 0 48 48" fill="none" xmlns="http://www.w3.org/2000/svg">
              <path d="M42.1691 29.2451L29.2631 42.1511C28.5879 42.8271 27.6716 43.2069 26.7161 43.2069C25.7606 43.2069 24.8444 42.8271 24.1691 42.1511L8 26V8H26L42.1691 24.1691C43.5649 25.5732 43.5649 27.841 42.1691 29.2451Z" fill="none" stroke="currentColor" stroke-width="4" stroke-linejoin="round" />
              <path fill-rule="evenodd" clip-rule="evenodd" d="M18.5 21C19.8807 21 21 19.8807 21 18.5C21 17.1193 19.8807 16 18.5 16C17.1193 16 16 17.1193 16 18.5C16 19.8807 17.1193 21 18.5 21Z" fill="currentColor" />
            </svg>
            <div class="inline-block">
              <span> Tag <span class="ml-2 text-base">标签</span></span>
            </div>
          </div>
          <div class="mt-2 card-body">
            <div class="flex flex-wrap gap-2">
              {tags.map(tag => (
                <a href={tag.href} class="flex cursor-pointer flex-row items-start px-2 py-1.5 rounded-lg bg-white/60 dark:bg-[#3e3e44ad] space-x-1.5 hover:opacity-80">
                  <div>{tag.name}</div>
                  <div class="text-xs opacity-75">{tag.count}</div>
                </a>
              ))}
            </div>
          </div>
        </div>

        {/* Friend Links */}
        <div class="mt-4 flex flex-col card rounded-lg bg-white/60 dark:bg-[#3e3e44ad] p-[16px] pt-[20px]">
          <div class="ml-2 flex flex-row items-center text-xl cardHeaderTitle">
            <svg class="mr-2 inline-block" width="24" height="24" viewBox="0 0 48 48" fill="none" xmlns="http://www.w3.org/2000/svg">
              <circle cx="14" cy="29" r="5" fill="none" stroke="currentColor" stroke-width="4" stroke-linecap="round" stroke-linejoin="round" /><circle cx="34" cy="29" r="5" fill="none" stroke="currentColor" stroke-width="4" stroke-linecap="round" stroke-linejoin="round" />
              <circle cx="24" cy="9" r="5" fill="none" stroke="currentColor" stroke-width="4" stroke-linecap="round" stroke-linejoin="round" />
              <path d="M24 44C24 38.4772 19.5228 34 14 34C8.47715 34 4 38.4772 4 44" stroke="currentColor" stroke-width="4" stroke-linecap="round" stroke-linejoin="round" />
              <path d="M44 44C44 38.4772 39.5228 34 34 34C28.4772 34 24 38.4772 24 44" stroke="currentColor" stroke-width="4" stroke-linecap="round" stroke-linejoin="round" />
              <path d="M34 24C34 18.4772 29.5228 14 24 14C18.4772 14 14 18.4772 14 24" stroke="currentColor" stroke-width="4" stroke-linecap="round" stroke-linejoin="round" />
            </svg>
            <div class="inline-block">
              <span> Friend <span class="ml-2 text-base">友链</span></span>
            </div>
          </div>
          <div class="mt-4 text-sm card-body" />
        </div>
      </div>
    </aside>
  )
}