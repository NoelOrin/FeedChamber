import { dehydrate } from '@tanstack/react-query';
import type { Metadata } from 'next';

import { LeaderboardBoard } from '~/components/leaderboard/leaderboard-board';
import { leaderboardOptions } from '~/components/leaderboard/queries';
import { loadPublicSettings } from '~/lib/queries/server';
import { getQueryClient } from '~/lib/query/client';
import { listPageMetadataFromSettings } from '~/lib/seo';

export async function generateMetadata(): Promise<Metadata> {
	const settings = await loadPublicSettings();
	return listPageMetadataFromSettings(settings, { title: '排行榜', path: '/leaderboard' });
}

/* 首屏只预取默认榜（积分），切签到/发帖榜由客户端 useQuery 现取。 */
export default async function LeaderboardPage() {
	const queryClient = getQueryClient();
	await queryClient.prefetchQuery(leaderboardOptions('points'));

	return (
		<HydrationBoundary state={dehydrate(queryClient)}>
			<LeaderboardBoard />
		</HydrationBoundary>
	);
}

import { HydrationBoundary } from '~/components/query-hydration';
