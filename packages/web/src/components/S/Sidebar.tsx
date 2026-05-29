import MyDesc from './MyDesc'
import SectionNav from './SectionNav'
import Friends from './Friends'

interface SidebarProps {
	tocData?: import('./SectionNav').TocItem[]
}

export default function Sidebar(props: SidebarProps) {
	return (
		<aside id="sidebar-left">
			<MyDesc />
			<div class="sticky top-12 flex flex-col">
				{props.tocData && props.tocData.length > 0 && (
					<div class="p-[16px] pt-[20px]">
						<SectionNav tocData={props.tocData} />
					</div>
				)}
				<div class="p-[16px] pt-[20px]">
					<Friends />
				</div>
			</div>
		</aside>
	)
}
