import { Show, type JSX } from 'solid-js'
import MyDesc from './MyDesc'
import SectionNav from './SectionNav'
import Friends from './Friends'

interface SidebarProps {
  isOpen?: boolean
  onClose?: () => void
  tocData?: import('./SectionNav').TocItem[]
  children?: JSX.Element
}

function SidebarContent(props: { tocData?: import('./SectionNav').TocItem[]; children?: JSX.Element }) {
  return (
    <>
      <MyDesc />
      <div class="sticky top-12 flex flex-col">
        <Show when={props.tocData && props.tocData.length > 0}>
          <div class="p-[16px] pt-[20px]">
            <SectionNav tocData={props.tocData!} />
          </div>
        </Show>
        {props.children}
        <div class="p-[16px] pt-[20px]">
          <Friends />
        </div>
      </div>
    </>
  )
}

export default function Sidebar(props: SidebarProps) {
  return (
    <>
      {/* Desktop */}
      <aside id="sidebar-left" class="hidden xl:block">
        <SidebarContent tocData={props.tocData}>
          {props.children}
        </SidebarContent>
      </aside>

      {/* Mobile */}
      <Show when={props.isOpen}>
        <div class="select-none" id="mySidebar">
          <div id="mask" class="active" onClick={props.onClose} />
          <aside id="sidebar-left" class="drawer-active sidebar-left-pop">
            <div class="-mt-2">
              <SidebarContent tocData={props.tocData}>
                {props.children}
              </SidebarContent>
            </div>
          </aside>
        </div>
      </Show>
    </>
  )
}
