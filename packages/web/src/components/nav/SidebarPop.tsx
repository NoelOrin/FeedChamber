import { Show } from 'solid-js'
import MyDesc from './MyDesc'
import SectionNav from './SectionNav'
import Friends from './Friends'

interface SidebarPopProps {
	isOpen: boolean
	onClose: () => void
	tocData?: import('./SectionNav').TocItem[]
}

export default function SidebarPop(props: SidebarPopProps) {
	return (
		<Show when={props.isOpen}>
			<div class="select-none" id="mySidebar">
				<div id="mask" class="active" onClick={props.onClose} />
				<aside id="sidebar-left" class="drawer-active sidebar-left-pop">
					<div class="-mt-2">
						<MyDesc />
					</div>
					<div class="p-[16px] pt-[20px]">
						<SectionNav tocData={props.tocData ?? []} />
					</div>
					<div class="p-[16px] pt-[20px]">
						<Friends />
					</div>
				</aside>
			</div>
		</Show>
	)
}
