import { For, Show } from 'solid-js'
import type { TocItem } from './SectionNav'

interface SectionRecursionProps {
	items: TocItem[]
}

function scrollToSection(id: string) {
	const targetElement = document.getElementById(id)
	if (targetElement) {
		const offsetTop = targetElement.offsetTop - 75
		window.scrollTo({ top: offsetTop, behavior: 'smooth' })
	}
}

function SectionRecursionInner(props: SectionRecursionProps) {
	return (
		<Show when={props.items && props.items.length > 0}>
			<ul class="h-full">
				<For each={props.items}>
					{(item) => (
						<li>
							<span
								class="flex w-full cursor-pointer flex-row items-center justify-between truncate px-2 py-1 sectionNavItem my-0.5 ml-1.5"
								title={item.text}
								onClick={() => scrollToSection(item.id)}
							>
								{item.text}
							</span>
							<Show when={item.children && item.children.length > 0}>
								<SectionRecursionInner items={item.children!} />
							</Show>
						</li>
					)}
				</For>
			</ul>
		</Show>
	)
}

export default SectionRecursionInner
