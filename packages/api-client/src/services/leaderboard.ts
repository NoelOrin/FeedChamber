import { postPaginated } from '../client';

export type LeaderboardItem = {
	id: string;
	username: string;
	display_name: string;
	avatar?: string;
	/** XP（旧的 total_points 别名已在 P4 下线） */
	total_xp: number;
	total_checkins: number;
	post_count?: number;
	trust_level?: number;
};

export const leaderboardService = {
	list(params: { type?: 'points' | 'checkin' | 'posts'; page?: number; pageSize?: number } = {}) {
		return postPaginated<LeaderboardItem[]>('/leaderboard/list', {
			type: params.type ?? 'points',
			page: params.page ?? 1,
			page_size: params.pageSize ?? 20,
		});
	},
};
