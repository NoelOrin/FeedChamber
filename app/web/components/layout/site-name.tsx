import Link from 'next/link';

import { MessageSquare } from '~/lib/icons';

/* 字标：实心主色方块 + 名称。 */
export function SiteName({ settings }: { settings: Record<string, string> }) {
	const siteName = settings.site_name || 'OrinForum';
	const siteLogo = settings.site_logo || '';

	return (
		<Link href="/" className="inline-flex items-center gap-2 transition-opacity hover:opacity-80">
			<span className="grid size-7 shrink-0 place-items-center rounded-md bg-primary text-primary-foreground">
				{siteLogo ? (
					<img src={siteLogo} alt="" className="size-7 rounded-md object-cover" />
				) : (
					<MessageSquare size={15} aria-hidden="true" />
				)}
			</span>
			<span className="truncate text-sm font-semibold tracking-tight text-foreground">{siteName}</span>
		</Link>
	);
}
