import { useRouter } from '@tanstack/solid-router'

export default function ErrorComponent({ error }: { error: unknown }) {
  const router = useRouter()
  const message = error instanceof Error ? error.message : '发生了未知错误'

  return (
    <div class="flex h-full w-full flex-col items-center justify-center p-4 pb-8 card">
      <div class="flex h-full w-full flex-col items-center">
        <div class="mt-4 text-[72px] font-bold text-red-400">!</div>
        <div class="mt-4 text-[24px] font-bold">页面出错了</div>
        <div class="mt-2 text-[14px] text-remark max-w-100 text-center">{message}</div>
        <div class="mt-6 flex gap-4">
          <button
            type="button"
            class="px-4 py-2 rounded-lg btn text-[16px]"
            onClick={() => router.invalidate()}
          >
            重试
          </button>
          <a href="/" class="px-4 py-2 rounded-lg btn text-[16px] inline-block">
            返回首页
          </a>
        </div>
      </div>
    </div>
  )
}