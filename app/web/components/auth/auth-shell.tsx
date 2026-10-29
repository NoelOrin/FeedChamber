import Link from 'next/link';

const CURRENT_YEAR = new Date().getFullYear();

/** 第三方登录 / 回调共用的居中卡片壳。 */
export function AuthShell({
	siteName,
	title,
	subtitle,
	icon,
	footer,
	children,
}: {
	siteName: string;
	title: string;
	subtitle?: string;
	icon?: React.ReactNode;
	footer?: React.ReactNode;
	children: React.ReactNode;
}) {
	return (
		<div className="flex min-h-screen items-center justify-center px-4 py-10">
			<div className="w-full max-w-sm">
				<Link
					href="/"
					className="mb-6 block text-center text-sm font-semibold tracking-tight text-foreground transition-opacity hover:opacity-80"
				>
					{siteName}
				</Link>
				<div className="rounded-lg border border-border bg-card p-6">
					<div className="mb-5 text-center">
						<div className="flex items-center justify-center gap-2">
							{icon}
							<h1 className="text-lg font-semibold tracking-tight">{title}</h1>
						</div>
						{subtitle ? <p className="mt-1 text-[13px] text-muted-foreground">{subtitle}</p> : null}
					</div>
					{children}
					{footer ? <p className="mt-5 text-center text-xs text-muted-foreground">{footer}</p> : null}
				</div>
				<p className="mt-6 text-center text-xs text-muted-foreground">
					&copy; {CURRENT_YEAR} {siteName}
				</p>
			</div>
		</div>
	);
}
