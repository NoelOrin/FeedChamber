import { get, post } from '../client';
import type { ListParams, Motion, MotionListItem } from '../types';

// 社区自治动议。与 reportService 同构，但完全独立的状态机与队列。
export const motionService = {
	create(body: { target_type: string; target_id: string; type: string; reason: string; source_motion_id?: string }) {
		return post<Motion>('/motions/create', body);
	},

	getById(id: string) {
		return get<Motion>(`/motions/${id}`);
	},

	list(params: ListParams & { status?: string; target_type?: string }) {
		return post<{ items: MotionListItem[]; total: number }>('/motions/list', params);
	},

	vote(id: string, side: 'approve' | 'reject') {
		return post<Motion>(`/motions/${id}/vote`, { side });
	},

	/** motion:manage —— 开放期满/达法定人数后结算 */
	resolve(id: string) {
		return post<Motion>(`/motions/${id}/resolve`);
	},

	/** motion:manage —— 翻案 / 标记滥诉 / 升级硬封禁 */
	updateStatus(id: string, body: { status?: string; abuse?: boolean; hard_ban?: boolean }) {
		return post<Motion>(`/motions/${id}/status`, body);
	},

	/** 发起者撤回（仅 pending） */
	remove(id: string) {
		return post<null>(`/motions/${id}/delete`);
	},
};
