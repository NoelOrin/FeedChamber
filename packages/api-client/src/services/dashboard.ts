import { post } from '../client';
import type { UserDashboardResponse } from '../types';

export const dashboardService = {
	home() {
		return post<UserDashboardResponse>('/dashboard');
	},
};
