'use client';

import { postService } from '@orinforum/api-client';
import { useQuery } from '@tanstack/react-query';
import Link from 'next/link';
import type * as React from 'react';
import { useEffect, useMemo, useRef, useState } from 'react';

import { Button } from '~/components/ui/button';
import { Search, X } from '~/lib/icons';
import { qk } from '~/lib/query/keys';
import { cn } from '~/lib/utils';

const RECENT_KEY = 'search_recent';
const RECENT_LIMIT = 5;
const SEARCH_PAGE_SIZE = 10;
const DEBOUNCE_MS = 300;

function readRecentQueries(): string[] {
	if (typeof localStorage === 'undefined') return [];
	try {
		const raw = localStorage.getItem(RECENT_KEY);
		const parsed = raw ? JSON.parse(raw) : [];
		return Array.isArray(parsed) ? parsed.filter((v: unknown) => typeof v === 'string').slice(0, RECENT_LIMIT) : [];
	} catch {
		return [];
	}
}

function saveRecentQueries(items: string[]) {
	if (typeof localStorage === 'undefined') return;
	try {
		localStorage.setItem(RECENT_KEY, JSON.stringify(items.slice(0, RECENT_LIMIT)));
	} catch {
		/* 隐私模式写不进去不影响搜索 */
	}
}

/** 命中片段高亮：截取 ±(40/90) 字符窗口，所有命中处都标 <mark>。 */
function Highlight({ text, query }: { text: string; query: string }) {
	if (!query) return <>{text}</>;

	const lowerText = text.toLowerCase();
	const lowerQuery = query.toLowerCase();
	const first = lowerText.indexOf(lowerQuery);
	if (first < 0) return <>{text}</>;

	const start = Math.max(0, first - 40);
	const end = Math.min(text.length, first + query.length + 90);
	const snippet = `${start > 0 ? '...' : ''}${text.slice(start, end)}${end < text.length ? '...' : ''}`;

	/* 按匹配起始位置定 key，避免用数组下标 */
	const nodes: React.ReactNode[] = [];
	const lowerSnippet = snippet.toLowerCase();
	let cursor = 0;
	let at = lowerSnippet.indexOf(lowerQuery);
	while (at >= 0) {
		if (at > cursor) nodes.push(<span key={`t${cursor}`}>{snippet.slice(cursor, at)}</span>);
		nodes.push(
			<mark key={`m${at}`} className="rounded bg-warning/20 px-0.5 text-foreground">
				{snippet.slice(at, at + query.length)}
			</mark>,
		);
		cursor = at + query.length;
		at = lowerSnippet.indexOf(lowerQuery, cursor);
	}
	if (cursor < snippet.length) nodes.push(<span key={`t${cursor}`}>{snippet.slice(cursor)}</span>);

	return <>{nodes}</>;
}

/** 全局检索浮层：Nav 里按 `/` 唤起，Esc 与遮罩点击关闭。 */
export function SearchOverlay({ onClose }: { onClose: () => void }) {
	const [query, setQuery] = useState('');
	const [debounced, setDebounced] = useState('');
	const [recent, setRecent] = useState<string[]>([]);
	const inputRef = useRef<HTMLInputElement>(null);
	const overlayRef = useRef<HTMLDivElement>(null);

	/* 首次挂载聚焦输入框并回填历史（localStorage 只能在客户端读） */
	useEffect(() => {
		inputRef.current?.focus();
		setRecent(readRecentQueries());
	}, []);

	useEffect(() => {
		const handleKeyDown = (e: KeyboardEvent) => {
			if (e.key === 'Escape') onClose();
		};
		document.addEventListener('keydown', handleKeyDown);
		return () => document.removeEventListener('keydown', handleKeyDown);
	}, [onClose]);

	/* 输入防抖交给 effect 托管，定时器随依赖变化自动清理 */
	useEffect(() => {
		const trimmed = query.trim();
		if (!trimmed) {
			setDebounced('');
			return;
		}
		const timer = setTimeout(() => setDebounced(trimmed), DEBOUNCE_MS);
		return () => clearTimeout(timer);
	}, [query]);

	const search = useMemo(() => ({ q: debounced, page: 1, page_size: SEARCH_PAGE_SIZE }), [debounced]);

	const { data, isFetching } = useQuery({
		queryKey: qk.search(search),
		queryFn: () => postService.search(search),
		enabled: debounced.length > 0,
	});

	const results = useMemo(() => data?.data ?? [], [data]);

	useEffect(() => {
		if (!debounced || !data) return;
		const next = [debounced, ...readRecentQueries().filter((v) => v !== debounced)];
		setRecent((prev) => (prev.join('\n') === next.join('\n') ? prev : next));
		saveRecentQueries(next);
	}, [debounced, data]);

	return (
		<div ref={overlayRef} className="fixed inset-0 z-[100] flex items-start justify-center px-4 pt-[15vh]">
			<button
				type="button"
				aria-label="关闭搜索"
				onClick={onClose}
				className="absolute inset-0 cursor-default bg-black/45"
			/>
			<div className="relative w-full max-w-xl overflow-hidden rounded-lg border border-border bg-popover text-popover-foreground shadow-lg">
				<div className="flex h-12 items-center gap-2 border-b border-border px-3">
					<Search className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
					<input
						ref={inputRef}
						type="text"
						placeholder="搜索帖子..."
						aria-label="搜索帖子"
						className="h-full w-full bg-transparent text-sm outline-none placeholder:text-muted-foreground"
						value={query}
						onChange={(e) => setQuery(e.target.value)}
					/>
					<Button variant="ghost" size="icon" className="shrink-0" onClick={onClose} aria-label="关闭">
						<X className="size-4" aria-hidden="true" />
					</Button>
				</div>

				{!query && recent.length > 0 ? (
					<div className="flex flex-wrap gap-1.5 border-b border-border px-3 py-2.5">
						<span className="self-center text-[11px] text-muted-foreground">最近</span>
						{recent.map((q) => (
							<button
								key={q}
								type="button"
								className="cursor-pointer rounded-md border border-border px-1.5 py-0.5 text-[11px] text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
								onClick={() => {
									setQuery(q);
									setDebounced(q.trim());
								}}
							>
								{q}
							</button>
						))}
					</div>
				) : null}

				<div className="max-h-[50vh] overflow-y-auto p-1.5">
					{debounced && isFetching ? (
						<div className="px-2 py-6 text-center text-xs text-muted-foreground">搜索中...</div>
					) : null}

					{debounced && !isFetching && results.length === 0 ? (
						<div className="px-2 py-6 text-center text-xs text-muted-foreground">未找到相关结果</div>
					) : null}

					{results.map((item) => (
						<Link
							key={item.id}
							href={`/t/${item.slug || 'post'}/${item.id}`}
							onClick={onClose}
							className={cn(
								'block rounded-md px-2 py-2 transition-colors hover:bg-muted',
								'focus-visible:bg-muted focus-visible:outline-none',
							)}
						>
							<div className="text-[13px] font-medium text-foreground">
								<Highlight text={item.title} query={query} />
							</div>
							<div className="mt-0.5 line-clamp-2 text-xs text-muted-foreground">
								<Highlight text={item.content_digest ?? ''} query={query} />
							</div>
						</Link>
					))}
				</div>

				<div className="flex items-center justify-between border-t border-border px-3 py-2">
					<span className="text-[11px] text-muted-foreground">Enter 打开 · Esc 关闭</span>
					<Button variant="ghost" size="xs" asChild>
						<Link href={`/search?q=${encodeURIComponent(query.trim())}&page=1`} onClick={onClose}>
							完整搜索页
						</Link>
					</Button>
				</div>
			</div>
		</div>
	);
}
