export default function Footer() {
  return (
    <footer class="mt-4 flex w-full cursor-default items-center justify-center text-sm text-remark">
      <div class="flex w-full items-center p-4 lg:max-w-[1080px] xl:max-w-[1080px] 2xl:max-w-[1248px]">
        <div class="flex w-full flex-col ">
          <div class="flex w-full sm:flex-col-reverse sm:items-center sm:space-y-[4px] md:space-y-0 sm:justify-center lg:flex-row lg:items-start lg:justify-between">
            <div class="flex flex-col sm:items-center lg:items-start">
              <div class="select-none footer_title">
                <span>Powered By </span>
                <span class="hover:text-[#7365E6] duration-200">
                  <a href="https://zeabur.com">Zeabur</a>
                </span>
                <span> | </span>
                <span class="hover:text-[#E36002] duration-200">
                  <a href="https://hono.dev">Hono</a>
                </span>
                <span> | </span>
                <span class="hover:text-[#00C777] duration-200">
                  <a href="https://nuxt.com">Nuxt</a>
                </span>
                <span> | </span>
                <span class="hover:text-[#00BCE1] duration-200">
                  <a href="https://tailwindcss.com">Tailwind</a>
                </span>
              </div>
              <div class="mt-1 flex justify-around space-x-4">
                <div>
                  <a href="/contact">留言</a>
                </div>
                <div>
                  <a href="/contact">联系我</a>
                </div>
              </div>
            </div>
            <div class="flex flex-col sm:items-center lg:items-end sm:mb-[8px]">
              <div class="footer_title">NoelOrin.Top © 2025</div>
              <div class="flex flex-col items-end space-y-1">
                <div class="flex sm:flex-col md:flex-row items-center whitespace-nowrap">
                  <span>
                    <span class="mx-1 footer_title">NoelOrin</span>
                    <span>All rights reserved</span>
                  </span>
                  <span class="mx-1 font-bold sm:hidden md:block"> | </span>
                  <span>本站原创内容未经许可禁止转载</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </footer>
  )
}
