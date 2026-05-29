import { createSignal, onMount, onCleanup, For } from 'solid-js'

interface FloatNavigatorProps {
  sectionIds: string[]
  excludeId: string[]
}

export default function FloatNavigator(props: FloatNavigatorProps) {
  const [activeSection, setActiveSection] = createSignal('')
  const [isShow, setIsShow] = createSignal(true)

  let rafId: number | null = null

  const TRIGGER_TOP = window.innerHeight * 0.1
  const TRIGGER_BOTTOM = window.innerHeight * 1.1

  const updateActiveSection = () => {
    let currentActive = ''
    for (const id of props.sectionIds) {
      const el = document.getElementById(id)
      if (!el) continue
      const rect = el.getBoundingClientRect()
      const elementCenter = rect.top + rect.height / 2
      if (elementCenter >= TRIGGER_TOP && elementCenter <= TRIGGER_BOTTOM) {
        setIsShow(!props.excludeId.includes(id))
        currentActive = id
        break
      }
    }
    if (currentActive !== activeSection()) {
      setActiveSection(currentActive)
    }
    rafId = requestAnimationFrame(updateActiveSection)
  }

  onMount(() => {
    rafId = requestAnimationFrame(updateActiveSection)
  })

  onCleanup(() => {
    if (rafId !== null) cancelAnimationFrame(rafId)
  })

  return (
    <div
      class="flex flex-col justify-center fixed sm:top-[20%] lg:top-1/2 -translate-y-1/2 sm:left-[8px] lg:left-[16px] z-50 sm:space-y-[24px] lg:space-y-[16px] select-none fontExtraBold"
      style={{ display: isShow() ? 'flex' : 'none' }}
    >
      <For each={props.sectionIds}>
        {(id) => (
          <div class="flex h-fit w-fit flex-1 justify-center">
            <a
              href={props.excludeId.includes(id) ? undefined : `#${id}`}
              class={`hover:text-themeColorMain transition-colors duration-200 ${
                activeSection() === id ? 'text-themeColorMain' : ''
              }`}
            >
              {props.excludeId.includes(id) ? '' : id}
            </a>
          </div>
        )}
      </For>
    </div>
  )
}
