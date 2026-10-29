import { post } from '../client';

export type Badge = {
	id: string;
	code: string;
	name: string;
	description?: string;
	icon?: string;
};

export const badgeService = {
	list() {
		return post<Badge[]>('/badges/list');
	},
	listByUser(userId: string) {
		return post<Badge[]>(`/user/${userId}/badges/list`);
	},
};
