'use client';

import { useQuery } from '@tanstack/react-query';
import { parseAsInteger, parseAsString, useQueryState } from 'nuqs';
import { type FormEvent, useEffect, useState } from 'react';

import { Pager } from '~/components/pagination/pager';
import { normalizePage, SEARCH_PAGE_SIZE, searchPostsOptions } from '~/components/search/queries';
import { SearchResultCard } from '~/components/search/search-result-card';
import { Button } from '~/components/ui/button';
import { EmptyState } from '~/components/ui/empty-state';
import { Input } from '~/components/ui/input';
import { PageHeader } from '~/components/ui/page-header';
import { Skeleton } from '~/components/ui/skeleton';
import { Search as SearchIcon } from '~/lib/icons';

/* URL 是这套检索的唯一状态源（旧 validateSearch 的 q/page 契约）：
 `/search?q=&page=1` 必须原样可读，因此两个参数都写回、不做默认值清空。 */
const NO_CLEAR = { clearOnDefault: false } as const;

export function SearchPanel() {
	const [q, setQ] = useQueryState('q', parseAsString.withDefault('').withOptions(NO_CLEAR));
	const [page, setPage] = useQueryState('page', parseAsInteger.withDefault(1).withOptions(NO_CLEAR));
	const [draft, setDraft] = useState(q);

	/* 外部导航（Nav 的 /search?q=…、浏览器前进后退）改写 URL 时回填输入框 */
	useEffect(() => {
		setDraft(q);
	}, [q]);

	const currentPage = normalizePage(page);
	const { data, isPending } = useQuery(searchPostsOptions(q, currentPage));
	const items = data?.data ?? [];
	const total = data?.total ?? 0;
	const totalPages = Math.max(1, Math.ceil(total / SEARCH_PAGE_SIZE));

	const submit = (event: FormEvent) => {
		event.preventDefault();
		void setQ(draft.trim());
		void setPage(1);
	};

	return (
		<div className="flex w-full flex-col gap-3 pb-10">
			<PageHeader title="搜索" meta={q ? `${total} 条命中` : undefined} />

			<form onSubmit={submit} className="flex flex-col gap-2 sm:flex-row">
				<div className="relative flex-1">
					<SearchIcon
						size={15}
						aria-hidden="true"
						className="pointer-events-none absolute top-1/2 left-3 -translate-y-1/2 text-muted-foreground"
					/>
					<Input
						type="text"
						value={draft}
						onChange={(event) => setDraft(event.target.value)}
						placeholder="搜索标题、摘要与正文…"
						aria-label="搜索标题、摘要与正文"
						className="pl-9"
					/>
				</div>
				<Button type="submit" className="shrink-0">
					搜索
				</Button>
			</form>

			{q && isPending ? (
				<div className="flex flex-col gap-2">
					<Skeleton className="h-24 w-full" />
					<Skeleton className="h-24 w-full" />
				</div>
			) : items.length === 0 ? (
				<EmptyState
					variant="search"
					title={q ? '没有找到结果' : '输入关键词开始搜索'}
					description={q ? '换个关键词再试。' : '例如：前端 / 微服务 / 部署'}
				/>
			) : (
				<div className="flex flex-col gap-2">
					{items.map((post) => (
						<SearchResultCard key={post.id} post={post} />
					))}
				</div>
			)}

			<Pager
				currentPage={currentPage}
				totalPages={totalPages}
				onPageChange={(next) => void setPage(next)}
				className="mt-2"
			/>
		</div>
	);
}
