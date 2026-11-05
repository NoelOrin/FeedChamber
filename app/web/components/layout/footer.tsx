import { PublicSlot } from '~/components/slot/slot-host';

/* 页脚只留必要信息。 */
export function Footer({ settings }: { settings: Record<string, string> }) {
	const siteName = settings.site_name || 'Feed Chamber';
	const year = String(new Date().getFullYear());

	return (
		<footer className="mt-12 w-full px-4 pb-8">
			<div className="flex flex-wrap items-center justify-between gap-3 border-t border-border pt-5 text-xs text-muted-foreground">
				<span className="flex items-center gap-2">
					{siteName} © {year}
					<PublicSlot name="footer.left" />
				</span>
				<span className="flex items-center gap-3">
					<span>社区讨论</span>
					<span aria-hidden="true" className="size-1 rounded-full bg-border" />
					<span>开放注册</span>
					<PublicSlot name="footer.right" />
				</span>
			</div>
		</footer>
	);
}
