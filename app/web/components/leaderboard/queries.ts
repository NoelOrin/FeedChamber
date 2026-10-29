import { leaderboardService } from '@orinforum/api-client';
import { queryOptions } from '@tanstack/react-query';

import { qk } from '~/lib/query/keys';

export type LeaderboardType = 'points' | 'checkin' | 'posts';

/* api-client 的 barrel 只导出了 leaderboardService，行类型从返回值上剥下来用 */
export type LeaderboardRow = Awaited<ReturnType<typeof leaderboardService.list>>['data'][number];

/* 旧 leaderboard loader 的取数尺寸：一次 50 条，无翻页 */
const LEADERBOARD_PAGE_SIZE = 50;

function leaderboardParams(type: LeaderboardType) {
	return { type, page: 1, pageSize: LEADERBOARD_PAGE_SIZE };
}

export function leaderboardOptions(type: LeaderboardType) {
	const params = leaderboardParams(type);
	return queryOptions({
		queryKey: qk.leaderboard.list(params),
		queryFn: () => leaderboardService.list(params),
	});
}
