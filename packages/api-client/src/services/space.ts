import { post } from '../client';
import type { Space, SpaceNode } from '../types';

export const spaceService = {
	/** POST /api/v1/spaces/tree — returns SpaceNode[] (NGA nested) */
	list() {
		return post<SpaceNode[]>('/spaces/tree');
	},

	tree() {
		return post<SpaceNode[]>('/spaces/tree');
	},

	flat() {
		return post<Space[]>('/spaces/flat');
	},

	roots() {
		return post<Space[]>('/spaces/roots');
	},

	get(id: string) {
		return post<Space>(`/spaces/${id}`);
	},

	create(body: {
		name: string;
		slug: string;
		description?: string;
		icon?: string;
		parent_id?: string;
		sort_order?: number;
	}) {
		return post<Space>('/spaces/create', body);
	},

	update(
		id: string,
		body: Partial<{
			name: string;
			description: string;
			icon: string;
			slug: string;
			parent_id: string;
			sort_order: number;
			is_active: boolean;
			visibility: 'public' | 'login' | 'level';
			min_level: number;
		}>,
	) {
		return post<Space>(`/spaces/${id}/update`, body);
	},

	toggleActive(id: string, is_active?: boolean) {
		return post<Space>(`/spaces/${id}/status`, { is_active });
	},

	delete(id: string) {
		return post<null>(`/spaces/${id}/delete`);
	},

	// ── NGA hierarchical ─────────────────────────────────────────
	children(id: string) {
		return post<Space[]>(`/spaces/${id}/children`);
	},

	ancestors(id: string) {
		return post<Space[]>(`/spaces/${id}/ancestors`);
	},

	breadcrumb(id: string) {
		return post<Space[]>(`/spaces/${id}/breadcrumb`);
	},

	descendants(id: string) {
		return post<Space[]>(`/spaces/${id}/descendants`);
	},

	subtree(id: string) {
		return post<SpaceNode>(`/spaces/${id}/subtree`);
	},

	move(id: string, body: { parent_id: string; sort_order?: number }) {
		return post<Space>(`/spaces/${id}/move`, body);
	},

	reorder(body: { parent_id: string; ordered_ids: string[] }) {
		return post<null>('/spaces/reorder', body);
	},
};
