import { createSignal, onMount, onCleanup } from 'solid-js'
import { Sun, Settings } from 'lucide-solid'

export default function SidebarButton() {
	const [isDark, setIsDark] = createSignal(false)

	onMount(() => {
		setIsDark(document.documentElement.classList.contains('dark'))
	})

	function toggleTheme() {
		const next = !isDark()
		setIsDark(next)
		if (next) {
			document.documentElement.classList.add('dark')
		} else {
			document.documentElement.classList.remove('dark')
		}
	}

	return (
		<div class="flex flex-row items-center gap-2">
			<button
				class="flex items-center justify-center rounded-full p-2 cursor-pointer hover:bg-gray-200 dark:hover:bg-zinc-700 transition-colors"
				onClick={toggleTheme}
				title={isDark() ? 'Switch to light mode' : 'Switch to dark mode'}
			>
				<Sun size={20} class={isDark() ? 'block' : 'hidden'} />
				<span class={isDark() ? 'hidden' : 'block'}>
					<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
						<path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
					</svg>
				</span>
			</button>
			<button
				class="flex items-center justify-center rounded-full p-2 cursor-pointer hover:bg-gray-200 dark:hover:bg-zinc-700 transition-colors"
				title="Settings"
			>
				<Settings size={20} />
			</button>
		</div>
	)
}
