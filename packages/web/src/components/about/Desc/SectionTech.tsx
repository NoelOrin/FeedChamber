import DescCard from './DescCard'

export default function SectionTech() {
  return (
    <DescCard title="Tech Stack">
      <div class="flex flex-col items-center space-y-[12px] text-center max-w-[600px]">
        <div class="flex flex-wrap justify-center gap-[12px]">
          <span class="px-[12px] py-[6px] rounded-lg bg-blue-500/10 text-blue-600 dark:text-blue-400 font-semibold">
            Vue.js
          </span>
          <span class="px-[12px] py-[6px] rounded-lg bg-cyan-500/10 text-cyan-600 dark:text-cyan-400 font-semibold">
            Nuxt
          </span>
          <span class="px-[12px] py-[6px] rounded-lg bg-sky-500/10 text-sky-600 dark:text-sky-400 font-semibold">
            SolidJS
          </span>
          <span class="px-[12px] py-[6px] rounded-lg bg-purple-500/10 text-purple-600 dark:text-purple-400 font-semibold">
            TypeScript
          </span>
          <span class="px-[12px] py-[6px] rounded-lg bg-teal-500/10 text-teal-600 dark:text-teal-400 font-semibold">
            Tailwind CSS
          </span>
          <span class="px-[12px] py-[6px] rounded-lg bg-green-500/10 text-green-600 dark:text-green-400 font-semibold">
            Node.js
          </span>
        </div>
        <p class="text-[14px] text-gray-600 dark:text-gray-400 leading-relaxed">
          Frontend developer focused on modern web technologies.
        </p>
      </div>
    </DescCard>
  )
}
