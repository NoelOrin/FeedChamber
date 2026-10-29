'use client';

import type * as React from 'react';

import { Sheet, SheetContent } from '~/components/ui/sheet';

interface SidebarProps {
	desktopCollapsed: boolean;
	mobileOpen: boolean;
	onClose: () => void;
	children?: React.ReactNode;
}

/**
 * 左侧栏：≥1280px（xl）常驻显示，桌面收起时整体不渲染；
 * <1280px 交给 Sheet(side="left") 抽屉承载，由 mobileOpen 驱动 open。
 */
export function Sidebar({ desktopCollapsed, mobileOpen, onClose, children }: SidebarProps) {
	return (
		<>
			{!desktopCollapsed ? <aside className="hidden w-60 min-w-0 shrink-0 xl:block">{children}</aside> : null}

			<Sheet
				open={mobileOpen}
				onOpenChange={(open) => {
					if (!open) onClose();
				}}
			>
				<SheetContent side="left" aria-label="侧边栏导航" className="w-64 select-none gap-2 overflow-y-auto p-3">
					{children}
				</SheetContent>
			</Sheet>
		</>
	);
}
