import { post, postBlob, postPaginated } from '../client';
import type { ListParams } from '../types';

export type Backup = {
	id: string;
	name: string;
	file_key: string;
	size: number;
	checksum: string;
	uploads_key?: string;
	uploads_size?: number;
	uploads_checksum?: string;
	status: string;
	created_by: string;
	created_at: string;
};

export const backupService = {
	create(body: { name?: string } = {}) {
		return post<Backup>('/admin/backups/create', body);
	},
	list(params: ListParams = {}) {
		return postPaginated<Backup[]>('/admin/backups/list', params);
	},
	cleanup() {
		return post<{ removed: number }>('/admin/backups/cleanup', {});
	},
	download(id: string) {
		return postBlob(`/admin/backups/${id}/download`, {});
	},
	restore(id: string) {
		return post<null>(`/admin/backups/${id}/restore`, {});
	},
	delete(id: string) {
		return post<null>(`/admin/backups/${id}/delete`, {});
	},
};
