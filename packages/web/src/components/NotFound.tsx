export default function NotFound() {
  return (
    <div class="flex h-full w-full flex-col items-center p-4 pb-8 card">
      <div class="flex h-full w-full flex-col items-center">
        <img
          alt="404 NOT FOUND"
          class="object-cover object-center"
          src="https://oss.noelorin.top/404.png"
        />
        <div class="mt-4 text-[24px] font-bold">404~ 找不到页面咯</div>
        <a href="/" class="mt-4 text-[18px] hover:underline">返回首页</a>
      </div>
    </div>
  )
}
