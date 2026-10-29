'use client';

import type * as React from 'react';
import { useState } from 'react';

import { Sidebar } from '~/components/aside/sidebar';
import { SidebarContent } from '~/components/aside/sidebar-content';
import { AnnouncementBanner, AnnouncementOverlay } from '~/components/layout/announcement-overlay';
import { BackToTop } from '~/components/layout/back-to-top';
import { Footer } from '~/components/layout/footer';
import { Nav } from '~/components/layout/nav';
import { t } from '~/lib/i18n';

/* 骨架：顶栏 + 左侧目录 + 主列（三栏仍是容器内布局，视觉换成描边语言）。 */
export function AppShell({ settings, children }: { settings: Record<string, string>; children: React.ReactNode }) {
	const [desktopCollapsed, setDesktopCollapsed] = useState(false);
	const [mobileOpen, setMobileOpen] = useState(false);

	const toggleSidebar = () => {
		if (window.innerWidth >= 1280) setDesktopCollapsed((value) => !value);
		else setMobileOpen((value) => !value);
	};

	return (
		<>
			<a
				className="sr-only focus:not-sr-only focus:fixed focus:top-2 focus:left-2 focus:z-[120] focus:rounded-md focus:bg-primary focus:px-3 focus:py-1.5 focus:text-xs focus:text-primary-foreground"
				href="#main-content"
			>
				{t('skip_to_content')}
			</a>
			<Nav settings={settings} onToggleSidebar={toggleSidebar} />
			{/* 公告横幅在文档流里占位：不再用 fixed 浮层压住页头 */}
			<AnnouncementBanner />
			<div className="w-full px-4 py-6">
				<div className="flex w-full items-start gap-8">
					<Sidebar desktopCollapsed={desktopCollapsed} mobileOpen={mobileOpen} onClose={() => setMobileOpen(false)}>
						<SidebarContent />
					</Sidebar>
					<main id="main-content" tabIndex={-1} className="min-w-0 flex-1 outline-none">
						{children}
					</main>
				</div>
			</div>
			<BackToTop />
			<Footer settings={settings} />
			<AnnouncementOverlay />
		</>
	);
}
