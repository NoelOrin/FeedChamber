import { post } from '../client';
import type { ListParams, User } from '../types';

export const followService = {
	follow(userId: string) {
		return post<null>(`/users/${userId}/follow`);
	},

	unfollow(userId: string) {
		return post<null>(`/users/${userId}/unfollow`);
	},

	status(userId: string) {
		return post<{ following: boolean }>(`/users/${userId}/follow/status`);
	},

	followers(userId: string, params: ListParams) {
		return post<{ items: User[]; total: number }>(`/users/${userId}/followers`, params);
	},

	following(userId: string, params: ListParams) {
		return post<{ items: User[]; total: number }>(`/users/${userId}/following`, params);
	},

	counts(userId: string) {
		return post<{ followers: number; following: number }>(`/users/${userId}/counts`);
	},
};
