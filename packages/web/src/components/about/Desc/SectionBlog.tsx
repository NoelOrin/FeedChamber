import DescCard from './DescCard'

export default function SectionBlog() {
  return (
    <DescCard title="Blog">
      <div class="flex flex-col items-center space-y-[12px] text-center max-w-[600px]">
        <p class="text-[14px] text-gray-600 dark:text-gray-400 leading-relaxed">
          Writing about frontend development, DevOps, and self-hosting.
        </p>
        <div class="flex flex-wrap justify-center gap-[16px] mt-[8px]">
          <div class="flex flex-col items-center">
            <span class="text-[28px] font-bold text-themeColorMain">50+</span>
            <span class="text-[12px] text-gray-500">Posts</span>
          </div>
          <div class="flex flex-col items-center">
            <span class="text-[28px] font-bold text-themeColorMain">10k+</span>
            <span class="text-[12px] text-gray-500">Views</span>
          </div>
          <div class="flex flex-col items-center">
            <span class="text-[28px] font-bold text-themeColorMain">5+</span>
            <span class="text-[12px] text-gray-500">Tags</span>
          </div>
        </div>
      </div>
    </DescCard>
  )
}
