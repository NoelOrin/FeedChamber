import { get, post } from '../client';

export interface CheckInResponse {
	points_earned: number;
	streak: number;
}

export interface CheckInStatus {
	checked_in_today: boolean;
	current_streak: number;
	max_streak: number;
	total_checkins: number;
	/** XP（签到不再单独记分；保留字段名 total_xp 与 user 一致）。 */
	total_xp: number;
}

export interface CheckInRecord {
	id: string;
	user_id: string;
	points_earned: number;
	streak: number;
	check_in_date: string;
}

export interface CheckInHistory {
	items: CheckInRecord[];
}

export const checkinService = {
	checkIn: () => post<CheckInResponse>('/checkin'),

	status: () => get<CheckInStatus>('/checkin/status'),

	history: (params?: { year?: number; month?: number }) => {
		const query = new URLSearchParams();
		if (params?.year !== undefined) query.set('year', String(params.year));
		if (params?.month !== undefined) query.set('month', String(params.month));
		const qs = query.toString();
		return get<CheckInHistory>(`/checkin/history${qs ? `?${qs}` : ''}`);
	},
};
