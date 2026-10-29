import { post, postForm } from '../client';
import type { ListParams, Upload, UploadPresignData } from '../types';

export const uploadService = {
	upload(file: File) {
		const form = new FormData();
		form.append('file', file);
		return postForm<{ id: string; url: string; file_name: string; size: number }>('/upload', form);
	},

	presign(file_name: string, content_type: string) {
		return post<UploadPresignData>('/upload/presign', { file_name, content_type });
	},

	async directUpload(file: File): Promise<{ id: string; url: string; file_name: string; size: number }> {
		const presign = await this.presign(file.name, file.type);

		const form = new FormData();
		for (const [key, value] of Object.entries(presign.fields)) {
			form.append(key, value);
		}
		form.append('file', file);

		const resp = await fetch(presign.url, { method: 'POST', body: form });
		if (!resp.ok) {
			throw new Error(`direct upload failed: ${resp.status}`);
		}

		return post<{ id: string; url: string; file_name: string; size: number }>('/upload/confirm', {
			upload_id: presign.upload_id,
		});
	},

	getUrl(id: string): string {
		return `/api/v1/upload/${id}`;
	},

	list(params: ListParams) {
		return post<Upload[]>('/upload/list', params);
	},

	delete(id: string) {
		return post<null>(`/upload/${id}/delete`);
	},
};
