'use client';

import { themeService } from '@orinforum/api-client';
import type * as React from 'react';

import { authSession } from '~/lib/auth';
import { Monitor, Moon, Sun } from '~/lib/icons';
import { useTheme } from '~/lib/theme';

/**
 * 三态主题切换（亮 / 暗 / 跟随系统），带 View Transition。
 * 图标显隐交给 globals.css 的 [data-theme-icon] 规则——它由首屏内联脚本落到
 * <html data-theme-mode> 上决定，SSR/水合两帧都一致，故不依赖 React 的 mode。
 */
export function ThemeToggle() {
	const { title, cycle } = useTheme();

	function handleClick(event: React.MouseEvent<HTMLButtonElement>) {
		cycle(event.nativeEvent, (mode) => {
			// 登录后才回写服务端偏好（旧 Nav 无条件写，匿名会白挨一次 401）
			if (authSession.isAuthenticated) {
				themeService.setPreference(mode).catch(() => {});
			}
		});
	}

	return (
		<button
			type="button"
			title={title}
			aria-label={title}
			data-id="theme-btn"
			onClick={handleClick}
			className="tap-target inline-flex size-8 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/80 focus-visible:outline-none"
		>
			<span data-theme-icon="light" className="hidden">
				<Sun size={16} aria-hidden="true" />
			</span>
			<span data-theme-icon="dark" className="hidden">
				<Moon size={16} aria-hidden="true" />
			</span>
			<span data-theme-icon="system" className="hidden">
				<Monitor size={16} aria-hidden="true" />
			</span>
			<Sun size={16} aria-hidden="true" className="theme-fallback-icon" />
		</button>
	);
}
