'use client';

import { themeService } from '@feedchamber/api-client';
import { lazy, Suspense, useEffect, useState } from 'react';

import { NotificationBell } from '~/components/layout/notification-bell';
import { SiteName } from '~/components/layout/site-name';
import { ThemeToggle } from '~/components/layout/theme-toggle';
import { UserInfo } from '~/components/layout/user-info';
import { Button } from '~/components/ui/button';
import { Dialog, DialogClose, DialogContent, DialogHeader, DialogTitle } from '~/components/ui/dialog';
import { useAuthStore } from '~/lib/auth';
import { Menu, Search as SearchIcon } from '~/lib/icons';
import { syncThemeFromServer } from '~/lib/theme';

const SearchOverlay = lazy(() =>
	import('~/components/layout/search-overlay').then((mod) => ({ default: mod.SearchOverlay })),
);

interface NavProps {
	settings: Record<string, string>;
	onToggleSidebar?: () => void;
}

const iconButton =
	'tap-target inline-flex size-8 shrink-0 cursor-pointer items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/80 focus-visible:outline-none';

export function Nav({ settings, onToggleSidebar }: NavProps) {
	const [searchVisible, setSearchVisible] = useState(false);
	const [shortcutHelpVisible, setShortcutHelpVisible] = useState(false);

	useEffect(() => {
		const onKeyDown = (event: KeyboardEvent) => {
			const target = event.target as HTMLElement | null;
			const typing =
				target && (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable);
			if (typing) return;
			if (event.key === '/') {
				event.preventDefault();
				setSearchVisible(true);
			} else if (event.key === '?') {
				event.preventDefault();
				setShortcutHelpVisible((open) => !open);
			}
		};
		document.addEventListener('keydown', onKeyDown);
		return () => document.removeEventListener('keydown', onKeyDown);
	}, []);

	// 服务端主题偏好回灌本地，失败静默
	const user = useAuthStore((state) => state.user);
	useEffect(() => {
		if (!user) return;
		themeService
			.getPreference()
			.then((res) => syncThemeFromServer(res?.theme))
			.catch(() => {});
	}, [user]);

	return (
		<div>
			<header className="sticky top-0 z-30 w-full border-b border-border bg-background/95 backdrop-blur">
				<div className="flex h-14 w-full items-center gap-1 px-4">
					<button type="button" className={iconButton} aria-label="菜单" onClick={onToggleSidebar}>
						<Menu size={17} aria-hidden="true" />
					</button>
					<SiteName settings={settings} />

					<div className="ml-auto flex shrink-0 items-center gap-1">
						<button
							type="button"
							className={iconButton}
							onClick={() => setSearchVisible(true)}
							aria-label="搜索（/）"
							title="搜索（/）"
						>
							<SearchIcon size={16} aria-hidden="true" />
						</button>
						<NotificationBell />
						<ThemeToggle />
						<span className="mx-1 hidden h-4 w-px bg-border sm:block" aria-hidden="true" />
						<UserInfo />
					</div>
				</div>
			</header>

			{searchVisible ? (
				<Suspense fallback={null}>
					<SearchOverlay onClose={() => setSearchVisible(false)} />
				</Suspense>
			) : null}

			<Dialog open={shortcutHelpVisible} onOpenChange={setShortcutHelpVisible}>
				<DialogContent className="max-w-sm">
					<DialogHeader>
						<DialogTitle>快捷键</DialogTitle>
					</DialogHeader>
					<div className="flex flex-col gap-1">
						{[
							['/', '打开搜索'],
							['?', '快捷键帮助'],
							['Esc', '关闭弹层'],
						].map(([key, label]) => (
							<div key={key} className="flex items-center justify-between py-1">
								<kbd className="rounded border border-border bg-muted px-1.5 py-0.5 font-mono text-[11px] text-foreground">
									{key}
								</kbd>
								<span className="text-xs text-muted-foreground">{label}</span>
							</div>
						))}
					</div>
					<DialogClose asChild>
						<Button variant="outline" size="sm" className="self-end">
							关闭
						</Button>
					</DialogClose>
				</DialogContent>
			</Dialog>
		</div>
	);
}
