import { post, postPaginated } from '../client';
import type { ListParams } from '../types';

export interface ModerationItem {
	id: string;
	target_type: string;
	target_id: string;
	scene?: string;
	status: string;
	action?: string;
	strategy?: string;
	original_body?: string;
	filtered_body?: string;
	matched_rules?: string;
	reviewer_id?: string;
	reason?: string;
	created_at: string;
	updated_at: string;
}

export interface ModerationConfig {
	[key: string]: string;
}

export interface ModerationRule {
	id: string;
	name: string;
	scene?: string;
	type: string;
	pattern: string;
	action: string;
	priority?: number;
	enabled: boolean;
	description?: string;
	created_at?: string;
	updated_at?: string;
}

export const moderationService = {
	listPending(params?: ListParams) {
		return postPaginated<ModerationItem[]>('/admin/moderation/pending', params ?? {});
	},

	approve(id: string) {
		return post<null>(`/admin/moderation/${id}/approve`, {});
	},

	reject(id: string, reason = '') {
		return post<null>(`/admin/moderation/${id}/reject`, { reason });
	},

	listRules(params?: ListParams) {
		return postPaginated<ModerationRule[]>('/admin/moderation/rules/list', params ?? {});
	},

	createRule(body: {
		name: string;
		type: string;
		pattern: string;
		action: string;
		scene?: string;
		enabled?: boolean;
		description?: string;
	}) {
		return post<ModerationRule>('/admin/moderation/rules/create', body);
	},

	updateRule(id: string, body: Partial<ModerationRule>) {
		return post<null>('/admin/moderation/rules/update', { id, ...body });
	},

	deleteRule(id: string) {
		return post<null>('/admin/moderation/rules/delete', { id });
	},

	getConfigs() {
		return post<Record<string, string>>('/admin/moderation/configs', {});
	},

	setConfig(key: string, value: string) {
		return post<null>('/admin/moderation/configs/set', { key, value });
	},
};
