'use client';

import { parseAsString, useQueryState } from 'nuqs';
import { useCallback, useMemo } from 'react';
import { HomeModules } from '~/components/home/home-modules';
import { PostListPage } from '~/components/post/post-list-page';
import {
	feedParamsOf,
	type ListView,
	normalizeListView,
	normalizeSortBy,
	normalizeTag,
	type PostFeedPage,
	type SortBy,
	TIMELINE_QUERY_KEYS,
} from '~/components/post/queries';

/*
 * /timeline 的客户端列表：URL 是唯一状态源（迁移计划 §3.4 选定的 nuqs 方案）。
 *
 * 三个参数的最终形态
 *   tag        标签名，空串与缺席都算「不筛选」（旧 validateSearch 的 `|| undefined`）
 *   sort_by    白名单 hot | most_commented | active，其它值（含缺席）= 最新
 *   list_view  白名单 new | unread | read | posted，其它值（含缺席）= 最新
 * 默认值一律不写进 URL；写入用 history.replace（旧版 navigate({ replace: true }) 的同义），
 * 并关掉 scroll —— 列表本身是滚动加载，点胶囊不该跳回顶部。
 * 两端（RSC 预取 / 这里）都过 normalize* 再拼 key，所以切换胶囊不会二次请求已取过的组合。
 */

const feedParam = parseAsString.withDefault('').withOptions({ history: 'replace', scroll: false });

export interface TimelineListProps {
	/** RSC 预取的第一页，仅在 initialPageKey 与当前参数指纹一致时作为 initialData 使用 */
	initialPage?: PostFeedPage | null;
	initialPageKey?: string | null;
}

export function TimelineList({ initialPage, initialPageKey }: TimelineListProps) {
	/* tag 只读：它的唯一写入方是标签页 / 侧栏的链接（href 直达），列表本身不提供改标签的控件。 */
	const [rawTag] = useQueryState(TIMELINE_QUERY_KEYS.tag, feedParam);
	const [rawSortBy, setSortBy] = useQueryState(TIMELINE_QUERY_KEYS.sortBy, feedParam);
	const [rawListView, setListView] = useQueryState(TIMELINE_QUERY_KEYS.listView, feedParam);

	const tag = normalizeTag(rawTag);
	const sortBy = normalizeSortBy(rawSortBy);
	const listView = normalizeListView(rawListView);

	const params = useMemo(() => feedParamsOf({ tag, sortBy, listView }), [tag, sortBy, listView]);

	const changeSortBy = useCallback((next: SortBy | null) => void setSortBy(next ?? ''), [setSortBy]);
	const changeListView = useCallback((next: ListView | null) => void setListView(next ?? ''), [setListView]);

	return (
		<>
			{/* 首页自定义模块（P1）：无配置时组件返回 null，首页保持既有布局。 */}
			<HomeModules />
			<PostListPage
				params={params}
				sortBy={sortBy}
				onSortByChange={changeSortBy}
				listView={listView}
				/* 视图胶囊：旧版按 userStore.isLoggedIn 决定显隐，(app) 组已在服务端 requireAuth，这里直接给出 */
				onListViewChange={changeListView}
				createLink="/timeline/create"
				initialPage={initialPage}
				initialPageKey={initialPageKey}
			/>
		</>
	);
}
