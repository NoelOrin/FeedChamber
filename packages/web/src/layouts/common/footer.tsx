export default function Footer() {
  return (
    <footer class="mt-4 flex w-full cursor-default items-center justify-center text-sm text-gray-500 dark:text-gray-400">
      <div class="flex w-full items-center p-4 lg:max-w-[1080px] xl:max-w-[1080px] 2xl:max-w-[1248px]">
        <div class="flex w-full flex-col">
          <div class="flex w-full sm:flex-col-reverse sm:items-center sm:space-y-[4px] md:space-y-0 sm:justify-center lg:flex-row lg:items-start lg:justify-between">
            <div class="flex flex-col sm:items-center lg:items-start">
              <div class="select-none">
                <span>Powered By </span>
                <span class="hover:text-[#7365E6] duration-200"><a href="https://zeabur.com" target="_blank" rel="noopener noreferrer">Zeabur</a></span>
                <span> | </span>
                <span class="hover:text-[#E36002] duration-200"><a href="https://hono.dev" target="_blank" rel="noopener noreferrer">Hono</a></span>
                <span> | </span>
                <span class="hover:text-[#00DC82] duration-200"><a href="https://nuxt.com" target="_blank" rel="noopener noreferrer">Nuxt</a></span>
                <span> | </span>
                <span class="hover:text-[#00BCE1] duration-200"><a href="https://tailwindcss.com" target="_blank" rel="noopener noreferrer">Tailwind</a></span>
              </div>
              <div class="mt-1 flex justify-around space-x-4">
                <a href="/contact" class="hover:underline">留言</a>
                <a href="/contact" class="hover:underline">联系我</a>
              </div>
            </div>
            <div class="flex flex-col sm:items-center lg:items-end sm:mb-[8px]">
              <div>NoelOrin.Top &copy; 2025</div>
              <div class="flex flex-col items-end space-y-1">
                <div class="flex sm:flex-col md:flex-row items-center whitespace-nowrap">
                  <span><span class="mx-1">NoelOrin</span><span>All rights reserved</span></span>
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