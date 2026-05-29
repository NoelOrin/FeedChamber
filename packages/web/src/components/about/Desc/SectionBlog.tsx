export default function SectionBlog() {
  return (
    <div class="card mt-0 py-[18px] h-fit">
      <span class="text-[20px] fontExtraBold mb-[12px] flex justify-between items-end">
        <span>关于本站</span>
        <del class="text-[12px]">捏鼻</del>
      </span>
      <ul class="list-disc list-inside space-y-[8px] ml-[16px]">
        <li class="space-y-[8px]">
          <span class="font-semibold">为什么建站:</span>
          <ul class="list-disc list-inside space-y-[8px] ml-[16px]">
            <li>
              <span class="font-semibold">记录一些 → 经验分享 & 碎碎念 & 突发灵感 & 踩坑感悟</span>
            </li>
            <li>
              <span class="font-semibold">为了在建站过程用上一堆新技术</span>
            </li>
            <li>
              <span class="font-semibold">博客不是扣扣空间，也不是啥都发</span>
            </li>
          </ul>
        </li>
        <li class="space-y-[8px]">
          <span class="font-semibold">本站技术栈:</span>
          <ul class="list-disc list-inside space-y-[8px] ml-[16px]">
            <li>
              <span class="font-semibold">前端:</span> Nuxt4 + TailwindCSS + ArcoDesign + Gsap
            </li>
            <li>
              <span class="font-semibold">后台:</span> SoybeanAdmin Vue3 + UnoCSS + ArcoDesign + Biome
            </li>
            <li>
              <span class="font-semibold">后端:</span> Hono + Prisma + SQLite + MongoDB + TS + Biome
            </li>
            <li>
              <span class="font-semibold">Devops & Deploy:</span> Github Action + Docker + Zeabur
            </li>
          </ul>
        </li>
        <li>
          <span class="font-semibold">换友链的朋友可以在最下边评论区Call一下</span>
        </li>
      </ul>
    </div>
  )
}
