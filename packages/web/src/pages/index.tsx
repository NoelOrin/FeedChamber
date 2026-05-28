import { createFileRoute } from '@tanstack/solid-router'

export const Route = createFileRoute('/')({
  component: Home,
})

const articles = [
  {
    id: 1,
    title: '迁移评论系统 Waline+pg',
    desc: '',
    cover: 'https://oss.noelorin.top/4a7d0903-39a6-4f73-bd38-77f67e8b4f87-7a96a0e017af2feb1b1c8fbe33439d16.png',
    link: '/b/waline-postgresql',
    views: 13,
    comments: 0,
    date: '四天前',
    tags: ['博客'],
  },
  {
    id: 2,
    title: '部署属于自己的MC服务器',
    desc: 'dokcer一键部署的轮椅服务器',
    cover: 'https://oss.noelorin.top/45db0fdf-24d9-4291-9cdf-84d3fc819acb-399a124f0eec294362e53008df1998b8.png',
    link: '/b/own-mc-server',
    views: 226,
    comments: 0,
    date: '五天前',
    tags: ['MineCraft'],
  },
  {
    id: 3,
    title: '用MarkdownAST来渲染Vue组件',
    desc: '理论上运行时框架和编译时框架都能用这种方法',
    cover: 'https://oss.noelorin.top/7ae8c679-971d-4f45-a6db-7843fd657507-10340492-2816-407a-9c22-0841b447cdd0',
    link: '/b/md-ast',
    views: 236,
    comments: 0,
    date: '2025年12月14日',
    tags: ['前端'],
  },
  {
    id: 4,
    title: 'Nuxt主题加载/切换闪烁解决方法',
    desc: '经验之道！',
    cover: 'https://oss.noelorin.top/5be015f9-9ebd-4252-85fb-21e2fa9c13cf-f4904640bf6f2b4521838dd683147d5a.png',
    link: '/b/nuxt-theme-usehaed',
    views: 269,
    comments: 0,
    date: '2025年12月13日',
    tags: ['前端'],
  },
  {
    id: 5,
    title: 'Nuxt4集成Algolia搜索服务',
    desc: '整它这个搜索组件比做索引麻烦多了',
    cover: 'https://oss.noelorin.top/3b3c55e7-b2e8-40c2-ab64-36e38cb9fe85-asu.avif',
    link: '/b/aloglia-nuxt',
    views: 251,
    comments: 0,
    date: '2025年12月10日',
    tags: ['前端'],
  },
  {
    id: 6,
    title: 'QQBot-Docker快速部署Napcat&AstrBot',
    desc: '插件太有活了',
    cover: 'https://oss.noelorin.top/2be1c9ff-dd72-4122-97bd-9b38ca15a69c-ff6d81cad7164377524cb13d1a4369eb.jpg',
    link: '/b/napcat-astrBot',
    views: 273,
    comments: 0,
    date: '2025年12月06日',
    tags: ['BOT'],
  },
]

function ArticleCard(props: (typeof articles)[number]) {
  return (
    <div class="box-content cursor-pointer select-none p-0 card min-h-[90px] h-fit rounded-lg bg-white/60 dark:bg-[#3e3e44ad] overflow-hidden">
      <a href={props.link} class="inline-flex flex-col w-full">
        <div class="select-none overflow-hidden bg-transparent">
          <img
            alt={props.title}
            class="select-none aspect-[2/1] duration-200 w-full object-cover hover:scale-105 transition-transform"
            src={props.cover}
            loading="lazy"
          />
        </div>
        <article class="px-[16px] py-[10px] w-full">
          <header>
            <div class="mb-1 title whitespace-nowrap text-ellipsis overflow-hidden font-semibold text-base">
              {props.title}
            </div>
            <p class="description min-h-[18px] text-sm text-gray-500 dark:text-gray-400">
              {props.desc}
            </p>
          </header>
          <div class="flex justify-between text-sm mb-[4px] text-gray-500 dark:text-gray-400 mt-2">
            <div class="flex space-x-[8px]">
              <div class="flex items-center space-x-1">
                <svg fill="currentColor" height="14" viewBox="0 0 24 24" width="14" xmlns="http://www.w3.org/2000/svg">
                  <g><path d="M12 3c5.392 0 9.878 3.88 10.819 9-.94 5.12-5.427 9-10.819 9-5.392 0-9.878-3.88-10.818-9C2.122 6.88 6.608 3 12 3Zm0 16a9.005 9.005 0 0 0 8.778-7 9.005 9.005 0 0 0-17.555 0A9.005 9.005 0 0 0 12 19Zm0-2.5a4.5 4.5 0 1 1 0-9 4.5 4.5 0 0 1 0 9Zm0-2a2.5 2.5 0 1 0 0-5 2.5 2.5 0 0 0 0 5Z" /></g>
                </svg>
                <span>{props.views}</span>
              </div>
              <div class="flex items-center space-x-1">
                <svg fill="none" height="14" viewBox="0 0 48 48" width="14" xmlns="http://www.w3.org/2000/svg">
                  <path d="M44 6H4V36H13V41L23 36H44V6Z" fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="4" />
                  <path d="M14 21H34" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="4" />
                </svg>
                <span>{props.comments}</span>
              </div>
            </div>
            <div class="flex items-center">
              <svg fill="none" height="14" viewBox="0 0 48 48" width="14" xmlns="http://www.w3.org/2000/svg">
                <path d="M24 44C35.0457 44 44 35.0457 44 24C44 12.9543 35.0457 4 24 4C12.9543 4 4 12.9543 4 24C4 35.0457 12.9543 44 24 44Z" fill="none" stroke="currentColor" stroke-linejoin="round" stroke-width="4" />
                <path d="M24.0084 12.0001L24.0072 24.0089L32.4866 32.4883" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="4" />
              </svg>
              <span class="ml-1">{props.date}</span>
            </div>
          </div>
        </article>
      </a>
    </div>
  )
}

function Home() {
  return (
    <div class="w-full h-full flex flex-col">
      <div class="grid gap-4 lg:grid-cols-1 xl:grid-cols-2">
        {articles.map(article => (
          <ArticleCard {...article} />
        ))}
      </div>

      {/* Pagination */}
      <div id="pagination" class="select-none pt-8 flex items-center justify-center gap-4">
        <div class="flex items-center gap-2">
          <span class="px-3 py-1 rounded bg-gray-200 dark:bg-zinc-700 font-bold">1</span>
          <a href="/?page=2" class="px-3 py-1 rounded hover:bg-gray-100 dark:hover:bg-zinc-700">2</a>
        </div>
        <a href="/?page=2" class="flex items-center">
          <svg width="22" height="22" viewBox="0 0 48 48" fill="none" xmlns="http://www.w3.org/2000/svg">
            <path d="M12 12L24 24L12 36" stroke="currentColor" stroke-width="4" stroke-linecap="round" stroke-linejoin="round" />
            <path d="M24 12L36 24L24 36" stroke="currentColor" stroke-width="4" stroke-linecap="round" stroke-linejoin="round" />
          </svg>
        </a>
      </div>
    </div>
  )
}