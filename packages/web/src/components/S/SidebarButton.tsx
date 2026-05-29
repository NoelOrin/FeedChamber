import { Settings } from 'lucide-solid'
import { useTheme } from '~/utils/theme'

export default function SidebarButton() {
	const { themeMode, toggleTheme } = useTheme()

	return (
		<div class="flex flex-row items-center gap-2">
			<button
				class="flex items-center justify-center rounded-full p-2 cursor-pointer hover:bg-gray-200 dark:hover:bg-zinc-700 transition-colors"
				onClick={toggleTheme}
				title={themeMode() === 'dark' ? '切换亮色模式' : '切换暗色模式'}
			>
				{themeMode() === 'dark' ? (
					<svg width="20" height="20" viewBox="0 0 48 48" fill="none" xmlns="http://www.w3.org/2000/svg">
						<path d="M24 37C31.1797 37 37 31.1797 37 24C37 16.8203 31.1797 11 24 11C16.8203 11 11 16.8203 11 24C11 31.1797 16.8203 37 24 37Z" fill="none" stroke="currentColor" stroke-linejoin="round" stroke-width="4" />
						<path d="M24 6C25.3807 6 26.5 4.88071 26.5 3.5C26.5 2.11929 25.3807 1 24 1C22.6193 1 21.5 2.11929 21.5 3.5C21.5 4.88071 22.6193 6 24 6Z" fill="currentColor" />
					</svg>
				) : (
					<svg width="20" height="20" viewBox="0 0 48 48" fill="none" xmlns="http://www.w3.org/2000/svg">
						<path d="M28.0527 4.41085C22.5828 5.83695 18.5455 10.8106 18.5455 16.7273C18.5455 23.7564 24.2436 29.4545 31.2727 29.4545C37.1894 29.4545 42.1631 25.4172 43.5891 19.9473C43.8585 21.256 44 22.6115 44 24C44 35.0457 35.0457 44 24 44C12.9543 44 4 35.0457 4 24C4 12.9543 12.9543 4 24 4C25.3885 4 26.744 4.14149 28.0527 4.41085Z" fill="none" stroke="currentColor" stroke-linejoin="round" stroke-width="4" />
					</svg>
				)}
			</button>
			<button
				class="flex items-center justify-center rounded-full p-2 cursor-pointer hover:bg-gray-200 dark:hover:bg-zinc-700 transition-colors"
				title="设置"
			>
				<Settings size={20} />
			</button>
		</div>
	)
}
