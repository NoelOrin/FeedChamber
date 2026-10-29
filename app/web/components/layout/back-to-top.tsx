'use client';

import { useEffect, useState } from 'react';

import { ChevronDown } from '~/lib/icons';
import { cn } from '~/lib/utils';

export function BackToTop() {
	const [shouldShowBackToTop, setShouldShowBackToTop] = useState(false);

	// 滚动事件按 30ms 去抖，避免每帧 setState
	useEffect(() => {
		let scrollTimeout: ReturnType<typeof setTimeout> | null = null;

		const handleScroll = () => {
			if (scrollTimeout) clearTimeout(scrollTimeout);
			scrollTimeout = setTimeout(() => {
				setShouldShowBackToTop(window.scrollY > 400);
			}, 30);
		};

		window.addEventListener('scroll', handleScroll);
		return () => {
			window.removeEventListener('scroll', handleScroll);
			if (scrollTimeout) clearTimeout(scrollTimeout);
		};
	}, []);

	const scrollToTop = () => {
		window.scrollTo({ top: 0, behavior: 'smooth' });
	};

	if (!shouldShowBackToTop) return null;

	return (
		<button
			type="button"
			onClick={scrollToTop}
			aria-label="回到顶部"
			title="回到顶部"
			className={cn(
				'fixed right-4 bottom-6 z-40 grid size-9 cursor-pointer place-items-center rounded-full border border-border bg-background text-muted-foreground shadow-sm transition-colors sm:right-6',
				'tap-target hover:bg-muted hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/80 focus-visible:outline-none',
			)}
		>
			<ChevronDown className="size-4 rotate-180" aria-hidden="true" />
		</button>
	);
}
