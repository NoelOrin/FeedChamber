export default function SiteName() {
  return (
    <div class="sm:block xl:hidden group/sitename relative">
      <a href="/">
        <div id="logo" class="ml-0 select-none">
          <div
            id="site-name"
            style="font-family: 'Segoe UI',Arial,'Microsoft Yahei',sans-serif;"
          >
            NoelOrin's Blog
          </div>
          <div class="absolute flex icon py-1.5">
            <svg
              fill="none"
              height="24"
              viewBox="0 0 48 48"
              width="24"
              xmlns="http://www.w3.org/2000/svg"
            >
              <path
                d="M44 44V20L24 4L4 20L4 44H16V26H32V44H44Z"
                fill="none"
                stroke="#ffffff"
                stroke-linejoin="round"
                stroke-width="5"
              />
            </svg>
          </div>
        </div>
      </a>
      {/* Popover content for mobile - shown on hover via CSS group */}
      <div
        class="hidden sm:flex space-x-[8px] xl:hidden select-none box-border items-center -mt-[6px] card absolute z-50 px-[10px] py-[6px]
               opacity-0 pointer-events-none group-hover/sitename:opacity-100 group-hover/sitename:pointer-events-auto
               transition-opacity duration-200"
        style="font-family:'Segoe UI',Arial,'Microsoft Yahei',sans-serif;"
      >
        <a href="/about" class="link fontExtraBold">关于</a>
        <a href="/friends" class="link fontExtraBold">友链</a>
        <a href="/contact" class="link fontExtraBold">联系我</a>
      </div>
    </div>
  )
}
